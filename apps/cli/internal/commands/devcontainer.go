package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/devcontainer"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// NewDevcontainerCmd creates the devcontainer command group.
func NewDevcontainerCmd(_ types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "devcontainer",
		Short:        "Generate a Development Container from your DevEx configuration",
		SilenceUsage: true,
	}
	cmd.AddCommand(newDevcontainerGenerateCmd(settings))
	return cmd
}

func newDevcontainerGenerateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		dir    string
		image  string
		name   string
		apps   []string
		dryRun bool
		force  bool
		format string
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Write .devcontainer/devcontainer.json and a Dockerfile",
		Long: `Generate a Development Container (https://containers.dev) for a project from
your DevEx applications: your configured defaults, or the applications named
with --apps.

Language runtimes and tools (Go, Node.js, Python, Rust, Ruby, Java, GitHub
CLI, git, Docker) become official Dev Container Features. Plain apt packages
become one apt-get layer in the Dockerfile. Applications that belong on the
host (desktop apps, flatpak and snap packages, apt packages from third-party
repositories, applications with setup hooks) are left out and listed with the
reason, so you can add them yourself.

Nothing is mounted from the host: no SSH keys, credentials or home
directory. Existing files are never overwritten without --force.`,
		Example: `  # Preview for the configured defaults
  devex devcontainer generate --dry-run

  # Generate for chosen applications on another base image
  devex devcontainer generate --apps Go,Node.js,curl --image mcr.microsoft.com/devcontainers/base:debian-12`,
		Annotations:  map[string]string{catalogAnnotation: catalogRequired},
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q; use table or json", format)
			}
			selected, err := devcontainerApps(cmd, settings, apps)
			if err != nil {
				return err
			}
			result, err := devcontainer.Generate(selected, devcontainer.Options{Name: name, Image: image})
			if err != nil {
				return err
			}
			spec, err := result.Marshal()
			if err != nil {
				return err
			}
			if err := devcontainer.Validate(spec); err != nil {
				return err
			}
			target := filepath.Join(dir, ".devcontainer")
			files := map[string][]byte{filepath.Join(target, "devcontainer.json"): spec, filepath.Join(target, "Dockerfile"): []byte(result.Dockerfile)}
			if !dryRun {
				if err := writeDevcontainer(target, files, force); err != nil {
					return err
				}
			}
			if format == "json" {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(struct {
					devcontainer.Result
					Directory string `json:"directory"`
					DryRun    bool   `json:"dry_run"`
				}{result, target, dryRun})
			}
			out := cmd.OutOrStdout()
			if dryRun {
				fmt.Fprintf(out, "Would write %s:\n\n%s\nand Dockerfile:\n\n%s\n", filepath.Join(target, "devcontainer.json"), spec, result.Dockerfile)
			} else {
				fmt.Fprintf(out, "Wrote %s and %s\n", filepath.Join(target, "devcontainer.json"), filepath.Join(target, "Dockerfile"))
			}
			fmt.Fprintf(out, "Included: %s\n", strings.Join(result.Included, ", "))
			for _, skipped := range result.Skipped {
				fmt.Fprintf(out, "Skipped %s: %s\n", skipped.App, skipped.Reason)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "Project directory to write .devcontainer into")
	cmd.Flags().StringVar(&image, "image", devcontainer.DefaultImage, "Base image")
	cmd.Flags().StringVar(&name, "name", "DevEx", "Container name shown by editors")
	cmd.Flags().StringSliceVar(&apps, "apps", nil, "Applications to include (default: your configured defaults)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the files without writing them")
	cmd.Flags().BoolVar(&force, "force", false, "Replace existing generated files")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}

func devcontainerApps(cmd *cobra.Command, settings config.CrossPlatformSettings, names []string) ([]types.CrossPlatformApp, error) {
	if len(names) > 0 {
		// Feature-backed runtimes need no catalog entry.
		var apps []types.CrossPlatformApp
		var lookup []string
		for _, name := range names {
			if devcontainer.HasFeature(name) {
				apps = append(apps, types.CrossPlatformApp{Name: strings.TrimSpace(name)})
				continue
			}
			lookup = append(lookup, name)
		}
		if len(lookup) > 0 {
			resolved, _, err := agentapi.SelectApplications(cmd.Context(), settings, agentapi.PlanRequest{Names: lookup})
			if err != nil {
				return nil, err
			}
			apps = append(apps, resolved...)
		}
		return apps, nil
	}
	_, merged, _, err := configuredApplications(cmd.Context(), settings)
	if err != nil {
		return nil, err
	}
	defaults := merged.GetDefaultApps()
	if len(defaults) == 0 {
		return nil, errors.New("no default applications are configured; name them with --apps")
	}
	return defaults, nil
}

// writeDevcontainer writes the files, refusing to replace any without force.
func writeDevcontainer(target string, files map[string][]byte, force bool) error {
	if info, err := os.Lstat(target); err == nil && !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory", target)
	}
	if !force {
		for path := range files {
			if _, err := os.Lstat(path); err == nil {
				return fmt.Errorf("%s already exists; use --force to replace it", path)
			}
		}
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	for path, data := range files {
		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		tmp := path + ".devex-tmp"
		if err := os.WriteFile(tmp, data, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}
