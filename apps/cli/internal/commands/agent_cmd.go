package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/jameswlane/devex/apps/cli/internal/agentdoctor"
	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/agentskills"
	skillinstall "github.com/jameswlane/devex/apps/cli/internal/agentskills/install"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/skillpacks"
	"github.com/spf13/cobra"
)

// AgentCmdDeps bundles injectable dependencies for the `agent` command tree.
type AgentCmdDeps struct {
	Settings         config.CrossPlatformSettings
	PluginBootstrap  pluginBootstrapProvider
	SandboxProbe     agentsandbox.Probe
	Executor         agentPluginExecutor
	DoctorProbe      agentdoctor.Probe
	LoginRunner      agentdoctor.LoginRunner
	SkillPackManager *skillpacks.Manager
}

type agentSkillOptions struct {
	agent      string
	scope      string
	projectDir string
	overwrite  bool
	dryRun     bool
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
	cmd.AddCommand(newAgentSkillsCmd(deps.SkillPackManager))
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
	destination, err := skillinstall.ResolveDestination(options.agent, options.scope, options.projectDir, agentskills.SkillName)
	if err != nil {
		return err
	}
	cmd.Printf("Destination: %s\n", destination.Path())
	skip, warning, err := skillinstall.CheckCodexKimiConflict(options.agent, options.scope, options.projectDir, agentskills.SkillName, "DevEx")
	if err != nil {
		return err
	}
	if warning != "" {
		cmd.PrintErr(warning)
	}
	if skip {
		cmd.Printf("Skipped: %s\n", destination.Path())
		return nil
	}
	return writeAgentSkill(cmd, destination, options)
}

func writeAgentSkill(cmd *cobra.Command, destination skillinstall.Destination, options agentSkillOptions) error {
	bundle, err := agentskills.Bundle()
	if err != nil {
		return err
	}
	root, rootExists, err := skillinstall.OpenRoot(destination, options.scope, options.dryRun)
	if err != nil {
		return err
	}
	if !rootExists {
		cmd.Printf("Would install: %s\n", destination.Path())
		return nil
	}
	defer func() { _ = root.Close() }()
	if err := skillinstall.ValidatePath(root, destination.Relative); err != nil {
		return err
	}

	existing := false
	identical := false
	info, err := root.Lstat(destination.Relative)
	if err == nil {
		existing = true
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("skill destination %q is not a regular directory", destination.Path())
		}
		expected, readErr := skillinstall.ReadBundle(bundle, ".")
		if readErr != nil {
			return fmt.Errorf("read embedded skill: %w", readErr)
		}
		actual, readErr := skillinstall.ReadBundle(root.FS(), destination.Relative)
		if readErr != nil {
			return fmt.Errorf("read installed skill: %w", readErr)
		}
		identical = reflect.DeepEqual(expected, actual)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect skill destination %q: %w", destination.Path(), err)
	}
	if identical {
		cmd.Printf("Already installed: %s\n", destination.Path())
		return nil
	}
	if existing && !options.overwrite {
		return fmt.Errorf("skill destination %q has differing content; use --overwrite to replace it and retain a backup", destination.Path())
	}
	if options.dryRun {
		if existing {
			cmd.Printf("Would replace: %s (existing content would be backed up)\n", destination.Path())
		} else {
			cmd.Printf("Would install: %s\n", destination.Path())
		}
		return nil
	}
	backupPath, err := skillinstall.StageAndPublish(cmd.Context(), root, destination, bundle, existing, true)
	if err != nil {
		return err
	}
	if existing {
		cmd.Printf("Replaced: %s\nBackup: %s\n", destination.Path(), backupPath)
	} else {
		cmd.Printf("Installed: %s\n", destination.Path())
	}
	return nil
}
