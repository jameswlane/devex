package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	toolAgentsPluginName = "tool-agents"
	agentsDocsURL        = "https://docs.devex.sh/configuration/agents"
)

type agentPluginExecutor interface {
	ExecuteAgentPlugin(ctx context.Context, pluginName string, args []string, streams sdk.OperationIO, allowDownload bool) error
}

type bootstrapAgentPluginExecutor struct {
	getBootstrap pluginBootstrapProvider
}

func (e bootstrapAgentPluginExecutor) ExecuteAgentPlugin(ctx context.Context, pluginName string, args []string, streams sdk.OperationIO, allowDownload bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var b *bootstrap.PluginBootstrap
	if e.getBootstrap != nil {
		b = e.getBootstrap()
	}
	if b == nil {
		var err error
		b, err = bootstrap.NewPluginBootstrap(!allowDownload || viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
		if err != nil {
			return fmt.Errorf("initialize plugin bootstrap: %w", err)
		}
		if err := b.Initialize(ctx); err != nil {
			return fmt.Errorf("discover plugins: %w", err)
		}
	}
	if allowDownload && !viper.GetBool("skip-plugin-download") && !viper.GetBool("offline") {
		if _, err := b.EnsurePluginWithContext(ctx, pluginName); err != nil {
			return fmt.Errorf("ensure %s plugin: %w", pluginName, err)
		}
	}
	if err := b.ExecutePluginWithIO(ctx, pluginName, args, streams); err != nil {
		if !allowDownload && strings.Contains(strings.ToLower(err.Error()), "not found") {
			return fmt.Errorf("%s plugin is not installed; install it with 'devex plugin install %s' or run 'devex agent apply' without --dry-run: %w", pluginName, pluginName, err)
		}
		return err
	}
	return nil
}

type resolvedAgentPluginSpec struct {
	Agents []resolvedAgentSpec `json:"agents"`
}

type resolvedAgentSpec struct {
	Name            string   `json:"name"`
	Preset          string   `json:"preset,omitempty"`
	Instructions    string   `json:"instructions,omitempty"`
	Model           string   `json:"model,omitempty"`
	AllowedCommands []string `json:"allowed_commands,omitempty"`
	DeniedCommands  []string `json:"denied_commands,omitempty"`
}

func newAgentApplyCmd(settings config.CrossPlatformSettings, executor agentPluginExecutor) *cobra.Command {
	var agentsFlag string
	var presetFlag string

	cmd := &cobra.Command{
		Use:          "apply",
		Short:        "Render and apply declarative configuration to installed coding agents",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Render shared global instructions, model defaults, permission presets, and
allowed/denied shell command rules from agents.yaml into each enabled,
installed coding agent's native configuration files.

Honors the global --dry-run flag (zero writes) and flags > env > config > defaults
precedence (DEVEX_AGENTS, DEVEX_AGENTS_PRESET, DEVEX_DRY_RUN).`,
		Example: `  # Apply agents.yaml to all enabled, installed coding agents
  devex agent apply

  # Preview changes without writing files, backups, or state
  devex agent apply --dry-run

  # Limit to specific enabled agents with a stricter preset
  devex agent apply --agents claude,codex --preset strict`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dryRun := viper.GetBool("dry-run")
			return runAgentPluginCommand(cmd, settings, executor, agentsFlag, presetFlag, dryRun)
		},
	}

	cmd.Flags().StringVar(&agentsFlag, "agents", "", "Comma-separated subset of enabled agents to target (e.g. claude,codex)")
	cmd.Flags().StringVar(&presetFlag, "preset", "", "Override configured permission preset (strict or balanced)")
	return cmd
}

func newAgentDiffCmd(settings config.CrossPlatformSettings, executor agentPluginExecutor) *cobra.Command {
	var agentsFlag string
	var presetFlag string

	cmd := &cobra.Command{
		Use:          "diff",
		Short:        "Preview changes to native coding-agent configuration files",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Preview what 'devex agent apply' would create or change in each enabled,
installed coding agent's native configuration files without writing any files,
backups, or state.`,
		Example: `  # Preview changes for all enabled agents
  devex agent diff

  # Preview changes for Claude Code only with the strict preset
  devex agent diff --agents claude --preset strict`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAgentPluginCommand(cmd, settings, executor, agentsFlag, presetFlag, true)
		},
	}

	cmd.Flags().StringVar(&agentsFlag, "agents", "", "Comma-separated subset of enabled agents to target (e.g. claude,codex)")
	cmd.Flags().StringVar(&presetFlag, "preset", "", "Override configured permission preset (strict or balanced)")
	return cmd
}

func runAgentPluginCommand(cmd *cobra.Command, settings config.CrossPlatformSettings, executor agentPluginExecutor, agentsFlag, presetFlag string, diffOnly bool) error {
	_ = viper.BindEnv("agents.preset", "DEVEX_AGENTS_PRESET")
	_ = viper.BindEnv("agents.selected", "DEVEX_AGENTS")

	cfg := settings.Agents
	if len(cfg.Enabled) == 0 && !cfg.IsConfigured() {
		agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
		cmd.Printf("No coding agents configured.\nCreate %s to declare enabled agents, shared instructions, presets, and command rules.\nDocumentation: %s\n", agentsPath, agentsDocsURL)
		return nil
	}

	if errs := config.ValidateAgentsConfig(cfg, "agents.yaml"); len(errs) > 0 {
		return fmt.Errorf("invalid agents configuration: %s", errs[0].Error())
	}

	if len(cfg.Enabled) == 0 {
		agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
		cmd.Printf("No coding agents enabled in %s.\nAdd one or more agents under agents.enabled (%s).\nDocumentation: %s\n",
			agentsPath, strings.Join(config.SupportedAgents(), ", "), agentsDocsURL)
		return nil
	}

	// Precedence: CLI flag > environment variable (via Viper) > config > defaults.
	presetOverride := ""
	if cmd.Flags().Changed("preset") {
		presetOverride = strings.TrimSpace(presetFlag)
	} else if envPreset := strings.TrimSpace(viper.GetString("agents.preset")); envPreset != "" {
		presetOverride = envPreset
	}
	if err := config.ValidateAgentPreset(presetOverride); err != nil {
		return err
	}

	selectedRaw := ""
	if cmd.Flags().Changed("agents") {
		selectedRaw = strings.TrimSpace(agentsFlag)
	} else if envAgents := strings.TrimSpace(viper.GetString("agents.selected")); envAgents != "" {
		selectedRaw = envAgents
	}

	targetAgents, err := resolveTargetAgents(cfg.Enabled, selectedRaw)
	if err != nil {
		return err
	}

	spec := buildResolvedAgentSpec(cfg, targetAgents, presetOverride)
	specBytes, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("encode resolved agents spec: %w", err)
	}
	specBytes = append(specBytes, '\n')

	tmpFile, err := os.CreateTemp("", "devex-agents-spec-*.json")
	if err != nil {
		return fmt.Errorf("create temporary agents spec: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("protect temporary agents spec: %w", err)
	}
	if _, err := tmpFile.Write(specBytes); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temporary agents spec: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temporary agents spec: %w", err)
	}

	pluginArgs := []string{"apply", "--config", tmpPath}
	allowDownload := true
	if diffOnly {
		pluginArgs = []string{"diff", "--config", tmpPath}
		allowDownload = false
	}

	streams := sdk.OperationIO{
		Stdin:  cmd.InOrStdin(),
		Stdout: cmd.OutOrStdout(),
		Stderr: cmd.ErrOrStderr(),
	}
	return executor.ExecuteAgentPlugin(cmd.Context(), toolAgentsPluginName, pluginArgs, streams, allowDownload)
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

func buildResolvedAgentSpec(cfg config.AgentsConfig, targetAgents []string, presetOverride string) resolvedAgentPluginSpec {
	out := resolvedAgentPluginSpec{
		Agents: make([]resolvedAgentSpec, 0, len(targetAgents)),
	}
	for _, name := range targetAgents {
		override := cfg.Overrides[name]
		preset := cfg.Preset
		if override.Preset != "" {
			preset = override.Preset
		}
		if presetOverride != "" {
			preset = presetOverride
		}
		out.Agents = append(out.Agents, resolvedAgentSpec{
			Name:            name,
			Preset:          preset,
			Instructions:    cfg.Instructions,
			Model:           override.Model,
			AllowedCommands: config.MergeAgentCommands(cfg.AllowedCommands, override.AllowedCommands),
			DeniedCommands:  config.MergeAgentCommands(cfg.DeniedCommands, override.DeniedCommands),
		})
	}
	return out
}
