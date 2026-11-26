package agentinit

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/jameswlane/devex/apps/cli/internal/config"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var (
	templates = template.Must(
		template.New("").Funcs(template.FuncMap{
			"join": strings.Join,
		}).ParseFS(templateFS, "templates/*.tmpl"),
	)

	mcpEnvRefRe = regexp.MustCompile(`^\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}$`)

	gitignoreEntries = []string{
		".claude/settings.local.json",
		"CLAUDE.local.md",
	}
)

const gitignoreComment = "# DevEx agent-local files"

// RenderAgentsMD renders the canonical AGENTS.md file from detected project info.
func RenderAgentsMD(info ProjectInfo) ([]byte, error) {
	return renderTemplate("AGENTS.md.tmpl", info)
}

// RenderClaudeMD renders the thin CLAUDE.md shim importing AGENTS.md.
func RenderClaudeMD() ([]byte, error) {
	return renderTemplate("CLAUDE.md.tmpl", nil)
}

// RenderGeminiMD renders the thin GEMINI.md shim importing AGENTS.md.
func RenderGeminiMD() ([]byte, error) {
	return renderTemplate("GEMINI.md.tmpl", nil)
}

func renderTemplate(name string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil, fmt.Errorf("render template %q: %w", name, err)
	}
	return buf.Bytes(), nil
}

type claudeMCPFile struct {
	MCPServers map[string]any `json:"mcpServers"`
}

type claudeStdioMCPServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

type claudeHTTPMCPServer struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// RenderClaudeMCPJSON renders Claude Code's project-scope .mcp.json content
// from declarative AgentsConfig. Returns ok=false when no Claude-applicable
// servers are configured. The devex server always uses the bare "devex"
// command: .mcp.json is committed and shared, so a machine-specific absolute
// executable path (as `agent apply` writes to user config) would break for
// other contributors.
func RenderClaudeMCPJSON(cfg config.AgentsConfig) ([]byte, bool, error) {
	if errs := config.ValidateAgentsConfig(cfg, "agents.yaml"); len(errs) > 0 {
		return nil, false, fmt.Errorf("invalid agents configuration: %s", errs[0].Error())
	}

	if !slices.Contains(cfg.Enabled, config.AgentClaude) {
		return nil, false, nil
	}

	servers := make(map[string]any)
	for _, srv := range cfg.MCPServers {
		if len(srv.Agents) > 0 && !slices.Contains(srv.Agents, config.AgentClaude) {
			continue
		}
		switch srv.Transport {
		case config.MCPTransportStdio:
			args := slices.Clone(srv.Args)
			if args == nil {
				args = []string{}
			}
			var env map[string]string
			if len(srv.Env) > 0 {
				env = make(map[string]string, len(srv.Env))
				for k, v := range srv.Env {
					env[k] = renderClaudeRefValue(v)
				}
			}
			servers[srv.Name] = claudeStdioMCPServer{
				Type:    "stdio",
				Command: srv.Command,
				Args:    args,
				Env:     env,
			}
		case config.MCPTransportHTTP:
			var headers map[string]string
			if len(srv.Headers) > 0 {
				headers = make(map[string]string, len(srv.Headers))
				for k, v := range srv.Headers {
					headers[k] = renderClaudeRefValue(v)
				}
			}
			servers[srv.Name] = claudeHTTPMCPServer{
				Type:    "http",
				URL:     srv.URL,
				Headers: headers,
			}
		}
	}

	if cfg.DevExMCP {
		servers["devex"] = claudeStdioMCPServer{
			Type:    "stdio",
			Command: "devex",
			Args:    []string{"mcp", "serve"},
		}
	}

	if len(servers) == 0 {
		return nil, false, nil
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(claudeMCPFile{MCPServers: servers}); err != nil {
		return nil, false, fmt.Errorf("encode .mcp.json: %w", err)
	}
	return buf.Bytes(), true, nil
}

func renderClaudeRefValue(val string) string {
	if name, ok := parseEnvRef(val); ok {
		return "${" + name + "}"
	}
	if strings.HasPrefix(val, "Bearer ") {
		if name, ok := parseEnvRef(strings.TrimPrefix(val, "Bearer ")); ok {
			return "Bearer ${" + name + "}"
		}
	}
	return val
}

func parseEnvRef(val string) (string, bool) {
	m := mcpEnvRefRe.FindStringSubmatch(val)
	if len(m) != 2 {
		return "", false
	}
	return m[1], true
}

// PlanGitignoreAppend appends missing DevEx agent-local ignore entries while
// preserving every existing byte.
func PlanGitignoreAppend(existing []byte, exists bool) ([]byte, bool) {
	present := make(map[string]bool, len(gitignoreEntries))
	if exists {
		for _, line := range strings.Split(string(existing), "\n") {
			// Like git: drop one CR before the newline, then unescaped trailing
			// spaces; leading spaces stay significant.
			present[strings.TrimRight(strings.TrimSuffix(line, "\r"), " ")] = true
		}
	}

	var missing []string
	for _, entry := range gitignoreEntries {
		if !present[entry] {
			missing = append(missing, entry)
		}
	}
	if len(missing) == 0 {
		return existing, false
	}

	var buf bytes.Buffer
	if exists && len(existing) > 0 {
		buf.Write(existing)
		if existing[len(existing)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	buf.WriteString(gitignoreComment)
	buf.WriteByte('\n')
	for _, entry := range missing {
		buf.WriteString(entry)
		buf.WriteByte('\n')
	}
	return buf.Bytes(), true
}
