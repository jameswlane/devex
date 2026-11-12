package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/dotfiles"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// NewDotfilesCmd creates the dotfiles command group.
func NewDotfilesCmd(_ types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dotfiles",
		Short: "Deploy files from your dotfiles Git repository",
		Long: `Deploy files from your own dotfiles Git repository.

The repository lists what to deploy in a devex-dotfiles.yaml file at its root:

  files:
    - source: zsh/.zshrc
      destination: ~/.zshrc
    - source: git/config
      destination: ~/.config/git/config

Destinations must be inside your home directory. A destination that differs
from the repository's file is backed up first, and every deployed file is
recorded so 'devex dotfiles rollback' can restore what was there before.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newDotfilesSyncCmd(settings), newDotfilesStatusCmd(settings), newDotfilesRollbackCmd(settings))
	return cmd
}

// confirm asks before a change unless yes is set; without a terminal it requires yes.
func confirm(cmd *cobra.Command, yes bool, question, hint string) error {
	if yes {
		return nil
	}
	if !interaction.IsInteractiveMode() {
		return fmt.Errorf("confirmation required; review with --dry-run, then use --yes to %s", hint)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N]: ", question)
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil || !strings.EqualFold(strings.TrimSpace(answer), "y") {
		return fmt.Errorf("cancelled; nothing changed")
	}
	return nil
}

func newDotfilesSyncCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		dryRun bool
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch your dotfiles repository and deploy its files",
		Long: `Fetch your dotfiles repository at a ref and deploy the files its
devex-dotfiles.yaml lists.

The repository is fetched into a temporary directory (depth 1, no submodules)
and removed afterwards. Private repositories use your SSH keys or git
credential helper; DevEx never prompts for or stores passwords, and refuses
URLs that contain credentials.

The plan is shown first. Unchanged files are left alone, files DevEx deployed
earlier are updated in place, and any other differing file is backed up
before it is replaced. Running sync again with no upstream changes changes
nothing.

The repository and ref come from --repo and --ref, DEVEX_DOTFILES_REPOSITORY
and DEVEX_DOTFILES_REF, or dotfiles.repository and dotfiles.ref in your
configuration.`,
		Example: `  # Preview
  devex dotfiles sync --repo https://github.com/me/dotfiles --dry-run

  # Deploy a tag without prompting
  devex dotfiles sync --repo git@github.com:me/dotfiles.git --ref v2 --yes`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			repository := strings.TrimSpace(viper.GetString("dotfiles.repository"))
			ref := strings.TrimSpace(viper.GetString("dotfiles.ref"))
			if err := dotfiles.ValidateRepository(repository); err != nil {
				return err
			}
			if err := dotfiles.ValidateRef(ref); err != nil {
				return err
			}
			stderr := cmd.ErrOrStderr()
			fmt.Fprintf(stderr, "Fetching %s", dotfiles.Redact(repository))
			if ref != "" {
				fmt.Fprintf(stderr, " at %s", ref)
			}
			fmt.Fprintln(stderr)
			checkout, err := dotfiles.Fetch(ctx, repository, ref)
			if err != nil {
				return err
			}
			defer func() { _ = checkout.Remove() }()
			mappings, err := dotfiles.LoadManifest(checkout, settings.HomeDir)
			if err != nil {
				return err
			}
			short := checkout.Commit
			if len(short) > 12 {
				short = short[:12]
			}
			fmt.Fprintf(stderr, "Plan for commit %s:\n", short)
			changes := 0
			options := configfiles.Options{Protected: dotfiles.ProtectedDirs(settings.HomeDir)}
			preview := configfiles.ApplyWithOptions(ctx, dotfiles.RecordKey, mappings, true, func(line string) {
				if strings.HasPrefix(line, "Copy ") || strings.HasPrefix(line, "Update ") {
					changes++
				}
				fmt.Fprintf(stderr, "  %s\n", line)
			}, options)
			if len(preview.Warnings) > 0 {
				return fmt.Errorf("the plan has %d problem(s); nothing changed", len(preview.Warnings))
			}
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run: nothing changed.")
				return nil
			}
			if changes > 0 {
				if err := confirm(cmd, yes, "Deploy these files?", "deploy the files"); err != nil {
					return err
				}
				result := configfiles.ApplyWithOptions(ctx, dotfiles.RecordKey, mappings, false, func(line string) {
					if strings.HasPrefix(line, "Warning: ") {
						fmt.Fprintln(stderr, line)
					}
				}, options)
				if len(result.Warnings) > 0 {
					return fmt.Errorf("%d file(s) could not be deployed; the rest were deployed and recorded; see 'devex dotfiles status'", len(result.Warnings))
				}
			}
			if err := dotfiles.WriteState(settings.HomeDir, dotfiles.State{Repository: dotfiles.Redact(repository), Ref: ref, Commit: checkout.Commit, SyncedAt: time.Now().UTC()}); err != nil {
				return fmt.Errorf("record the sync: %w", err)
			}
			if changes == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Dotfiles are up to date.")
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Deployed %d file(s) from commit %s.\n", changes, short)
			}
			return nil
		},
	}
	cmd.Flags().String("repo", "", "Dotfiles repository: https://, ssh://, git@host:path or an absolute path")
	cmd.Flags().String("ref", "", "Branch, tag or commit to deploy (default: the repository's default branch)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the plan without changing anything")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Deploy without prompting")
	_ = viper.BindPFlag("dotfiles.repository", cmd.Flags().Lookup("repo"))
	_ = viper.BindPFlag("dotfiles.ref", cmd.Flags().Lookup("ref"))
	_ = viper.BindEnv("dotfiles.repository", "DEVEX_DOTFILES_REPOSITORY")
	_ = viper.BindEnv("dotfiles.ref", "DEVEX_DOTFILES_REF")
	return cmd
}

// DotfileStatus is one deployed file.
type DotfileStatus struct {
	Destination string `json:"destination"`
	State       string `json:"state"`
	Backup      string `json:"backup,omitempty"`
}

func newDotfilesStatusCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the last sync and the state of each deployed file",
		Long: `Show the repository, ref and commit of the last sync, and whether each
deployed file is unchanged, modified since, or missing.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q; use table or json", format)
			}
			state, synced, err := dotfiles.ReadState(settings.HomeDir)
			if err != nil {
				return err
			}
			record, err := configfiles.ReadResult(dotfiles.RecordKey)
			if err != nil {
				return err
			}
			files := make([]DotfileStatus, 0, len(record.Files))
			for _, file := range record.Files {
				fileState, err := configfiles.CopyState(file)
				if err != nil {
					return err
				}
				files = append(files, DotfileStatus{Destination: file.Destination, State: fileState, Backup: file.Backup})
			}
			if format == "json" {
				var last *dotfiles.State
				if synced {
					last = &state
				}
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(struct {
					Sync  *dotfiles.State `json:"sync"`
					Files []DotfileStatus `json:"files"`
				}{last, files})
			}
			out := cmd.OutOrStdout()
			if !synced && len(files) == 0 {
				fmt.Fprintln(out, "No dotfiles have been synced. Run 'devex dotfiles sync --repo <url>'.")
				return nil
			}
			if synced {
				fmt.Fprintf(out, "Repository: %s\n", state.Repository)
				if state.Ref != "" {
					fmt.Fprintf(out, "Ref: %s\n", state.Ref)
				}
				fmt.Fprintf(out, "Commit: %s\nSynced: %s\n\n", state.Commit, state.SyncedAt.Format(time.RFC3339))
			}
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "FILE\tSTATE\tBACKUP")
			for _, file := range files {
				fmt.Fprintf(w, "%s\t%s\t%s\n", file.Destination, file.State, file.Backup)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}

func newDotfilesRollbackCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		dryRun bool
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Restore the files that were there before your dotfiles were deployed",
		Long: `Undo deployed dotfiles: each file that is still exactly as DevEx deployed
it is removed, and the file it replaced, if any, is restored from its backup.
Files you have changed since are kept, and their backups are reported.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stderr := cmd.ErrOrStderr()
			record, err := configfiles.ReadResult(dotfiles.RecordKey)
			if err != nil {
				return err
			}
			if len(record.Files) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No deployed dotfiles are recorded.")
				return nil
			}
			fmt.Fprintln(stderr, "Rollback plan:")
			if err := configfiles.Revert(dotfiles.RecordKey, true, func(line string) { fmt.Fprintf(stderr, "  %s\n", line) }); err != nil {
				return err
			}
			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run: nothing changed.")
				return nil
			}
			if err := confirm(cmd, yes, "Roll back these files?", "roll back"); err != nil {
				return err
			}
			if err := configfiles.Revert(dotfiles.RecordKey, false, nil); err != nil {
				return err
			}
			remaining, err := configfiles.ReadResult(dotfiles.RecordKey)
			if err != nil {
				return err
			}
			if len(remaining.Files) == 0 {
				if err := dotfiles.ClearState(settings.HomeDir); err != nil {
					return err
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Rolled back deployed dotfiles.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be restored without changing anything")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Roll back without prompting")
	return cmd
}
