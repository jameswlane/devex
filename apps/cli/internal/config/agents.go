package config

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/localmodels"
)

const (
	AgentClaude   = "claude"
	AgentCodex    = "codex"
	AgentGemini   = "gemini"
	AgentOpenCode = "opencode"

	AgentPresetStrict   = "strict"
	AgentPresetBalanced = "balanced"

	MCPTransportStdio = "stdio"
	MCPTransportHTTP  = "http"

	AgentBlockStart = "<!-- >>> devex agents >>> -->"
	AgentBlockEnd   = "<!-- <<< devex agents <<< -->"

	ErrAgentsSkipPermissions = "DevEx will not enable unsandboxed or skip-permission modes; set it in the agent's own config yourself"
)

// AgentsConfig declares shared coding-agent configuration and per-agent
// overrides in DevEx user configuration (agents.yaml).
type AgentsConfig struct {
	Enabled         []string                       `mapstructure:"enabled" yaml:"enabled"`
	Preset          string                         `mapstructure:"preset" yaml:"preset"`
	Instructions    string                         `mapstructure:"instructions" yaml:"instructions"`
	AllowedCommands []string                       `mapstructure:"allowed_commands" yaml:"allowed_commands"`
	DeniedCommands  []string                       `mapstructure:"denied_commands" yaml:"denied_commands"`
	Overrides       map[string]AgentOverrideConfig `mapstructure:"overrides" yaml:"overrides"`
	MCPServers      []AgentMCPServerConfig         `mapstructure:"mcp_servers" yaml:"mcp_servers"`
	DevExMCP        bool                           `mapstructure:"devex_mcp" yaml:"devex_mcp"`
	LocalModels     []string                       `mapstructure:"local_models" yaml:"local_models"`
}

// AgentMCPServerConfig declares one MCP server to render into target agents.
type AgentMCPServerConfig struct {
	Name      string            `mapstructure:"name" yaml:"name"`
	Transport string            `mapstructure:"transport" yaml:"transport"`
	Command   string            `mapstructure:"command" yaml:"command"`
	Args      []string          `mapstructure:"args" yaml:"args"`
	Env       map[string]string `mapstructure:"env" yaml:"env"`
	URL       string            `mapstructure:"url" yaml:"url"`
	Headers   map[string]string `mapstructure:"headers" yaml:"headers"`
	Agents    []string          `mapstructure:"agents" yaml:"agents"`
}

// AgentOverrideConfig overrides shared settings for a single coding agent.
type AgentOverrideConfig struct {
	Model           string   `mapstructure:"model" yaml:"model"`
	Preset          string   `mapstructure:"preset" yaml:"preset"`
	AllowedCommands []string `mapstructure:"allowed_commands" yaml:"allowed_commands"`
	DeniedCommands  []string `mapstructure:"denied_commands" yaml:"denied_commands"`
}

// AgentCapabilities lists which declarative fields have a native equivalent in
// a given coding-agent harness.
type AgentCapabilities struct {
	Instructions    bool
	Model           bool
	Preset          bool
	AllowedCommands bool
	DeniedCommands  bool
	MCPServers      bool
}

var agentCapabilities = map[string]AgentCapabilities{
	AgentClaude:   {Instructions: true, Model: true, Preset: true, AllowedCommands: true, DeniedCommands: true, MCPServers: true},
	AgentCodex:    {Instructions: true, Model: true, Preset: true, AllowedCommands: true, DeniedCommands: true, MCPServers: true},
	AgentGemini:   {Instructions: true, Model: true, Preset: true, AllowedCommands: true, DeniedCommands: true, MCPServers: true},
	AgentOpenCode: {Instructions: true, Model: true, Preset: true, AllowedCommands: true, DeniedCommands: true, MCPServers: true},
}

// SupportedAgents returns the canonical list of supported coding agents.
func SupportedAgents() []string {
	return []string{AgentClaude, AgentCodex, AgentGemini, AgentOpenCode}
}

// IsSupportedAgent reports whether name is a known coding agent.
func IsSupportedAgent(name string) bool {
	_, ok := agentCapabilities[name]
	return ok
}

// CapabilitiesForAgent returns the native field support matrix for agent.
func CapabilitiesForAgent(agent string) (AgentCapabilities, bool) {
	caps, ok := agentCapabilities[agent]
	return caps, ok
}

// IsForbiddenAgentPreset reports whether value attempts to enable an
// unsandboxed or skip-all-permissions mode.
func IsForbiddenAgentPreset(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "bypasspermissions",
		"bypass-permissions",
		"bypass_permissions",
		"dangerously-skip-permissions",
		"skip-permissions",
		"skip_permissions",
		"danger-full-access",
		"danger_full_access",
		"never",
		"yolo",
		"allow",
		"auto",
		"dontask",
		"dont-ask",
		"dont_ask":
		return true
	default:
		return false
	}
}

// IsConfigured reports whether any field in AgentsConfig was set.
func (c AgentsConfig) IsConfigured() bool {
	return len(c.Enabled) > 0 ||
		strings.TrimSpace(c.Preset) != "" ||
		strings.TrimSpace(c.Instructions) != "" ||
		len(c.AllowedCommands) > 0 ||
		len(c.DeniedCommands) > 0 ||
		len(c.Overrides) > 0 ||
		len(c.MCPServers) > 0 ||
		c.DevExMCP ||
		len(c.LocalModels) > 0
}

// ValidateAgentsConfig validates values in AgentsConfig and returns structured
// validation errors suitable for `devex config validate` and `devex agent`.
func ValidateAgentsConfig(cfg AgentsConfig, file string) []ValidationError {
	return validateAgentsConfigWithCaps(cfg, file, agentCapabilities)
}

func validateAgentsConfigWithCaps(cfg AgentsConfig, file string, capsMap map[string]AgentCapabilities) []ValidationError {
	if !cfg.IsConfigured() {
		return nil
	}
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{File: file, Field: field, Message: msg})
	}

	seenEnabled := make(map[string]bool, len(cfg.Enabled))
	for i, name := range cfg.Enabled {
		field := fmt.Sprintf("agents.enabled[%d]", i)
		if _, ok := capsMap[name]; !ok {
			add(field, fmt.Sprintf("unsupported agent %q (supported: %s)", name, strings.Join(SupportedAgents(), ", ")))
			continue
		}
		if seenEnabled[name] {
			add(field, fmt.Sprintf("duplicate agent %q", name))
			continue
		}
		seenEnabled[name] = true
	}

	if errMsg := validateAgentPresetValue(cfg.Preset); errMsg != "" {
		add("agents.preset", errMsg)
	}

	if cfg.Instructions != "" {
		if strings.Contains(cfg.Instructions, AgentBlockStart) || strings.Contains(cfg.Instructions, AgentBlockEnd) {
			add("agents.instructions", fmt.Sprintf("instructions must not contain managed block markers (%s or %s)", AgentBlockStart, AgentBlockEnd))
		}
	}

	for i, cmd := range cfg.AllowedCommands {
		if errMsg := validateAgentCommandValue(cmd); errMsg != "" {
			add(fmt.Sprintf("agents.allowed_commands[%d]", i), errMsg)
		}
	}
	for i, cmd := range cfg.DeniedCommands {
		if errMsg := validateAgentCommandValue(cmd); errMsg != "" {
			add(fmt.Sprintf("agents.denied_commands[%d]", i), errMsg)
		}
	}

	// Check that shared fields are supported by every enabled agent.
	for _, name := range cfg.Enabled {
		caps, ok := capsMap[name]
		if !ok {
			continue
		}
		if strings.TrimSpace(cfg.Instructions) != "" && !caps.Instructions {
			add("agents.instructions", fmt.Sprintf("agent %q has no native equivalent for instructions", name))
		}
		if cfg.Preset != "" && !caps.Preset {
			add("agents.preset", fmt.Sprintf("agent %q has no native equivalent for preset", name))
		}
		if len(cfg.AllowedCommands) > 0 && !caps.AllowedCommands {
			add("agents.allowed_commands", fmt.Sprintf("agent %q has no native equivalent for allowed_commands", name))
		}
		if len(cfg.DeniedCommands) > 0 && !caps.DeniedCommands {
			add("agents.denied_commands", fmt.Sprintf("agent %q has no native equivalent for denied_commands", name))
		}
	}

	overrideNames := make([]string, 0, len(cfg.Overrides))
	for name := range cfg.Overrides {
		overrideNames = append(overrideNames, name)
	}
	sort.Strings(overrideNames)

	for _, name := range overrideNames {
		override := cfg.Overrides[name]
		prefix := "agents.overrides." + name
		caps, ok := capsMap[name]
		if !ok {
			add(prefix, fmt.Sprintf("unsupported agent %q (supported: %s)", name, strings.Join(SupportedAgents(), ", ")))
			continue
		}
		if override.Model != "" {
			if !caps.Model {
				add(prefix+".model", fmt.Sprintf("agent %q has no native equivalent for model", name))
			} else if errMsg := validateAgentModelValue(override.Model); errMsg != "" {
				add(prefix+".model", errMsg)
			}
		}
		if override.Preset != "" {
			if !caps.Preset {
				add(prefix+".preset", fmt.Sprintf("agent %q has no native equivalent for preset", name))
			} else if errMsg := validateAgentPresetValue(override.Preset); errMsg != "" {
				add(prefix+".preset", errMsg)
			}
		}
		if len(override.AllowedCommands) > 0 && !caps.AllowedCommands {
			add(prefix+".allowed_commands", fmt.Sprintf("agent %q has no native equivalent for allowed_commands", name))
		}
		for i, cmd := range override.AllowedCommands {
			if errMsg := validateAgentCommandValue(cmd); errMsg != "" {
				add(fmt.Sprintf("%s.allowed_commands[%d]", prefix, i), errMsg)
			}
		}
		if len(override.DeniedCommands) > 0 && !caps.DeniedCommands {
			add(prefix+".denied_commands", fmt.Sprintf("agent %q has no native equivalent for denied_commands", name))
		}
		for i, cmd := range override.DeniedCommands {
			if errMsg := validateAgentCommandValue(cmd); errMsg != "" {
				add(fmt.Sprintf("%s.denied_commands[%d]", prefix, i), errMsg)
			}
		}
	}

	validateAgentMCPServers(cfg, seenEnabled, capsMap, add)

	seenModels := make(map[string]bool, len(cfg.LocalModels))
	for i, model := range cfg.LocalModels {
		field := fmt.Sprintf("agents.local_models[%d]", i)
		if err := localmodels.ValidateModelName(model); err != nil {
			add(field, err.Error())
			continue
		}
		canonical := localmodels.CanonicalModelName(model)
		if seenModels[canonical] {
			add(field, fmt.Sprintf("duplicate local model %q", model))
			continue
		}
		seenModels[canonical] = true
	}

	return errs
}

// ValidateAgentPreset validates a single preset string (for CLI flags/env too).
func ValidateAgentPreset(preset string) error {
	if msg := validateAgentPresetValue(preset); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func validateAgentPresetValue(preset string) string {
	if preset == "" {
		return ""
	}
	if IsForbiddenAgentPreset(preset) {
		return fmt.Sprintf("invalid preset %q: %s", preset, ErrAgentsSkipPermissions)
	}
	if preset != AgentPresetStrict && preset != AgentPresetBalanced {
		return fmt.Sprintf("unsupported preset %q (supported: %s, %s)", preset, AgentPresetStrict, AgentPresetBalanced)
	}
	return ""
}

func validateAgentCommandValue(cmd string) string {
	if strings.TrimSpace(cmd) == "" {
		return "command must not be empty"
	}
	if strings.ContainsAny(cmd, "\r\n") || hasAgentControlChars(cmd) {
		return fmt.Sprintf("command %q must be a single line without control characters", cmd)
	}
	return ""
}

func validateAgentModelValue(model string) string {
	if strings.TrimSpace(model) == "" || strings.ContainsAny(model, "\r\n\t") || hasAgentControlChars(model) {
		return fmt.Sprintf("invalid model %q: must be a single-line non-empty string without control characters", model)
	}
	return ""
}

func hasAgentControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// MergeAgentCommands appends override commands to shared commands without duplicates.
func MergeAgentCommands(shared, extra []string) []string {
	out := make([]string, 0, len(shared)+len(extra))
	for _, cmd := range shared {
		if !slices.Contains(out, cmd) {
			out = append(out, cmd)
		}
	}
	for _, cmd := range extra {
		if !slices.Contains(out, cmd) {
			out = append(out, cmd)
		}
	}
	return out
}
