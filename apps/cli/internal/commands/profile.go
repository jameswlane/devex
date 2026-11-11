package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const defaultProfile = "dev"

// NewProfileCmd creates the profile command group.
func NewProfileCmd(_ types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Create and switch between named configuration profiles",
		Long: `Keep separate configurations, such as work and personal, as named profiles.

A profile is a directory of configuration overrides, environments/<name>/,
in your configuration directory (and optionally in the team or default
configuration). Its files are applied on top of the shared configuration.
The current profile is saved by 'devex profile switch'; DEVEX_ENV, if set,
overrides it for one command. Without a saved profile, "dev" is used.

Switching profiles changes configuration only. It never installs or removes
software; run 'devex install' afterwards to apply the new profile.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newProfileCreateCmd(settings), newProfileListCmd(settings), newProfileShowCmd(settings), newProfileSwitchCmd(settings))
	return cmd
}

func newProfileCreateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a profile",
		Long: `Create an empty profile directory in your configuration, or a copy of another
profile's user configuration with --from. Add YAML files to the directory to
override the shared configuration while the profile is active.`,
		Example: `  devex profile create work
  devex profile create personal --from work`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := config.ValidateProfileName(name); err != nil {
				return err
			}
			dir := settings.ProfileDirs(name)["user"]
			if _, err := os.Lstat(dir); err == nil {
				return fmt.Errorf("profile %s already exists at %s", name, dir)
			}
			if from != "" {
				if err := config.ValidateProfileName(from); err != nil {
					return err
				}
				source := settings.ProfileDirs(from)["user"]
				if info, err := os.Stat(source); err != nil || !info.IsDir() {
					return fmt.Errorf("profile %s has no user configuration to copy at %s", from, source)
				}
				if err := copyConfigurationTree(source, dir); err != nil {
					return fmt.Errorf("copy profile %s: %w", from, err)
				}
			} else if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created profile %s at %s\nSwitch to it with 'devex profile switch %s'.\n", name, dir, name)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Copy the user configuration of this profile")
	return cmd
}

// ProfileInfo describes one profile.
type ProfileInfo struct {
	Name    string   `json:"name"`
	Current bool     `json:"current"`
	Tiers   []string `json:"tiers"`
}

func newProfileListCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List profiles and show the current one",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q; use table or json", format)
			}
			found, err := settings.Profiles()
			if err != nil {
				return err
			}
			current := settings.GetEnvironment()
			if _, ok := found[defaultProfile]; !ok {
				found[defaultProfile] = nil
			}
			if _, ok := found[current]; !ok {
				found[current] = nil
			}
			profiles := make([]ProfileInfo, 0, len(found))
			for name, tiers := range found {
				if tiers == nil {
					tiers = []string{}
				}
				profiles = append(profiles, ProfileInfo{Name: name, Current: name == current, Tiers: tiers})
			}
			sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
			if format == "json" {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(profiles)
			}
			out := cmd.OutOrStdout()
			for _, profile := range profiles {
				marker := " "
				if profile.Current {
					marker = "*"
				}
				where := "no overrides"
				if len(profile.Tiers) > 0 {
					where = strings.Join(profile.Tiers, ", ")
				}
				fmt.Fprintf(out, "%s %s (%s)\n", marker, profile.Name, where)
			}
			if variable, value := config.ProfileOverride(); variable != "" {
				fmt.Fprintf(out, "\n%s=%s overrides the saved profile.\n", variable, value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}

func newProfileShowCmd(settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:          "show [name]",
		Short:        "Show a profile's directories and files",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := settings.GetEnvironment()
			if len(args) > 0 {
				name = args[0]
			}
			if err := config.ValidateProfileName(name); err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Profile: %s\n", name)
			dirs := settings.ProfileDirs(name)
			for _, tier := range []string{"default", "team", "user"} {
				files := configurationFiles(dirs[tier])
				state := fmt.Sprintf("%d file(s)", len(files))
				if files == nil {
					state = "not present"
				}
				fmt.Fprintf(out, "  %s: %s (%s)\n", tier, dirs[tier], state)
				for _, file := range files {
					fmt.Fprintf(out, "    %s\n", file)
				}
			}
			return nil
		},
	}
}

// configurationFiles lists the files under dir, or nil when dir is missing.
func configurationFiles(dir string) []string {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return nil
	}
	files := []string{}
	_ = walkRegularFiles(dir, func(rel string) { files = append(files, rel) })
	sort.Strings(files)
	return files
}

// ProfileSwitch reports a switch.
type ProfileSwitch struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Added   []string `json:"added_default_apps"`
	Removed []string `json:"removed_default_apps"`
	DryRun  bool     `json:"dry_run"`
}

func newProfileSwitchCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		dryRun bool
		format string
	)
	cmd := &cobra.Command{
		Use:   "switch <name>",
		Short: "Make a profile the current one",
		Long: `Make a profile the current one, after showing how the default applications
change. Only the saved profile changes: nothing is installed or removed. Run
'devex install' afterwards to install the new profile's applications.`,
		Example:      `  devex profile switch work --dry-run`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q; use table or json", format)
			}
			name := args[0]
			if err := config.ValidateProfileName(name); err != nil {
				return err
			}
			found, err := settings.Profiles()
			if err != nil {
				return err
			}
			if _, ok := found[name]; !ok && name != defaultProfile {
				return fmt.Errorf("profile %s does not exist; create it with 'devex profile create %s'", name, name)
			}
			current := settings.GetEnvironment()
			before, err := profileDefaultApps(cmd.Context(), settings, current)
			if err != nil {
				return fmt.Errorf("load profile %s: %w", current, err)
			}
			after, err := profileDefaultApps(cmd.Context(), settings, name)
			if err != nil {
				return fmt.Errorf("load profile %s: %w", name, err)
			}
			result := ProfileSwitch{From: current, To: name, Added: difference(after, before), Removed: difference(before, after), DryRun: dryRun}
			if !dryRun {
				if err := config.SaveCurrentProfile(settings.HomeDir, name); err != nil {
					return err
				}
			}
			if format == "json" {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}
			out := cmd.OutOrStdout()
			verb := "Switched"
			if dryRun {
				verb = "Would switch"
			}
			fmt.Fprintf(out, "%s from %s to %s.\n", verb, current, name)
			if len(result.Added) > 0 {
				fmt.Fprintf(out, "Default applications added: %s\n", strings.Join(result.Added, ", "))
			}
			if len(result.Removed) > 0 {
				fmt.Fprintf(out, "Default applications removed from the profile: %s (they stay installed)\n", strings.Join(result.Removed, ", "))
			}
			fmt.Fprintln(out, "Nothing was installed or removed; run 'devex install' to apply the profile.")
			if variable, value := config.ProfileOverride(); variable != "" {
				fmt.Fprintf(out, "Note: %s=%s is set and overrides the saved profile until it is unset.\n", variable, value)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the change without saving it")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}

func profileDefaultApps(ctx context.Context, settings config.CrossPlatformSettings, profile string) (map[string]bool, error) {
	loaded, err := config.LoadConfigDirs(ctx, settings.HomeDir, config.EffectiveConfigDirsFor(settings, profile), config.FileApplicationLoader{})
	if err != nil {
		return nil, err
	}
	loaded.Catalog = settings.Catalog
	names := map[string]bool{}
	for _, app := range loaded.GetDefaultApps() {
		names[app.Name] = true
	}
	return names, nil
}

func difference(a, b map[string]bool) []string {
	out := []string{}
	for name := range a {
		if !b[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func walkRegularFiles(dir string, visit func(rel string)) error {
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil {
				return relErr
			}
			visit(filepath.ToSlash(rel))
		}
		return nil
	})
}
