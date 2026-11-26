package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	AgentClaude   = "claude"
	AgentCodex    = "codex"
	AgentGemini   = "gemini"
	AgentOpenCode = "opencode"

	PresetStrict   = "strict"
	PresetBalanced = "balanced"

	MCPTransportStdio = "stdio"
	MCPTransportHTTP  = "http"

	ErrSkipPermissionsText = "DevEx will not enable unsandboxed or skip-permission modes; set it in the agent's own config yourself"
)

// Spec is the resolved declarative agent configuration passed by the CLI.
type Spec struct {
	Agents []AgentSpec `json:"agents"`
}

// AgentSpec describes the resolved configuration for one coding agent.
type AgentSpec struct {
	Name            string          `json:"name"`
	Preset          string          `json:"preset,omitempty"`
	Instructions    string          `json:"instructions,omitempty"`
	Model           string          `json:"model,omitempty"`
	AllowedCommands []string        `json:"allowed_commands,omitempty"`
	DeniedCommands  []string        `json:"denied_commands,omitempty"`
	MCPServers      []MCPServerSpec `json:"mcp_servers,omitempty"`
}

// MCPServerSpec describes one MCP server resolved for an agent.
type MCPServerSpec struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// SupportedAgents returns the canonical list of supported v1 agents.
func SupportedAgents() []string {
	return []string{AgentClaude, AgentCodex, AgentGemini, AgentOpenCode}
}

// IsSupportedAgent reports whether name is a supported agent identifier.
func IsSupportedAgent(name string) bool {
	switch name {
	case AgentClaude, AgentCodex, AgentGemini, AgentOpenCode:
		return true
	default:
		return false
	}
}

// IsForbiddenPermissionMode reports whether value attempts to enable a
// skip-permission or unsandboxed mode in any supported harness.
func IsForbiddenPermissionMode(value string) bool {
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

// LoadSpec reads and strictly validates a JSON spec file.
func LoadSpec(path string) (Spec, error) {
	if strings.TrimSpace(path) == "" {
		return Spec{}, fmt.Errorf("--config file path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return ParseSpec(data)
}

// ParseSpec decodes and validates a JSON spec payload.
func ParseSpec(data []byte) (Spec, error) {
	var spec Spec
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return Spec{}, fmt.Errorf("decode agents spec: %w", err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Spec{}, fmt.Errorf("decode agents spec: unexpected trailing JSON data")
	}
	if err := spec.Validate(); err != nil {
		return Spec{}, err
	}
	return spec, nil
}

// Validate checks every agent entry in the spec for validity and safety.
func (s Spec) Validate() error {
	seen := make(map[string]bool, len(s.Agents))
	for i, agent := range s.Agents {
		name := strings.TrimSpace(agent.Name)
		if !IsSupportedAgent(name) {
			return fmt.Errorf("agents[%d]: unsupported agent %q (supported: %s)", i, agent.Name, strings.Join(SupportedAgents(), ", "))
		}
		if name != agent.Name {
			return fmt.Errorf("agents[%d]: agent name %q must not have surrounding whitespace", i, agent.Name)
		}
		if seen[name] {
			return fmt.Errorf("agents[%d]: duplicate agent %q", i, name)
		}
		seen[name] = true

		if err := validateAgentSpec(agent); err != nil {
			return fmt.Errorf("agent %s: %w", name, err)
		}
	}
	return nil
}

func validateAgentSpec(agent AgentSpec) error {
	if agent.Preset != "" {
		if IsForbiddenPermissionMode(agent.Preset) {
			return fmt.Errorf("preset %q: %s", agent.Preset, ErrSkipPermissionsText)
		}
		if agent.Preset != PresetStrict && agent.Preset != PresetBalanced {
			return fmt.Errorf("unsupported preset %q (supported: %s, %s)", agent.Preset, PresetStrict, PresetBalanced)
		}
	}

	if agent.Model != "" {
		if strings.TrimSpace(agent.Model) == "" || strings.ContainsAny(agent.Model, "\r\n\t") || hasControlChars(agent.Model) {
			return fmt.Errorf("invalid model %q: must be a single-line non-empty string without control characters", agent.Model)
		}
	}

	if agent.Instructions != "" {
		if strings.Contains(agent.Instructions, BlockStart) || strings.Contains(agent.Instructions, BlockEnd) {
			return fmt.Errorf("instructions must not contain managed block markers (%s or %s)", BlockStart, BlockEnd)
		}
	}

	if err := validateCommands("allowed_commands", agent.AllowedCommands); err != nil {
		return err
	}
	if err := validateCommands("denied_commands", agent.DeniedCommands); err != nil {
		return err
	}
	if err := validateMCPServers(agent.MCPServers); err != nil {
		return err
	}
	return nil
}

func validateCommands(field string, commands []string) error {
	for i, cmd := range commands {
		if strings.TrimSpace(cmd) == "" {
			return fmt.Errorf("%s[%d]: command must not be empty", field, i)
		}
		if strings.ContainsAny(cmd, "\r\n") || hasControlChars(cmd) {
			return fmt.Errorf("%s[%d]: command %q must be a single line without control characters", field, i, cmd)
		}
	}
	return nil
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
