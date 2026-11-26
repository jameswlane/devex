package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jameswlane/devex/apps/cli/internal/skillpacks"
	"github.com/spf13/cobra"
)

func newAgentSkillsCmd(manager *skillpacks.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install, list, update, and remove third-party agent skill packs",
		Long: `Manage curated and pinned third-party agent skill packs for Claude Code,
Codex, and Kimi Code.

Skills are installed to each agent's documented skills directory, pinned to a
resolved Git commit SHA in a lock file, and protected against overwriting local
edits or unmanaged copies.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newAgentSkillsAddCmd(manager))
	cmd.AddCommand(newAgentSkillsListCmd(manager))
	cmd.AddCommand(newAgentSkillsUpdateCmd(manager))
	cmd.AddCommand(newAgentSkillsRemoveCmd(manager))
	return cmd
}

func newAgentSkillsAddCmd(manager *skillpacks.Manager) *cobra.Command {
	var opts skillpacks.AddOptions
	cmd := &cobra.Command{
		Use:          "add <pack>[@ref]",
		Short:        "Install a curated or pinned third-party skill pack",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Long: `Install skills from a curated catalog pack (such as tstack) or an explicit
raw git+https:// source (with --allow-unreviewed).

Curated packs pin a reviewed commit SHA by default and resolve dependencies
between skills automatically. Pass --skill a,b to install a subset and its
dependency closure. Pass --dry-run to preview the full plan without writing.`,
		Example: `  # Preview installing all tstack skills for Claude Code
  devex agent skills add tstack --agent claude --scope user --dry-run

  # Install full-send and its dependency closure for Codex
  devex agent skills add tstack --skill full-send --agent codex --scope user

  # Adopt matching manual copies into the lock file
  devex agent skills add tstack --agent claude --scope user --adopt`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runOpts := opts
			runOpts.PackSpec = args[0]
			res, err := manager.Add(cmd.Context(), runOpts)
			if err != nil {
				return err
			}
			renderSkillPackAddResult(cmd, res)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&opts.Skills, "skill", nil, "Specific skill(s) to install along with their dependency closure")
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "Agent to install for: claude, codex or kimi (required)")
	cmd.Flags().StringVar(&opts.Scope, "scope", "", "Installation scope: user or project (required)")
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", "", "Project root (default: current working directory)")
	cmd.Flags().StringVar(&opts.SkillsPath, "skills-path", "skills", "Subdirectory containing skills inside a raw git+https:// repository")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview without writing files or updating the lock file")
	cmd.Flags().BoolVar(&opts.Overwrite, "overwrite", false, "Replace locally edited DevEx-managed skills and retain a backup")
	cmd.Flags().BoolVar(&opts.Adopt, "adopt", false, "Adopt unmanaged existing copies that hash-match the pinned snapshot")
	cmd.Flags().BoolVar(&opts.AllowUnreviewed, "allow-unreviewed", false, "Allow installing from an unreviewed raw git+https:// source")
	return cmd
}

func newAgentSkillsListCmd(manager *skillpacks.Manager) *cobra.Command {
	var (
		opts   skillpacks.ListOptions
		format string
	)
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List installed agent skill packs from the lock file",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q: use table or json", format)
			}
			entries, err := manager.List(opts)
			if err != nil {
				return err
			}
			if format == "json" {
				if entries == nil {
					entries = []skillpacks.ListEntry{}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}
			if len(entries) == 0 {
				cmd.Println("No agent skill packs installed.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "PACK\tSKILL\tAGENT\tSCOPE\tSHA\tEDITED")
			for _, e := range entries {
				edited := "no"
				if e.Missing {
					edited = "missing"
				} else if e.Edited {
					edited = "yes"
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Pack, e.Skill, e.Agent, e.Scope, e.SHA7, edited)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "Filter by agent: claude, codex or kimi")
	cmd.Flags().StringVar(&opts.Scope, "scope", "", "Filter by scope: user or project")
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", "", "Project root for project scope (default: current working directory)")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}

func newAgentSkillsUpdateCmd(manager *skillpacks.Manager) *cobra.Command {
	var opts skillpacks.UpdateOptions
	cmd := &cobra.Command{
		Use:          "update [<pack>[@ref]]",
		Short:        "Update installed agent skill packs and show file and script diffs",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `Update installed skill packs to the current embedded catalog SHA or an
explicit <pack>@<ref>. Shows a file-level diff for each changed skill and a
full unified line diff for any changed files under scripts/ before applying.

Locally edited skills are skipped and listed unless --overwrite is passed,
which retains a .devex-backup-* backup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runOpts := opts
			if len(args) == 1 {
				runOpts.PackSpec = args[0]
			}
			res, err := manager.Update(cmd.Context(), runOpts)
			if err != nil {
				return err
			}
			renderSkillPackUpdateResult(cmd, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "Filter by agent: claude, codex or kimi")
	cmd.Flags().StringVar(&opts.Scope, "scope", "", "Filter by scope: user or project")
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", "", "Project root for project scope (default: current working directory)")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview updates and diffs without writing")
	cmd.Flags().BoolVar(&opts.Overwrite, "overwrite", false, "Replace locally edited skills and retain a backup")
	return cmd
}

func newAgentSkillsRemoveCmd(manager *skillpacks.Manager) *cobra.Command {
	var opts skillpacks.RemoveOptions
	cmd := &cobra.Command{
		Use:          "remove <pack>",
		Short:        "Remove DevEx-managed skills belonging to a skill pack",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Long: `Remove lock-managed skills from a pack for a specific agent and scope.

Refuses to remove a skill that another installed skill depends on unless
--force is passed. Refuses to delete locally edited skills without --force;
with --force, locally edited skills are moved to a .devex-backup-* sibling
instead of being deleted. Unmanaged directories are never touched.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			runOpts := opts
			runOpts.Pack = args[0]
			res, err := manager.Remove(runOpts)
			if err != nil {
				return err
			}
			renderSkillPackRemoveResult(cmd, res)
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&opts.Skills, "skill", nil, "Specific skill(s) to remove (default: all skills in the pack)")
	cmd.Flags().StringVar(&opts.Agent, "agent", "", "Agent to remove from: claude, codex or kimi (required)")
	cmd.Flags().StringVar(&opts.Scope, "scope", "", "Installation scope: user or project (required)")
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", "", "Project root (default: current working directory)")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Override dependency checks and move locally edited skills to a backup")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "Preview removals without modifying files or the lock file")
	return cmd
}

func renderSkillPackAddResult(cmd *cobra.Command, res *skillpacks.AddResult) {
	for _, w := range res.Warnings {
		cmd.PrintErrln(w)
	}
	suffix := ""
	if res.Unreviewed {
		suffix = " [unreviewed]"
	}
	cmd.Printf("Pack: %s@%s (%s)%s\n", res.Pack, skillpacks.ShortSHA(res.SHA), res.Source, suffix)
	cmd.Printf("Agent: %s (%s scope)\n", res.Agent, res.Scope)
	if res.DryRun {
		cmd.Println("Dry run (no files or lock entries written):")
	}
	for _, item := range res.Items {
		cmd.Printf("  %-22s %-22s %s", formatActionLabel(item.Action, res.DryRun), item.Skill, item.Destination)
		if item.Reason != "" {
			cmd.Printf(" (%s)", item.Reason)
		}
		cmd.Println()
		if item.BackupPath != "" {
			cmd.Printf("    Backup: %s\n", item.BackupPath)
		}
	}
	if len(res.LegacyCodexCopies) > 0 {
		cmd.Println("\nLegacy Codex copies found (Codex reads ~/.agents/skills/, not ~/.codex/skills/; left untouched):")
		for _, legacy := range res.LegacyCodexCopies {
			status := fmt.Sprintf("differs from %s@%s", res.Pack, skillpacks.ShortSHA(res.SHA))
			if legacy.MatchesPin {
				status = fmt.Sprintf("matches %s@%s", res.Pack, skillpacks.ShortSHA(res.SHA))
			}
			cmd.Printf("  - %s: %s (%s)\n", legacy.Skill, legacy.Path, status)
		}
	}
	if len(res.Notes) > 0 {
		cmd.Println("\nNotes:")
		skills := make([]string, 0, len(res.Notes))
		for s := range res.Notes {
			skills = append(skills, s)
		}
		sort.Strings(skills)
		for _, s := range skills {
			for _, note := range res.Notes[s] {
				cmd.Printf("  - %s: %s\n", s, note)
			}
		}
	}
	if len(res.MissingRequirements) > 0 {
		cmd.Println("\nMissing requirements (not installed automatically):")
		for _, req := range res.MissingRequirements {
			cmd.Printf("  - %s: %s\n", req.Command, req.Reason)
			if req.Hint != "" {
				cmd.Printf("    Hint: %s\n", req.Hint)
			}
		}
	}
}

func renderSkillPackUpdateResult(cmd *cobra.Command, res *skillpacks.UpdateResult) {
	if len(res.Packs) == 0 {
		cmd.Println("No installed skill packs to update.")
		return
	}
	if res.DryRun {
		cmd.Println("Dry run (no files or lock entries written):")
	}
	for _, pack := range res.Packs {
		cmd.Printf("Pack: %s -> %s (%s scope)\n", pack.Pack, skillpacks.ShortSHA(pack.Target), pack.Scope)
		for _, item := range pack.Items {
			cmd.Printf("  %-22s %-22s %s", formatActionLabel(item.Action, res.DryRun), item.Skill, item.Destination)
			if item.Reason != "" {
				cmd.Printf(" (%s)", item.Reason)
			}
			cmd.Println()
			if item.Diff != nil && item.Diff.HasChanges() {
				for _, ch := range item.Diff.Changes {
					prefix := "M"
					switch ch.Kind {
					case skillpacks.FileAdded:
						prefix = "A"
					case skillpacks.FileRemoved:
						prefix = "D"
					}
					modeNote := ""
					if ch.OldMode != "" && ch.NewMode != "" && ch.OldMode != ch.NewMode {
						modeNote = fmt.Sprintf(" (mode %s -> %s)", ch.OldMode, ch.NewMode)
					}
					cmd.Printf("    %s %s%s\n", prefix, ch.Path, modeNote)
					if ch.UnifiedDiff != "" {
						for _, line := range strings.Split(strings.TrimRight(ch.UnifiedDiff, "\n"), "\n") {
							cmd.Printf("      %s\n", line)
						}
					}
				}
			}
			if item.BackupPath != "" {
				cmd.Printf("    Backup: %s\n", item.BackupPath)
			}
		}
	}
}

func renderSkillPackRemoveResult(cmd *cobra.Command, res *skillpacks.RemoveResult) {
	cmd.Printf("Pack: %s (%s, %s scope)\n", res.Pack, res.Agent, res.Scope)
	if res.DryRun {
		cmd.Println("Dry run (no files or lock entries modified):")
	}
	for _, item := range res.Items {
		cmd.Printf("  %-22s %-22s %s", formatActionLabel(item.Action, res.DryRun), item.Skill, item.Destination)
		if item.Reason != "" {
			cmd.Printf(" (%s)", item.Reason)
		}
		cmd.Println()
		if item.BackupPath != "" {
			cmd.Printf("    Backup: %s\n", item.BackupPath)
		}
	}
}

func formatActionLabel(action skillpacks.SkillAction, dryRun bool) string {
	switch action {
	case skillpacks.ActionInstall:
		if dryRun {
			return "Would install"
		}
		return "Installed"
	case skillpacks.ActionReplace:
		if dryRun {
			return "Would replace"
		}
		return "Replaced"
	case skillpacks.ActionAdopt:
		if dryRun {
			return "Would adopt"
		}
		return "Adopted"
	case skillpacks.ActionAlreadyInstalled:
		return "Already installed"
	case skillpacks.ActionSkipEdited:
		return "Skipped (edited)"
	case skillpacks.ActionSkipAdoptable:
		return "Skipped (adoptable)"
	case skillpacks.ActionSkipLocalCopy:
		return "Skipped (local copy)"
	case skillpacks.ActionSkipSharedCodex:
		return "Skipped (shared)"
	case skillpacks.ActionRemove:
		if dryRun {
			return "Would remove"
		}
		return "Removed"
	case skillpacks.ActionBackupAndRemove:
		if dryRun {
			return "Would backup+remove"
		}
		return "Backed up+removed"
	default:
		return string(action)
	}
}
