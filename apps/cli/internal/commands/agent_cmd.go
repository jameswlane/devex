package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentdoctor"
	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/agentskills"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/spf13/cobra"
)

// AgentCmdDeps bundles injectable dependencies for the `agent` command tree.
type AgentCmdDeps struct {
	Settings        config.CrossPlatformSettings
	PluginBootstrap pluginBootstrapProvider
	SandboxProbe    agentsandbox.Probe
	Executor        agentPluginExecutor
	DoctorProbe     agentdoctor.Probe
	LoginRunner     agentdoctor.LoginRunner
}

type agentSkillOptions struct {
	agent      string
	scope      string
	projectDir string
	overwrite  bool
	dryRun     bool
}

type agentSkillDestination struct {
	root     string
	relative string
}

func (d agentSkillDestination) path() string {
	return filepath.Join(d.root, filepath.FromSlash(d.relative))
}

func NewAgentCmd() *cobra.Command {
	return NewAgentCmdWithDeps(config.CrossPlatformSettings{}, nil, agentsandbox.DefaultProbe(), nil)
}

// NewAgentCmdWithSandboxProbe constructs the `agent` command with an injected
// sandbox probe so tests can run without inspecting the host kernel or binaries.
func NewAgentCmdWithSandboxProbe(sandboxProbe agentsandbox.Probe) *cobra.Command {
	return NewAgentCmdWithDeps(config.CrossPlatformSettings{}, nil, sandboxProbe, nil)
}

// NewAgentCmdWithDeps constructs the `agent` command with full dependency
// injection for configuration, plugin bootstrap, sandbox probe, and plugin execution.
func NewAgentCmdWithDeps(settings config.CrossPlatformSettings, getPluginBootstrap pluginBootstrapProvider, sandboxProbe agentsandbox.Probe, executor agentPluginExecutor) *cobra.Command {
	doctorProbe := agentdoctor.DefaultProbe()
	doctorProbe.SandboxProbe = sandboxProbe
	return NewAgentCmdFromDeps(AgentCmdDeps{
		Settings:        settings,
		PluginBootstrap: getPluginBootstrap,
		SandboxProbe:    sandboxProbe,
		Executor:        executor,
		DoctorProbe:     doctorProbe,
		LoginRunner:     agentdoctor.DefaultLoginRunner(),
	})
}

// NewAgentCmdFromDeps constructs the `agent` command from a full set of
// injected dependencies, including the doctor probe and login runner.
func NewAgentCmdFromDeps(deps AgentCmdDeps) *cobra.Command {
	executor := deps.Executor
	if executor == nil {
		executor = bootstrapAgentPluginExecutor{getBootstrap: deps.PluginBootstrap}
	}
	if deps.LoginRunner == nil {
		deps.LoginRunner = agentdoctor.DefaultLoginRunner()
	}
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage coding agent integrations",
		Args:  cobra.NoArgs,
	}
	var options agentSkillOptions
	install := &cobra.Command{
		Use:          "install-skill",
		Short:        "Install the DevEx skill bundled with this CLI",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Install the exact DevEx skill and references embedded in this CLI.

User destinations:
  Claude Code: ~/.claude/skills/devex/
  Codex:       ~/.agents/skills/devex/
  Kimi Code:   $KIMI_CODE_HOME/skills/devex/ (default ~/.kimi-code/skills/devex/)

Project destinations use PROJECT/.claude, PROJECT/.agents or PROJECT/.kimi-code.
PROJECT defaults to the current working directory; --project-dir selects it explicitly.
Codex's .agents/skills location is also read by Kimi Code, so one Codex
installation covers both agents and Kimi skips when that copy exists. Codex
still installs alongside a Kimi-specific copy, but warns with the Kimi path
to remove so Kimi does not load DevEx twice. Differing content requires --overwrite.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return installAgentSkill(cmd, options)
		},
	}
	install.Flags().StringVar(&options.agent, "agent", "", "Agent to install for: claude, codex or kimi (required)")
	install.Flags().StringVar(&options.scope, "scope", "", "Installation scope: user or project (required)")
	install.Flags().StringVar(&options.projectDir, "project-dir", "", "Project root (default: current working directory)")
	install.Flags().BoolVar(&options.overwrite, "overwrite", false, "Replace differing content and retain a backup")
	install.Flags().BoolVar(&options.dryRun, "dry-run", false, "Preview without writing files")
	cmd.AddCommand(install)
	cmd.AddCommand(newAgentInitCmd(deps.Settings))
	cmd.AddCommand(newAgentSandboxCmd(deps.SandboxProbe))
	cmd.AddCommand(newAgentApplyCmd(deps.Settings, executor))
	cmd.AddCommand(newAgentDiffCmd(deps.Settings, executor))
	cmd.AddCommand(newAgentDoctorCmd(deps.DoctorProbe))
	cmd.AddCommand(newAgentLoginCmd(deps.DoctorProbe, deps.LoginRunner))
	return cmd
}

func newAgentSandboxCmd(probe agentsandbox.Probe) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:          "sandbox",
		Short:        "Check Linux sandbox prerequisites for coding agents",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Verify that Linux sandbox prerequisites (bubblewrap, socat, user namespaces,
and AppArmor profiles) are installed and able to launch an unprivileged sandbox.

DevEx never modifies kernel sysctls or AppArmor profiles automatically; when a
restriction is detected, exact manual remediation commands are printed instead.
The JSON report is saved to the DevEx state directory for reuse by diagnostics.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format %q: use table or json", format)
			}
			report := agentsandbox.Check(cmd.Context(), probe)
			if _, err := agentsandbox.SaveReport(report); err != nil {
				cmd.PrintErrf("Warning: could not record sandbox report: %v\n", err)
			}
			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			cmd.Print(agentsandbox.FormatTable(report))
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	return cmd
}

func installAgentSkill(cmd *cobra.Command, options agentSkillOptions) error {
	if options.agent != "claude" && options.agent != "codex" && options.agent != "kimi" {
		return fmt.Errorf("unsupported agent %q: use claude, codex or kimi", options.agent)
	}
	if options.scope != "user" && options.scope != "project" {
		return fmt.Errorf("unsupported scope %q: use user or project", options.scope)
	}
	if options.scope == "user" && options.projectDir != "" {
		return fmt.Errorf("--project-dir requires --scope project")
	}
	destination, err := resolveAgentSkillDestination(options.agent, options.scope, options.projectDir)
	if err != nil {
		return err
	}
	cmd.Printf("Destination: %s\n", destination.path())
	if options.agent != "claude" {
		otherAgent := "codex"
		if options.agent == "codex" {
			otherAgent = "kimi"
		}
		other, err := resolveAgentSkillDestination(otherAgent, options.scope, options.projectDir)
		if err != nil {
			return err
		}
		_, err = os.Lstat(other.path())
		if err == nil {
			if options.agent == "kimi" {
				cmd.PrintErrf("Warning: DevEx is already installed for %s at %s; Kimi Code reads both locations, so no duplicate was created. One Codex installation in .agents/skills covers both Codex and Kimi.\n", otherAgent, other.path())
				cmd.Printf("Skipped: %s\n", destination.path())
				return nil
			}
			cmd.PrintErrf("Warning: DevEx is already installed for Kimi Code at %s; after installing for Codex, Kimi Code will load DevEx twice. Remove %s to avoid the duplicate; it was left unchanged.\n", other.path(), other.path())
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect existing %s skill %q: %w", otherAgent, other.path(), err)
		}
	}
	return writeAgentSkill(cmd, destination, options)
}

func resolveAgentSkillDestination(agent, scope, projectDir string) (agentSkillDestination, error) {
	var root, directory string
	if scope == "project" {
		var err error
		root, err = filepath.Abs(projectDir)
		if err != nil {
			return agentSkillDestination{}, fmt.Errorf("resolve project directory: %w", err)
		}
		if projectDir == "" {
			root, err = os.Getwd()
			if err != nil {
				return agentSkillDestination{}, fmt.Errorf("resolve current working directory: %w", err)
			}
		}
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return agentSkillDestination{}, fmt.Errorf("resolve home directory: %w", err)
		}
		root = home
	}
	switch agent {
	case "claude":
		directory = ".claude"
	case "codex":
		directory = ".agents"
	case "kimi":
		directory = ".kimi-code"
		if scope == "user" && os.Getenv("KIMI_CODE_HOME") != "" {
			var err error
			root, err = filepath.Abs(os.Getenv("KIMI_CODE_HOME"))
			if err != nil {
				return agentSkillDestination{}, fmt.Errorf("resolve KIMI_CODE_HOME: %w", err)
			}
			directory = ""
		}
	}
	return agentSkillDestination{root: filepath.Clean(root), relative: filepath.ToSlash(filepath.Join(directory, "skills", agentskills.SkillName))}, nil
}

type bundleEntry struct {
	directory bool
	data      []byte
}

func readBundle(bundle fs.FS, root string) (map[string]bundleEntry, error) {
	entries := map[string]bundleEntry{}
	err := fs.WalkDir(bundle, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(filepath.FromSlash(root), filepath.FromSlash(path))
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if entry.IsDir() {
			entries[name] = bundleEntry{directory: true}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("skill bundle entry %q is not a regular file", path)
		}
		data, err := fs.ReadFile(bundle, path)
		if err != nil {
			return err
		}
		entries[name] = bundleEntry{data: data}
		return nil
	})
	return entries, err
}

func validateAgentSkillPath(root *os.Root, destination string) error {
	current := ""
	for _, component := range strings.Split(destination, "/") {
		if current == "" {
			current = component
		} else {
			current += "/" + component
		}
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect skill path %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("skill path %q contains a symlink at %q", destination, current)
		}
		if current != destination && !info.IsDir() {
			return fmt.Errorf("skill path component %q is not a directory", current)
		}
	}
	return nil
}

func writeAgentSkill(cmd *cobra.Command, destination agentSkillDestination, options agentSkillOptions) error {
	bundle, err := agentskills.Bundle()
	if err != nil {
		return err
	}
	baseInfo, err := os.Stat(destination.root)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if options.dryRun {
			cmd.Printf("Would install: %s\n", destination.path())
			return nil
		}
		if options.scope == "project" {
			return fmt.Errorf("project directory %q does not exist", destination.root)
		}
		if err := os.MkdirAll(destination.root, 0o755); err != nil {
			return fmt.Errorf("create skill home %q: %w", destination.root, err)
		}
	case err != nil:
		return fmt.Errorf("inspect skill root %q: %w", destination.root, err)
	case !baseInfo.IsDir():
		return fmt.Errorf("skill root %q is not a directory", destination.root)
	}
	root, err := os.OpenRoot(destination.root)
	if err != nil {
		return fmt.Errorf("open skill root %q: %w", destination.root, err)
	}
	defer func() { _ = root.Close() }()
	if err := validateAgentSkillPath(root, destination.relative); err != nil {
		return err
	}

	existing := false
	identical := false
	info, err := root.Lstat(destination.relative)
	if err == nil {
		existing = true
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("skill destination %q is not a regular directory", destination.path())
		}
		expected, readErr := readBundle(bundle, ".")
		if readErr != nil {
			return fmt.Errorf("read embedded skill: %w", readErr)
		}
		actual, readErr := readBundle(root.FS(), destination.relative)
		if readErr != nil {
			return fmt.Errorf("read installed skill: %w", readErr)
		}
		identical = reflect.DeepEqual(expected, actual)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect skill destination %q: %w", destination.path(), err)
	}
	if identical {
		cmd.Printf("Already installed: %s\n", destination.path())
		return nil
	}
	if existing && !options.overwrite {
		return fmt.Errorf("skill destination %q has differing content; use --overwrite to replace it and retain a backup", destination.path())
	}
	if options.dryRun {
		if existing {
			cmd.Printf("Would replace: %s (existing content would be backed up)\n", destination.path())
		} else {
			cmd.Printf("Would install: %s\n", destination.path())
		}
		return nil
	}
	return stageAndPublishAgentSkill(cmd, root, destination, bundle, existing)
}

func stageAndPublishAgentSkill(cmd *cobra.Command, root *os.Root, destination agentSkillDestination, bundle fs.FS, existing bool) error {
	parent := filepath.ToSlash(filepath.Dir(destination.path()))
	parentRelative := filepath.ToSlash(filepath.Dir(filepath.FromSlash(destination.relative)))
	if err := root.MkdirAll(parentRelative, 0o755); err != nil {
		return fmt.Errorf("create skill parent %q: %w", filepath.Dir(destination.path()), err)
	}
	stage, err := os.MkdirTemp(parent, ".devex-install-*")
	if err != nil {
		return fmt.Errorf("create skill staging directory: %w", err)
	}
	stageRelative := filepath.ToSlash(filepath.Join(parentRelative, filepath.Base(stage)))
	defer func() { _ = root.RemoveAll(stageRelative) }()
	if err := stageAgentSkillBundle(cmd.Context(), root, stageRelative, bundle); err != nil {
		return err
	}
	if err := cmd.Context().Err(); err != nil {
		return err
	}
	backupRelative := ""
	if existing {
		backupRelative = filepath.ToSlash(filepath.Join(parentRelative, fmt.Sprintf(".devex-backup-%d", time.Now().UnixNano())))
		if err := root.Rename(destination.relative, backupRelative); err != nil {
			return fmt.Errorf("back up existing skill: %w", err)
		}
	}
	if err := root.Rename(stageRelative, destination.relative); err != nil {
		if existing {
			if restoreErr := root.Rename(backupRelative, destination.relative); restoreErr != nil {
				return fmt.Errorf("publish skill: %w; restore backup %q manually: %w", err, filepath.Join(destination.root, filepath.FromSlash(backupRelative)), restoreErr)
			}
		}
		return fmt.Errorf("publish skill: %w", err)
	}
	if existing {
		cmd.Printf("Replaced: %s\nBackup: %s\n", destination.path(), filepath.Join(destination.root, filepath.FromSlash(backupRelative)))
	} else {
		cmd.Printf("Installed: %s\n", destination.path())
	}
	return nil
}

func stageAgentSkillBundle(ctx context.Context, root *os.Root, destination string, bundle fs.FS) error {
	return fs.WalkDir(bundle, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		target := destination
		if path != "." {
			target = destination + "/" + path
		}
		if entry.IsDir() {
			if err := root.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create staged skill directory %q: %w", target, err)
			}
			return root.Chmod(target, 0o755)
		}
		data, err := fs.ReadFile(bundle, path)
		if err != nil {
			return fmt.Errorf("read embedded skill file %q: %w", path, err)
		}
		if err := root.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write staged skill file %q: %w", target, err)
		}
		if err := root.Chmod(target, 0o644); err != nil {
			return fmt.Errorf("set staged skill permissions %q: %w", target, err)
		}
		return nil
	})
}
