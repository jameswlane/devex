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
		user   string
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

Any base image must be Debian or Ubuntu based. The Dockerfile switches to
root for the apt layer and creates the --remote-user (default vscode) unless
the image already provides it, as the official Dev Container images do.

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
			result, err := devcontainer.Generate(selected, devcontainer.Options{Name: name, Image: image, RemoteUser: user})
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
			files := []devcontainerFile{
				{path: filepath.Join(target, "devcontainer.json"), data: spec},
				{path: filepath.Join(target, "Dockerfile"), data: []byte(result.Dockerfile)},
			}
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
	cmd.Flags().StringVar(&user, "remote-user", devcontainer.DefaultRemoteUser, "Non-root user editors connect as; created in the Dockerfile unless the base image provides it")
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

// devcontainerFile is one generated file and its contents.
type devcontainerFile struct {
	path string
	data []byte
}

// devcontainerRename installs staged files; tests replace it to inject failures.
var devcontainerRename = os.Rename

// writeDevcontainer installs the files as a set. Every destination is checked
// before anything is written, the contents are staged in exclusively created,
// randomly named temporary files, and if installing any file fails the files
// already installed are rolled back, so a failed run leaves an existing
// configuration as it was. Existing files are only replaced with force.
func writeDevcontainer(target string, files []devcontainerFile, force bool) (err error) {
	if info, statErr := os.Lstat(target); statErr == nil && !info.IsDir() {
		return fmt.Errorf("%s exists and is not a directory; move it aside or choose another --dir", target)
	}
	for _, file := range files {
		info, statErr := os.Lstat(file.path)
		switch {
		case errors.Is(statErr, os.ErrNotExist):
		case statErr != nil:
			return fmt.Errorf("checking %s: %w", file.path, statErr)
		case !info.Mode().IsRegular():
			return fmt.Errorf("%s is not a regular file; remove it and run the command again", file.path)
		case !force:
			return fmt.Errorf("%s already exists; use --force to replace it", file.path)
		}
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", target, err)
	}

	staged := make([]string, 0, len(files))
	defer func() {
		for _, path := range staged {
			_ = os.Remove(path) // already renamed into place on success
		}
	}()
	for _, file := range files {
		tmp, stageErr := stageDevcontainerFile(target, file)
		if stageErr != nil {
			return stageErr
		}
		staged = append(staged, tmp)
	}

	// Install each file, moving any original aside first so it can be restored.
	type installed struct{ path, backup string }
	var done []installed
	defer func() {
		for i := len(done) - 1; i >= 0; i-- {
			step := done[i]
			switch {
			case err == nil:
				if step.backup != "" {
					_ = os.Remove(step.backup)
				}
			case step.backup == "":
				_ = os.Remove(step.path)
			default:
				if restoreErr := devcontainerRename(step.backup, step.path); restoreErr != nil {
					err = fmt.Errorf("%w; restoring %s also failed, the original is at %s: %w", err, step.path, step.backup, restoreErr)
				}
			}
		}
	}()
	for i, file := range files {
		step := installed{path: file.path}
		if _, statErr := os.Lstat(file.path); statErr == nil {
			backup, reserveErr := reserveTemp(target, filepath.Base(file.path)+".devex-backup-*")
			if reserveErr != nil {
				return reserveErr
			}
			if renameErr := devcontainerRename(file.path, backup); renameErr != nil {
				_ = os.Remove(backup)
				return fmt.Errorf("backing up %s: %w", file.path, renameErr)
			}
			step.backup = backup
		}
		if renameErr := devcontainerRename(staged[i], file.path); renameErr != nil {
			// Roll this file back together with the ones already installed.
			if step.backup != "" {
				done = append(done, step)
			}
			return fmt.Errorf("writing %s: %w; no files were changed", file.path, renameErr)
		}
		done = append(done, step)
	}
	return nil
}

// stageDevcontainerFile writes data to a new, exclusively created temporary
// file next to the destination and returns its path.
func stageDevcontainerFile(dir string, file devcontainerFile) (path string, err error) {
	tmp, err := os.CreateTemp(dir, filepath.Base(file.path)+".devex-*.tmp")
	if err != nil {
		return "", fmt.Errorf("staging %s: %w", file.path, err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(file.data); err != nil {
		return "", fmt.Errorf("staging %s: %w", file.path, err)
	}
	if err = tmp.Chmod(0o644); err != nil {
		return "", fmt.Errorf("staging %s: %w", file.path, err)
	}
	if err = tmp.Close(); err != nil {
		return "", fmt.Errorf("staging %s: %w", file.path, err)
	}
	return tmp.Name(), nil
}

// reserveTemp creates an empty, exclusively owned temporary file name in dir.
func reserveTemp(dir, pattern string) (string, error) {
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("reserving a backup name in %s: %w", dir, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", fmt.Errorf("reserving a backup name in %s: %w", dir, err)
	}
	return tmp.Name(), nil
}
