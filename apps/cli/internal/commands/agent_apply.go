package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/agentapply"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/localmodels"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	toolAgentsPluginName = agentapply.ToolAgentsPluginName
	agentsDocsURL        = "https://docs.devex.sh/configuration/agents"

	// Read directly: root binds every flag into viper, and the --agents flag
	// shadows nested "agents.*" keys, so viper never returns these values.
	agentsSelectEnv = "DEVEX_AGENTS"
	agentsPresetEnv = "DEVEX_AGENTS_PRESET"
)

type agentPluginExecutor = agentapply.PluginExecutor

type resolvedAgentPluginSpec = agentapply.ResolvedPluginSpec
type resolvedAgentSpec = agentapply.ResolvedAgentSpec
type resolvedMCPServerSpec = agentapply.ResolvedMCPServerSpec

var (
	agentLookPath   = exec.LookPath
	agentExecutable = os.Executable
)

func newAgentApplyCmd(settings config.CrossPlatformSettings, executor agentPluginExecutor, lmProbe *localmodels.Probe, lmOpts localmodels.PlanOptions) *cobra.Command {
	var agentsFlag string
	var presetFlag string
	var initFlag bool

	cmd := &cobra.Command{
		Use:          "apply",
		Short:        "Render and apply declarative configuration to installed coding agents",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Render shared global instructions, model defaults, permission presets,
allowed/denied shell command rules, and MCP servers from agents.yaml into each
enabled, installed coding agent's native configuration files, then pull any
missing agents.local_models into the local Ollama runtime.

Honors the global --dry-run flag (zero writes and zero pulls) and
flags > env > config > defaults precedence (DEVEX_AGENTS, DEVEX_AGENTS_PRESET,
DEVEX_DRY_RUN).`,
		Example: `  # Apply agents.yaml to all enabled, installed coding agents
  devex agent apply

  # Preview changes without writing files, backups, state, or pulling models
  devex agent apply --dry-run

  # Limit to specific enabled agents with a stricter preset
  devex agent apply --agents claude,codex --preset strict`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dryRun := viper.GetBool("dry-run")
			if initFlag {
				return runAgentApplyInit(cmd, settings, executor, agentsFlag, presetFlag, dryRun)
			}
			return runAgentPluginCommand(cmd, settings, executor, lmProbe, lmOpts, agentsFlag, presetFlag, dryRun)
		},
	}

	cmd.Flags().StringVar(&agentsFlag, "agents", "", "Comma-separated subset of enabled agents to target (e.g. claude,codex)")
	cmd.Flags().StringVar(&presetFlag, "preset", "", "Override configured permission preset (strict or balanced)")
	cmd.Flags().BoolVar(&initFlag, "init", false, "Write a minimal agents.yaml for --agents if absent (preserving an existing file) before applying")
	return cmd
}

func newAgentDiffCmd(settings config.CrossPlatformSettings, executor agentPluginExecutor, lmProbe *localmodels.Probe, lmOpts localmodels.PlanOptions) *cobra.Command {
	var agentsFlag string
	var presetFlag string

	cmd := &cobra.Command{
		Use:          "diff",
		Short:        "Preview changes to native coding-agent configuration files",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Preview what 'devex agent apply' would create or change in each enabled,
installed coding agent's native configuration files, and which local Ollama
models it would pull, without writing any files, backups, or state.`,
		Example: `  # Preview changes for all enabled agents
  devex agent diff

  # Preview changes for Claude Code only with the strict preset
  devex agent diff --agents claude --preset strict`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAgentPluginCommand(cmd, settings, executor, lmProbe, lmOpts, agentsFlag, presetFlag, true)
		},
	}

	cmd.Flags().StringVar(&agentsFlag, "agents", "", "Comma-separated subset of enabled agents to target (e.g. claude,codex)")
	cmd.Flags().StringVar(&presetFlag, "preset", "", "Override configured permission preset (strict or balanced)")
	return cmd
}

func runAgentApplyInit(cmd *cobra.Command, settings config.CrossPlatformSettings, executor agentPluginExecutor, agentsFlag, presetFlag string, dryRun bool) error {
	preset := ""
	if cmd.Flags().Changed("preset") {
		preset = strings.TrimSpace(presetFlag)
	} else if envPreset := strings.TrimSpace(os.Getenv(agentsPresetEnv)); envPreset != "" {
		preset = envPreset
	}
	if err := config.ValidateAgentPreset(preset); err != nil {
		return err
	}

	selectedRaw := ""
	if cmd.Flags().Changed("agents") {
		selectedRaw = strings.TrimSpace(agentsFlag)
	} else if envAgents := strings.TrimSpace(os.Getenv(agentsSelectEnv)); envAgents != "" {
		selectedRaw = envAgents
	}
	var harnessIDs []string
	for _, raw := range strings.Split(selectedRaw, ",") {
		if id, ok := agentapply.HarnessIDForNameOrSlug(raw); ok {
			harnessIDs = append(harnessIDs, id)
		}
	}
	harnessIDs = agentapply.CanonicalHarnessIDs(harnessIDs)

	userConfigDir := settings.GetUserConfigDir()
	agentsPath := filepath.Join(userConfigDir, "agents.yaml")
	exists := settings.Agents.IsConfigured()
	if !exists {
		if _, err := os.Stat(agentsPath); err == nil {
			exists = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", agentsPath, err)
		}
	}

	agentapply.EnsureUserBinDirsOnPath(settings.HomeDir)
	if exists {
		// The loaded settings already merge every configuration tier, so apply
		// them as they are instead of re-reading one file.
		cmd.Printf("Kept your existing agents configuration (%s); run 'devex agent diff' to review.\n", agentsPath)
		if dryRun {
			return nil
		}
		if len(settings.Agents.Enabled) == 0 {
			cmd.Printf("No coding agents enabled in %s; add them under agents.enabled.\n", agentsPath)
			return nil
		}
		if errs := config.ValidateAgentsConfig(settings.Agents, "agents.yaml"); len(errs) > 0 {
			return fmt.Errorf("invalid agents configuration: %s", errs[0].Error())
		}
		return applyResolvedAgents(cmd, settings.Agents, settings.Agents.Enabled, executor)
	}

	if len(harnessIDs) == 0 {
		cmd.Printf("No configurable harness agents selected; skipping agents.yaml creation.\n")
		return nil
	}
	if dryRun {
		data, renderErr := agentapply.RenderMinimalAgentsYAML(harnessIDs, preset)
		if renderErr != nil {
			return renderErr
		}
		cmd.Printf("Would write %s:\n%s", agentsPath, string(data))
		return nil
	}
	_, created, err := agentapply.WriteDefaultAgentsYAMLIfMissing(userConfigDir, harnessIDs, preset)
	if err != nil {
		return err
	}
	if !created {
		cmd.Printf("%s appeared while writing it and was left unchanged; run 'devex agent apply' to apply it.\n", agentsPath)
		return nil
	}
	cfg := config.AgentsConfig{Enabled: harnessIDs, Preset: preset}
	return applyResolvedAgents(cmd, cfg, harnessIDs, executor)
}

// applyResolvedAgents applies cfg to targets that --init already resolved, so
// the raw --agents/--preset values (which may be catalog slugs) are not re-read.
func applyResolvedAgents(cmd *cobra.Command, cfg config.AgentsConfig, targets []string, executor agentPluginExecutor) error {
	spec := agentapply.BuildResolvedSpec(cfg, targets, "", agentLookPath, agentExecutable)
	streams := sdk.OperationIO{Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()}
	return agentapply.Execute(cmd.Context(), executor, spec, streams, false)
}

func runAgentPluginCommand(cmd *cobra.Command, settings config.CrossPlatformSettings, executor agentPluginExecutor, lmProbe *localmodels.Probe, lmOpts localmodels.PlanOptions, agentsFlag, presetFlag string, diffOnly bool) error {
	cfg := settings.Agents
	if len(cfg.Enabled) == 0 && !cfg.IsConfigured() {
		agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
		cmd.Printf("No coding agents configured.\nCreate %s to declare enabled agents, shared instructions, presets, and command rules.\nDocumentation: %s\n", agentsPath, agentsDocsURL)
		return nil
	}

	if errs := config.ValidateAgentsConfig(cfg, "agents.yaml"); len(errs) > 0 {
		return fmt.Errorf("invalid agents configuration: %s", errs[0].Error())
	}

	if len(cfg.Enabled) == 0 && len(cfg.LocalModels) == 0 {
		agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
		cmd.Printf("No coding agents enabled in %s.\nAdd one or more agents under agents.enabled (%s).\nDocumentation: %s\n",
			agentsPath, strings.Join(config.SupportedAgents(), ", "), agentsDocsURL)
		return nil
	}

	// Precedence: CLI flag > environment variable (via Viper) > config > defaults.
	presetOverride := ""
	if cmd.Flags().Changed("preset") {
		presetOverride = strings.TrimSpace(presetFlag)
	} else if envPreset := strings.TrimSpace(os.Getenv(agentsPresetEnv)); envPreset != "" {
		presetOverride = envPreset
	}
	if err := config.ValidateAgentPreset(presetOverride); err != nil {
		return err
	}

	selectedRaw := ""
	if cmd.Flags().Changed("agents") {
		selectedRaw = strings.TrimSpace(agentsFlag)
	} else if envAgents := strings.TrimSpace(os.Getenv(agentsSelectEnv)); envAgents != "" {
		selectedRaw = envAgents
	}

	var agentsErr error
	if len(cfg.Enabled) > 0 || selectedRaw != "" {
		targetAgents, err := resolveTargetAgents(cfg.Enabled, selectedRaw)
		if err != nil {
			return err
		}

		spec := agentapply.BuildResolvedSpec(cfg, targetAgents, presetOverride, agentLookPath, agentExecutable)
		streams := sdk.OperationIO{
			Stdin:  cmd.InOrStdin(),
			Stdout: cmd.OutOrStdout(),
			Stderr: cmd.ErrOrStderr(),
		}
		agentsErr = agentapply.Execute(cmd.Context(), executor, spec, streams, diffOnly)
	}
	if len(cfg.LocalModels) == 0 {
		return agentsErr
	}
	// Local models do not depend on the tool-agents plugin, so report them
	// even when agent configuration failed.
	if len(cfg.Enabled) > 0 {
		cmd.Println()
	}
	return errors.Join(agentsErr, runLocalModels(cmd, cfg.LocalModels, lmProbe, lmOpts, diffOnly))
}

// runLocalModels previews (diffOnly) or pulls the configured local models.
func runLocalModels(cmd *cobra.Command, models []string, lmProbe *localmodels.Probe, lmOpts localmodels.PlanOptions, diffOnly bool) error {
	probe := localmodels.DefaultProbe()
	if lmProbe != nil {
		probe = *lmProbe
	}
	offline := lmOpts.Offline || viper.GetBool("offline")
	if diffOnly {
		opts := lmOpts
		opts.Offline = offline
		plan, err := localmodels.BuildPlan(cmd.Context(), probe, models, opts)
		if err != nil {
			return err
		}
		cmd.Print(localmodels.FormatPlan(plan))
		return nil
	}
	if offline {
		cmd.Println("Local models: skipped pulling in offline mode.")
		return nil
	}
	return localmodels.PullModels(cmd.Context(), probe, models, localmodels.PullOptions{
		OllamaBaseURL: lmOpts.OllamaBaseURL,
	}, cmd.OutOrStdout())
}

func resolveTargetAgents(enabled []string, filterCSV string) ([]string, error) {
	if filterCSV == "" {
		return slices.Clone(enabled), nil
	}
	parts := strings.Split(filterCSV, ",")
	targets := make([]string, 0, len(parts))
	for _, raw := range parts {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !config.IsSupportedAgent(name) {
			return nil, fmt.Errorf("unsupported agent %q in --agents (supported: %s)", name, strings.Join(config.SupportedAgents(), ", "))
		}
		if !slices.Contains(enabled, name) {
			return nil, fmt.Errorf("agent %q is not enabled in agents.yaml (enabled: %s)", name, strings.Join(enabled, ", "))
		}
		if !slices.Contains(targets, name) {
			targets = append(targets, name)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("--agents must specify at least one enabled agent")
	}
	return targets, nil
}
