package agentinit_test

import (
	"encoding/json"

	"github.com/jameswlane/devex/apps/cli/internal/agentinit"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent init rendering", func() {
	It("renders AGENTS.md with detected stack, commands, and editable placeholders", func() {
		out, err := agentinit.RenderAgentsMD(agentinit.ProjectInfo{
			ProjectName:  "demo-service",
			Languages:    []string{"Go"},
			TaskRunner:   "task",
			TestCommands: []string{"task test"},
			LintCommands: []string{"task lint"},
		})
		Expect(err).NotTo(HaveOccurred())
		text := string(out)
		Expect(text).To(ContainSubstring("devex agent init"))
		Expect(text).To(ContainSubstring("# demo-service"))
		Expect(text).To(ContainSubstring("Go"))
		Expect(text).To(ContainSubstring("`task`"))
		Expect(text).To(ContainSubstring("# Test\ntask test\n\n# Lint\ntask lint"))
		Expect(text).To(ContainSubstring("## Conventions"))
		Expect(text).To(ContainSubstring("## Do Not Touch"))
	})

	It("omits Stack and Commands sections when nothing was detected", func() {
		out, err := agentinit.RenderAgentsMD(agentinit.ProjectInfo{ProjectName: "blank"})
		Expect(err).NotTo(HaveOccurred())
		text := string(out)
		Expect(text).To(ContainSubstring("# blank"))
		Expect(text).NotTo(ContainSubstring("## Stack"))
		Expect(text).NotTo(ContainSubstring("## Commands"))
	})

	It("renders thin CLAUDE.md and GEMINI.md shims without duplicating instructions", func() {
		claude, err := agentinit.RenderClaudeMD()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(claude)).To(And(
			ContainSubstring("@AGENTS.md"),
			ContainSubstring("AGENTS.md"),
		))
		Expect(string(claude)).NotTo(ContainSubstring("## Conventions"))

		gemini, err := agentinit.RenderGeminiMD()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(gemini)).To(And(
			ContainSubstring("@./AGENTS.md"),
			ContainSubstring("AGENTS.md"),
		))
		Expect(string(gemini)).NotTo(ContainSubstring("## Conventions"))
	})

	Describe("gitignore planning", func() {
		It("creates a new .gitignore with both agent-local entries", func() {
			out, changed := agentinit.PlanGitignoreAppend(nil, false)
			Expect(changed).To(BeTrue())
			Expect(string(out)).To(Equal("# DevEx agent-local files\n.claude/settings.local.json\nCLAUDE.local.md\n"))
		})

		It("preserves existing bytes and adds a newline when missing", func() {
			existing := []byte("dist/\n*.log")
			out, changed := agentinit.PlanGitignoreAppend(existing, true)
			Expect(changed).To(BeTrue())
			Expect(string(out)).To(Equal("dist/\n*.log\n# DevEx agent-local files\n.claude/settings.local.json\nCLAUDE.local.md\n"))
		})

		It("appends only missing entries and becomes a no-op when all exist", func() {
			existing := []byte("dist/\nCLAUDE.local.md\n")
			out, changed := agentinit.PlanGitignoreAppend(existing, true)
			Expect(changed).To(BeTrue())
			Expect(string(out)).To(Equal("dist/\nCLAUDE.local.md\n# DevEx agent-local files\n.claude/settings.local.json\n"))

			again, changedAgain := agentinit.PlanGitignoreAppend(out, true)
			Expect(changedAgain).To(BeFalse())
			Expect(again).To(Equal(out))
		})

		It("treats leading-space patterns as different, since git keeps leading spaces", func() {
			existing := []byte("  .claude/settings.local.json\nCLAUDE.local.md  \r\n")
			out, changed := agentinit.PlanGitignoreAppend(existing, true)
			Expect(changed).To(BeTrue())
			Expect(string(out)).To(Equal(string(existing) + "# DevEx agent-local files\n.claude/settings.local.json\n"))
		})
	})

	Describe("Claude .mcp.json rendering", func() {
		It("renders stdio, http, and devex servers while excluding non-claude servers", func() {
			cfg := config.AgentsConfig{
				Enabled:  []string{"claude", "codex"},
				DevExMCP: true,
				MCPServers: []config.AgentMCPServerConfig{
					{
						Name:      "github",
						Transport: config.MCPTransportStdio,
						Command:   "npx",
						Args:      []string{"-y", "@modelcontextprotocol/server-github"},
						Env: map[string]string{
							"GITHUB_TOKEN": "${env:GITHUB_TOKEN}",
							"LOG_LEVEL":    "info",
						},
					},
					{
						Name:      "api",
						Transport: config.MCPTransportHTTP,
						URL:       "https://mcp.example.com/v1",
						Headers: map[string]string{
							"Authorization": "Bearer ${env:API_TOKEN}",
							"X-Client":      "devex",
						},
						Agents: []string{"claude"},
					},
					{
						Name:      "codex-only",
						Transport: config.MCPTransportStdio,
						Command:   "codex-tool",
						Agents:    []string{"codex"},
					},
				},
			}

			out, ok, err := agentinit.RenderClaudeMCPJSON(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(out).To(HaveSuffix("\n"))

			var parsed map[string]any
			Expect(json.Unmarshal(out, &parsed)).To(Succeed())
			servers, isMap := parsed["mcpServers"].(map[string]any)
			Expect(isMap).To(BeTrue())
			Expect(servers).To(HaveLen(3))
			Expect(servers).To(HaveKey("api"))
			Expect(servers).To(HaveKey("devex"))
			Expect(servers).To(HaveKey("github"))
			Expect(servers).NotTo(HaveKey("codex-only"))

			github := servers["github"].(map[string]any)
			Expect(github["type"]).To(Equal("stdio"))
			Expect(github["command"]).To(Equal("npx"))
			Expect(github["env"]).To(Equal(map[string]any{
				"GITHUB_TOKEN": "${GITHUB_TOKEN}",
				"LOG_LEVEL":    "info",
			}))

			api := servers["api"].(map[string]any)
			Expect(api["type"]).To(Equal("http"))
			Expect(api["url"]).To(Equal("https://mcp.example.com/v1"))
			Expect(api["headers"]).To(Equal(map[string]any{
				"Authorization": "Bearer ${API_TOKEN}",
				"X-Client":      "devex",
			}))

			devexSrv := servers["devex"].(map[string]any)
			Expect(devexSrv).To(Equal(map[string]any{
				"type":    "stdio",
				"command": "devex",
				"args":    []any{"mcp", "serve"},
			}))
		})

		It("returns ok=false when no Claude-applicable MCP servers are configured", func() {
			cfg := config.AgentsConfig{
				Enabled: []string{"codex"},
				MCPServers: []config.AgentMCPServerConfig{
					{
						Name:      "codex-only",
						Transport: config.MCPTransportStdio,
						Command:   "codex-tool",
						Agents:    []string{"codex"},
					},
				},
			}
			out, ok, err := agentinit.RenderClaudeMCPJSON(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(out).To(BeNil())
		})

		It("writes no .mcp.json when Claude is not enabled, even for untargeted servers", func() {
			cfg := config.AgentsConfig{
				Enabled:  []string{"codex"},
				DevExMCP: true,
				MCPServers: []config.AgentMCPServerConfig{
					{Name: "shared", Transport: config.MCPTransportStdio, Command: "shared-tool"},
				},
			}
			out, ok, err := agentinit.RenderClaudeMCPJSON(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
			Expect(out).To(BeNil())
		})

		It("rejects a literal token in an http server URL path", func() {
			cfg := config.AgentsConfig{
				Enabled: []string{"claude"},
				MCPServers: []config.AgentMCPServerConfig{
					{Name: "hook", Transport: config.MCPTransportHTTP, URL: "https://mcp.example.com/ghp_abcdefghijklmnop1234/mcp"},
				},
			}
			_, _, err := agentinit.RenderClaudeMCPJSON(cfg)
			Expect(err).To(MatchError(ContainSubstring("url path appears to contain a literal secret")))
			Expect(err.Error()).NotTo(ContainSubstring("ghp_"))
		})

		It("rejects invalid agents config with a literal secret", func() {
			cfg := config.AgentsConfig{
				Enabled: []string{"claude"},
				MCPServers: []config.AgentMCPServerConfig{
					{
						Name:      "bad",
						Transport: config.MCPTransportStdio,
						Command:   "npx",
						Env: map[string]string{
							"GITHUB_TOKEN": "ghp_1234567890abcdef",
						},
					},
				},
			}
			_, _, err := agentinit.RenderClaudeMCPJSON(cfg)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("env"))
		})
	})
})
