// Package agentdoctor diagnoses AI coding agent installations, PATH shadowing,
// authentication status, sandbox prerequisites, Node.js requirements, and MCP
// configuration syntax without ever reading, storing, or printing secrets.
package agentdoctor

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"gopkg.in/yaml.v3"
)

// MCPFormat identifies the serialization format of an agent's MCP config file.
type MCPFormat string

const (
	MCPFormatJSON MCPFormat = "json"
	MCPFormatTOML MCPFormat = "toml"
	MCPFormatYAML MCPFormat = "yaml"
	// MCPFormatJSONC is JSON with // and /* */ comments and trailing commas,
	// which OpenCode accepts in opencode.jsonc.
	MCPFormatJSONC MCPFormat = "jsonc"
)

// PathTemplate describes how to resolve a user-level file path from the
// injected Probe environment without reading the host environment directly.
type PathTemplate struct {
	// EnvDirVar, when set in the environment, replaces the base directory
	// (e.g., CODEX_HOME replaces ~/.codex).
	EnvDirVar string
	// XDGEnvVar, when set in the environment, supplies an XDG root such as
	// XDG_CONFIG_HOME or XDG_DATA_HOME, combined with XDGSubPath.
	XDGEnvVar string
	// XDGSubPath is joined under XDGEnvVar when set, or under DefaultXDGDir
	// inside the home directory otherwise.
	XDGSubPath string
	// DefaultXDGDir is the home-relative XDG fallback (e.g., ".config" or ".local/share").
	DefaultXDGDir string
	// HomeRelPath is the default home-relative path when neither EnvDirVar nor
	// XDGEnvVar applies (e.g., ".claude/.credentials.json").
	HomeRelPath string
	// EnvRelPath is the relative path joined under EnvDirVar when EnvDirVar is set.
	EnvRelPath string
}

// Resolve expands PathTemplate using probe's environment and home directory.
// Returns an empty string when no base directory can be determined.
func (t PathTemplate) Resolve(probe Probe) string {
	if t.EnvDirVar != "" {
		if dir := strings.TrimSpace(probe.Getenv(t.EnvDirVar)); dir != "" {
			if t.EnvRelPath == "" {
				return filepath.Clean(dir)
			}
			return filepath.Join(dir, filepath.FromSlash(t.EnvRelPath))
		}
	}
	if t.XDGEnvVar != "" {
		if base := strings.TrimSpace(probe.Getenv(t.XDGEnvVar)); base != "" {
			return filepath.Join(base, filepath.FromSlash(t.XDGSubPath))
		}
		home := probe.homeDir()
		if home == "" {
			return ""
		}
		return filepath.Join(home, filepath.FromSlash(t.DefaultXDGDir), filepath.FromSlash(t.XDGSubPath))
	}
	home := probe.homeDir()
	if home == "" {
		return ""
	}
	return filepath.Join(home, filepath.FromSlash(t.HomeRelPath))
}

// MCPConfigSpec declares a single MCP configuration file an agent reads.
type MCPConfigSpec struct {
	Path       PathTemplate
	Format     MCPFormat
	ServersKey string
}

// AgentSpec captures verified metadata for one cataloged AI coding agent.
type AgentSpec struct {
	ID              string
	Name            string
	CatalogFile     string
	Binary          string
	VersionArgs     []string
	LoginStatusArgs []string
	LoginArgs       []string
	NoLoginGuidance string
	CredentialPaths []PathTemplate
	UsesKeyring     bool
	// OtherAuthStore names a credential store DevEx cannot inspect
	// (besides the OS keyring); a missing credential file then means unknown.
	OtherAuthStore string
	// EnvVarsDoNotAuthenticate marks agents whose API-key variables are not
	// consumed automatically, so a set variable is not a login signal.
	EnvVarsDoNotAuthenticate bool
	APIKeyEnvVars            []string
	MCPConfigs               []MCPConfigSpec
	NPMPackage               string
	MinNodeVersion           string
	NodeRequired             bool
	SandboxPrerequisites     []string
}

// knownAgentDefs is the static registry of AI coding agents matching
// apps/cli/config/applications/ai-agents/*.yaml. Every fact below is verified
// against vendor documentation; unconfirmed fields are intentionally omitted so
// doctor reports "unknown" instead of guessing.
var knownAgentDefs = []AgentSpec{
	{
		// Binary: apps/cli/config/applications/ai-agents/claude-code.yaml (installed_binary: claude).
		// Auth commands (`claude auth status`, `claude auth login`), ANTHROPIC_API_KEY / ANTHROPIC_AUTH_TOKEN,
		// and ~/.claude/.credentials.json (or $CLAUDE_CONFIG_DIR/.credentials.json):
		//   https://code.claude.com/docs/en/cli-reference
		//   https://code.claude.com/docs/en/authentication
		// MCP config (~/.claude.json, key "mcpServers"):
		//   https://code.claude.com/docs/en/mcp
		// Legacy npm package @anthropic-ai/claude-code (Node >= 18):
		//   https://www.npmjs.com/package/@anthropic-ai/claude-code
		ID:              "claude",
		Name:            "Claude Code",
		CatalogFile:     "claude-code.yaml",
		Binary:          "claude",
		VersionArgs:     []string{"--version"},
		LoginStatusArgs: []string{"auth", "status"},
		LoginArgs:       []string{"auth", "login"},
		CredentialPaths: []PathTemplate{
			{
				EnvDirVar:   "CLAUDE_CONFIG_DIR",
				EnvRelPath:  ".credentials.json",
				HomeRelPath: ".claude/.credentials.json",
			},
		},
		UsesKeyring:   true,
		APIKeyEnvVars: []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"},
		MCPConfigs: []MCPConfigSpec{
			{
				Path: PathTemplate{
					EnvDirVar:   "CLAUDE_CONFIG_DIR",
					EnvRelPath:  ".claude.json",
					HomeRelPath: ".claude.json",
				},
				Format:     MCPFormatJSON,
				ServersKey: "mcpServers",
			},
		},
		NPMPackage:     "@anthropic-ai/claude-code",
		MinNodeVersion: "18.0.0",
	},
	{
		// Binary and npm package (@openai/codex): apps/cli/config/applications/ai-agents/codex.yaml.
		// Auth (`codex login`, `codex login status`, $CODEX_HOME/auth.json, OPENAI_API_KEY, CODEX_API_KEY)
		// and MCP config ($CODEX_HOME/config.toml, key "mcp_servers"):
		//   https://developers.openai.com/codex/cli/reference
		//   https://developers.openai.com/codex/mcp
		ID:              "codex",
		Name:            "Codex CLI",
		CatalogFile:     "codex.yaml",
		Binary:          "codex",
		VersionArgs:     []string{"--version"},
		LoginStatusArgs: []string{"login", "status"},
		LoginArgs:       []string{"login"},
		CredentialPaths: []PathTemplate{
			{
				EnvDirVar:   "CODEX_HOME",
				EnvRelPath:  "auth.json",
				HomeRelPath: ".codex/auth.json",
			},
		},
		UsesKeyring:   true,
		APIKeyEnvVars: []string{"OPENAI_API_KEY", "CODEX_API_KEY"},
		MCPConfigs: []MCPConfigSpec{
			{
				Path: PathTemplate{
					EnvDirVar:   "CODEX_HOME",
					EnvRelPath:  "config.toml",
					HomeRelPath: ".codex/config.toml",
				},
				Format:     MCPFormatTOML,
				ServersKey: "mcp_servers",
			},
		},
		NPMPackage: "@openai/codex",
	},
	{
		// Package (@google/gemini-cli) and Node.js dependency: apps/cli/config/applications/ai-agents/gemini-cli.yaml.
		// Node.js >= 20, OAuth credential cache (~/.gemini/oauth_creds.json), GEMINI_API_KEY / GOOGLE_API_KEY,
		// interactive /auth (no standalone CLI login subcommand), and MCP config (~/.gemini/settings.json, key "mcpServers"):
		//   https://geminicli.com/docs/get-started/authentication/
		//   https://geminicli.com/docs/tools/mcp-server/
		//   https://github.com/google-gemini/gemini-cli
		ID:              "gemini",
		Name:            "Gemini CLI",
		CatalogFile:     "gemini-cli.yaml",
		Binary:          "gemini",
		VersionArgs:     []string{"--version"},
		NoLoginGuidance: `Gemini CLI does not have a standalone login subcommand. Start "gemini" and run /auth for Google OAuth, or export GEMINI_API_KEY (or GOOGLE_API_KEY with GOOGLE_GENAI_USE_VERTEXAI=true) from your secret manager.`,
		CredentialPaths: []PathTemplate{
			{EnvDirVar: "GEMINI_CLI_HOME", EnvRelPath: ".gemini/oauth_creds.json", HomeRelPath: ".gemini/oauth_creds.json"},
		},
		APIKeyEnvVars: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
		MCPConfigs: []MCPConfigSpec{
			{
				// GEMINI_CLI_HOME replaces the home directory, and settings.json
				// accepts comments: https://geminicli.com/docs/reference/configuration/
				Path:       PathTemplate{EnvDirVar: "GEMINI_CLI_HOME", EnvRelPath: ".gemini/settings.json", HomeRelPath: ".gemini/settings.json"},
				Format:     MCPFormatJSONC,
				ServersKey: "mcpServers",
			},
		},
		NPMPackage:     "@google/gemini-cli",
		MinNodeVersion: "20.0.0",
		NodeRequired:   true,
	},
	{
		// Binary: apps/cli/config/applications/ai-agents/copilot-cli.yaml (installed_binary: copilot).
		// Auth (`copilot login`, COPILOT_GITHUB_TOKEN / GH_TOKEN / GITHUB_TOKEN, OS keyring storage),
		// MCP config ($COPILOT_HOME/mcp-config.json or ~/.copilot/mcp-config.json, key "mcpServers"),
		// and npm package @github/copilot (Node >= 22):
		//   https://docs.github.com/en/copilot/how-tos/copilot-cli/use-copilot-cli
		//   https://github.com/github/copilot-cli
		// Note: no confirmed standalone non-interactive login-status subcommand; credential file
		// is omitted because ~/.copilot/config.json is a general config file and OAuth uses OS keyring.
		ID:            "copilot",
		Name:          "GitHub Copilot CLI",
		CatalogFile:   "copilot-cli.yaml",
		Binary:        "copilot",
		VersionArgs:   []string{"--version"},
		LoginArgs:     []string{"login"},
		UsesKeyring:   true,
		APIKeyEnvVars: []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"},
		MCPConfigs: []MCPConfigSpec{
			{
				Path: PathTemplate{
					EnvDirVar:   "COPILOT_HOME",
					EnvRelPath:  "mcp-config.json",
					HomeRelPath: ".copilot/mcp-config.json",
				},
				Format:     MCPFormatJSON,
				ServersKey: "mcpServers",
			},
		},
		NPMPackage:     "@github/copilot",
		MinNodeVersion: "22.0.0",
	},
	{
		// Binary: apps/cli/config/applications/ai-agents/opencode.yaml (installed_binary: opencode).
		// Auth (`opencode auth login`, ~/.local/share/opencode/auth.json),
		// MCP config (~/.config/opencode/opencode.json, key "mcp"), and npm package opencode-ai:
		//   https://opencode.ai/docs/providers/
		//   https://opencode.ai/docs/mcp-servers/
		//   https://opencode.ai/docs/config/
		ID:          "opencode",
		Name:        "OpenCode",
		CatalogFile: "opencode.yaml",
		Binary:      "opencode",
		VersionArgs: []string{"--version"},
		LoginArgs:   []string{"auth", "login"},
		// OpenCode v2 stores credentials in its SQLite database; auth.json is
		// the v1 store: https://opencode.ai/docs/cli/providers/#storage
		OtherAuthStore: "OpenCode's database",
		CredentialPaths: []PathTemplate{
			{
				XDGEnvVar:     "XDG_DATA_HOME",
				DefaultXDGDir: ".local/share",
				XDGSubPath:    "opencode/auth.json",
			},
		},
		APIKeyEnvVars: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY"},
		MCPConfigs: []MCPConfigSpec{
			{
				Path: PathTemplate{
					XDGEnvVar:     "XDG_CONFIG_HOME",
					DefaultXDGDir: ".config",
					XDGSubPath:    "opencode/opencode.json",
				},
				Format:     MCPFormatJSON,
				ServersKey: "mcp",
			},
			{
				Path: PathTemplate{
					XDGEnvVar:     "XDG_CONFIG_HOME",
					DefaultXDGDir: ".config",
					XDGSubPath:    "opencode/opencode.jsonc",
				},
				Format:     MCPFormatJSONC,
				ServersKey: "mcp",
			},
		},
		NPMPackage: "opencode-ai",
	},
	{
		// Binary: apps/cli/config/applications/ai-agents/goose.yaml (aqua:aaif-goose/goose).
		// Interactive provider setup (`goose configure`), OS keyring with ~/.config/goose/secrets.yaml fallback,
		// and MCP extensions in ~/.config/goose/config.yaml (key "extensions"):
		//   https://block.github.io/goose/docs/getting-started/providers
		//   https://block.github.io/goose/docs/guides/config-files
		ID:          "goose",
		Name:        "Goose",
		CatalogFile: "goose.yaml",
		Binary:      "goose",
		VersionArgs: []string{"--version"},
		LoginArgs:   []string{"configure"},
		CredentialPaths: []PathTemplate{
			{
				XDGEnvVar:     "XDG_CONFIG_HOME",
				DefaultXDGDir: ".config",
				XDGSubPath:    "goose/secrets.yaml",
			},
		},
		UsesKeyring:   true,
		APIKeyEnvVars: []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY", "OPENROUTER_API_KEY", "DATABRICKS_TOKEN"},
		MCPConfigs: []MCPConfigSpec{
			{
				Path: PathTemplate{
					XDGEnvVar:     "XDG_CONFIG_HOME",
					DefaultXDGDir: ".config",
					XDGSubPath:    "goose/config.yaml",
				},
				Format:     MCPFormatYAML,
				ServersKey: "extensions",
			},
		},
	},
	{
		// Binary: apps/cli/config/applications/ai-agents/aider.yaml (pipx:aider-chat).
		// Env-var / config-based API keys (no interactive OAuth login command, no native MCP host config):
		//   https://aider.chat/docs/config/dotenv.html
		//   https://aider.chat/docs/config/aider_conf.html
		ID:              "aider",
		Name:            "Aider",
		CatalogFile:     "aider.yaml",
		Binary:          "aider",
		VersionArgs:     []string{"--version"},
		NoLoginGuidance: `Aider does not have an interactive login command. Export your provider API key (for example ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, OPENROUTER_API_KEY, or DEEPSEEK_API_KEY) from your secret manager, or configure ~/.aider.conf.yml / .env outside DevEx config.`,
		APIKeyEnvVars:   []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY", "DEEPSEEK_API_KEY"},
	},
	{
		// Binary and npm alternative (@moonshot-ai/kimi-code): apps/cli/config/applications/ai-agents/kimi-code.yaml.
		// Device-code login (`kimi login`), $KIMI_CODE_HOME/mcp.json (default ~/.kimi-code/mcp.json, key "mcpServers"),
		// env vars (KIMI_API_KEY, MOONSHOT_API_KEY, KIMI_MODEL_API_KEY), and Node >= 22.19.0 for npm installs:
		//   https://moonshotai.github.io/kimi-code/
		//   https://code.kimi.com/
		// Note: exact internal OAuth credential filename inside $KIMI_CODE_HOME is not confirmed in vendor docs,
		// so CredentialPaths is omitted and doctor reports "unknown" when no API key env var is set.
		ID:            "kimi",
		Name:          "Kimi Code",
		CatalogFile:   "kimi-code.yaml",
		Binary:        "kimi",
		VersionArgs:   []string{"--version"},
		LoginArgs:     []string{"login"},
		APIKeyEnvVars: []string{"KIMI_API_KEY", "MOONSHOT_API_KEY", "KIMI_MODEL_API_KEY"},
		// Kimi does not read these automatically:
		// https://moonshotai.github.io/kimi-code/en/configuration/env-vars.html
		EnvVarsDoNotAuthenticate: true,
		MCPConfigs: []MCPConfigSpec{
			{
				Path: PathTemplate{
					EnvDirVar:   "KIMI_CODE_HOME",
					EnvRelPath:  "mcp.json",
					HomeRelPath: ".kimi-code/mcp.json",
				},
				Format:     MCPFormatJSON,
				ServersKey: "mcpServers",
			},
		},
		NPMPackage:     "@moonshot-ai/kimi-code",
		MinNodeVersion: "22.19.0",
	},
}

var (
	catalogPrereqsOnce sync.Once
	catalogPrereqs     map[string][]string
)

func loadCatalogSandboxPrereqs() map[string][]string {
	catalogPrereqsOnce.Do(func() {
		catalogPrereqs = map[string][]string{}
		_ = fs.WalkDir(shipconfig.Files, "applications/ai-agents", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
				return nil
			}
			data, err := fs.ReadFile(shipconfig.Files, path)
			if err != nil {
				return nil
			}
			var app types.CrossPlatformApp
			if err := yaml.Unmarshal(data, &app); err != nil {
				return nil
			}
			if len(app.Linux.SandboxPrerequisites) > 0 {
				prereqs := make([]string, len(app.Linux.SandboxPrerequisites))
				copy(prereqs, app.Linux.SandboxPrerequisites)
				catalogPrereqs[filepath.Base(path)] = prereqs
			}
			return nil
		})
	})
	return catalogPrereqs
}

// KnownAgents returns a copy of the static agent table enriched with
// sandbox_prerequisites from the embedded ai-agents catalog.
func KnownAgents() []AgentSpec {
	prereqsByFile := loadCatalogSandboxPrereqs()
	out := make([]AgentSpec, len(knownAgentDefs))
	for i, def := range knownAgentDefs {
		spec := def
		if prereqs, ok := prereqsByFile[def.CatalogFile]; ok {
			spec.SandboxPrerequisites = append([]string(nil), prereqs...)
		}
		out[i] = spec
	}
	return out
}

// ValidAgentIDs returns the sorted list of recognized agent IDs.
func ValidAgentIDs() []string {
	ids := make([]string, len(knownAgentDefs))
	for i, def := range knownAgentDefs {
		ids[i] = def.ID
	}
	sort.Strings(ids)
	return ids
}

// LookupAgent returns the AgentSpec for id (case-insensitive).
func LookupAgent(id string) (AgentSpec, bool) {
	needle := strings.ToLower(strings.TrimSpace(id))
	for _, spec := range KnownAgents() {
		if spec.ID == needle {
			return spec, true
		}
	}
	return AgentSpec{}, false
}

// FilterAgents validates ids and returns the matching AgentSpecs in the order
// requested, deduplicated. When ids is empty, all KnownAgents are returned.
func FilterAgents(ids []string) ([]AgentSpec, error) {
	if len(ids) == 0 {
		return KnownAgents(), nil
	}
	seen := make(map[string]bool, len(ids))
	result := make([]AgentSpec, 0, len(ids))
	for _, raw := range ids {
		id := strings.ToLower(strings.TrimSpace(raw))
		spec, ok := LookupAgent(id)
		if !ok {
			// Never echo the rejected value: it may be a pasted credential.
			return nil, fmt.Errorf("unknown agent: valid agents are %s", strings.Join(ValidAgentIDs(), ", "))
		}
		if seen[spec.ID] {
			continue
		}
		seen[spec.ID] = true
		result = append(result, spec)
	}
	return result, nil
}
