package config

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agents configuration schema and validation", func() {
	var (
		home     string
		settings CrossPlatformSettings
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		settings = CrossPlatformSettings{HomeDir: home}
	})

	writeAgentsYAML := func(content string) string {
		path := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
		Expect(os.MkdirAll(filepath.Dir(path), 0o750)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		return path
	}

	It("loads a valid agents.yaml without warnings and passes strict validation", func() {
		writeAgentsYAML(`agents:
  enabled: [claude, codex, gemini, opencode]
  preset: balanced
  instructions: |
    Prefer small diffs.
  allowed_commands:
    - "git status"
    - "go test ./..."
  denied_commands:
    - "git push --force"
  overrides:
    claude:
      model: claude-opus-5-5
      preset: strict
      allowed_commands:
        - "gh pr view"
      denied_commands:
        - "rm -rf"
`)
		loaded, err := LoadCrossPlatformSettings(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Agents.Enabled).To(Equal([]string{"claude", "codex", "gemini", "opencode"}))
		Expect(loaded.Agents.Preset).To(Equal("balanced"))
		Expect(loaded.Agents.Instructions).To(ContainSubstring("Prefer small diffs."))
		Expect(loaded.Agents.AllowedCommands).To(Equal([]string{"git status", "go test ./..."}))
		Expect(loaded.Agents.DeniedCommands).To(Equal([]string{"git push --force"}))
		Expect(loaded.Agents.Overrides["claude"].Model).To(Equal("claude-opus-5-5"))
		Expect(loaded.Agents.Overrides["claude"].Preset).To(Equal("strict"))
		Expect(loaded.Agents.Overrides["claude"].AllowedCommands).To(Equal([]string{"gh pr view"}))
		Expect(loaded.Agents.Overrides["claude"].DeniedCommands).To(Equal([]string{"rm -rf"}))

		result, err := ValidateConfig(context.Background(), loaded, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "%v", result.Diagnostics)
		Expect(result.Warnings).To(BeZero())
	})

	It("treats agents.yaml as a non-legacy config file", func() {
		Expect(isLegacyConfigFile("agents.yaml")).To(BeFalse())
	})

	It("rejects unknown keys in agents.yaml under strict schema validation", func() {
		path := writeAgentsYAML(`agents:
  enabled: [claude]
  bogus_field: true
  overrides:
    claude:
      model: sonnet
      unknown_override: 123
`)
		result, err := ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "agents.bogus_field"),
			HaveField("Severity", SeverityError),
		)))
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "agents.overrides.claude.unknown_override"),
			HaveField("Severity", SeverityError),
		)))
	})

	It("rejects unsupported agent names and duplicate enabled agents", func() {
		path := writeAgentsYAML(`agents:
  enabled: [claude, claude, unknown-harness]
  overrides:
    bogus:
      model: foo
`)
		result, err := ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "agents.enabled[1]"),
			HaveField("Message", ContainSubstring("duplicate agent")),
		)))
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "agents.enabled[2]"),
			HaveField("Message", ContainSubstring("unsupported agent")),
		)))
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "agents.overrides.bogus"),
			HaveField("Message", ContainSubstring("unsupported agent")),
		)))
	})

	It("rejects unknown presets", func() {
		path := writeAgentsYAML(`agents:
  enabled: [claude]
  preset: permissive
`)
		result, err := ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "agents.preset"),
			HaveField("Message", ContainSubstring("unsupported preset")),
		)))
	})

	DescribeTable("rejects forbidden skip-permission modes at top-level and in per-agent overrides",
		func(forbidden string) {
			errs := ValidateAgentsConfig(AgentsConfig{
				Enabled: []string{AgentClaude},
				Preset:  forbidden,
			}, "agents.yaml")
			Expect(errs).NotTo(BeEmpty())
			Expect(errs[0].Message).To(ContainSubstring(ErrAgentsSkipPermissions))

			overrideErrs := ValidateAgentsConfig(AgentsConfig{
				Enabled: []string{AgentCodex},
				Overrides: map[string]AgentOverrideConfig{
					AgentCodex: {Preset: forbidden},
				},
			}, "agents.yaml")
			Expect(overrideErrs).NotTo(BeEmpty())
			Expect(overrideErrs[0].Message).To(ContainSubstring(ErrAgentsSkipPermissions))
		},
		Entry("bypassPermissions", "bypassPermissions"),
		Entry("danger-full-access", "danger-full-access"),
		Entry("never", "never"),
		Entry("yolo", "yolo"),
		Entry("allow", "allow"),
	)

	It("rejects empty or multiline command strings and invalid model values", func() {
		errs := ValidateAgentsConfig(AgentsConfig{
			Enabled:         []string{AgentClaude},
			AllowedCommands: []string{"   ", "git status\nrm -rf /"},
			DeniedCommands:  []string{"bad\rcmd"},
			Overrides: map[string]AgentOverrideConfig{
				AgentClaude: {Model: "bad\nmodel"},
			},
		}, "agents.yaml")
		Expect(errs).To(HaveLen(4))
	})

	It("rejects fields when an agent has no native equivalent", func() {
		caps := map[string]AgentCapabilities{
			AgentClaude: {Instructions: true, Model: false, Preset: true, AllowedCommands: true, DeniedCommands: false},
		}
		errs := validateAgentsConfigWithCaps(AgentsConfig{
			Enabled:        []string{AgentClaude},
			DeniedCommands: []string{"rm -rf /"},
			Overrides: map[string]AgentOverrideConfig{
				AgentClaude: {Model: "sonnet"},
			},
		}, "agents.yaml", caps)
		Expect(errs).To(ContainElement(And(
			HaveField("Field", "agents.denied_commands"),
			HaveField("Message", ContainSubstring("has no native equivalent for denied_commands")),
		)))
		Expect(errs).To(ContainElement(And(
			HaveField("Field", "agents.overrides.claude.model"),
			HaveField("Message", ContainSubstring("has no native equivalent for model")),
		)))
	})

	It("loads agents.yaml with mcp_servers and devex_mcp preserving env/header key case and passing strict validation", func() {
		writeAgentsYAML(`agents:
  enabled: [claude, codex, gemini, opencode]
  devex_mcp: true
  mcp_servers:
    - name: github
      transport: http
      url: https://api.githubcopilot.com/mcp/
      headers:
        Authorization: "Bearer ${env:GITHUB_TOKEN}"
        X-Org-Name: acme
      agents: [claude, codex]
    - name: filesystem
      transport: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/me/src"]
      env:
        GITHUB_TOKEN: "${env:GITHUB_TOKEN}"
        LOG_LEVEL: info
`)
		loaded, err := LoadCrossPlatformSettings(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Agents.DevExMCP).To(BeTrue())
		Expect(loaded.Agents.MCPServers).To(HaveLen(2))
		Expect(loaded.Agents.MCPServers[0].Headers).To(HaveKeyWithValue("Authorization", "Bearer ${env:GITHUB_TOKEN}"))
		Expect(loaded.Agents.MCPServers[0].Headers).To(HaveKeyWithValue("X-Org-Name", "acme"))
		Expect(loaded.Agents.MCPServers[1].Env).To(HaveKeyWithValue("GITHUB_TOKEN", "${env:GITHUB_TOKEN}"))
		Expect(loaded.Agents.MCPServers[1].Env).To(HaveKeyWithValue("LOG_LEVEL", "info"))

		result, err := ValidateConfig(context.Background(), loaded, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "%v", result.Diagnostics)
		Expect(result.Warnings).To(BeZero())
	})

	It("rejects unknown keys inside mcp_servers under strict schema validation", func() {
		path := writeAgentsYAML(`agents:
  enabled: [claude]
  mcp_servers:
    - name: github
      transport: http
      url: https://api.githubcopilot.com/mcp/
      bogus_mcp_key: true
`)
		result, err := ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "agents.mcp_servers[0].bogus_mcp_key"),
			HaveField("Severity", SeverityError),
		)))
	})

	It("validates MCP server names, transports, transport-specific fields, agents subset, and devex_mcp reservation", func() {
		errs := ValidateAgentsConfig(AgentsConfig{
			Enabled:  []string{AgentClaude, AgentCodex},
			DevExMCP: true,
			MCPServers: []AgentMCPServerConfig{
				{
					Name:      "devex",
					Transport: MCPTransportStdio,
					Command:   "devex",
				},
				{
					Name:      "bad name!",
					Transport: "sse",
				},
				{
					Name:      "dup",
					Transport: MCPTransportStdio,
					URL:       "https://example.com",
					Headers:   map[string]string{"X-Test": "1"},
					Agents:    []string{AgentGemini, "bogus"},
				},
				{
					Name:      "dup",
					Transport: MCPTransportHTTP,
					URL:       "ftp://example.com",
					Command:   "npx",
					Args:      []string{"foo"},
					Env:       map[string]string{"A": "1"},
				},
			},
		}, "agents.yaml")

		Expect(errs).To(ContainElement(HaveField("Message", ContainSubstring(`name "devex" is reserved`))))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[1].name")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[1].transport")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[2].command")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[2].url")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[2].headers")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[2].agents[0]")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[2].agents[1]")))
		Expect(errs).To(ContainElement(HaveField("Message", ContainSubstring(`duplicate MCP server name "dup"`))))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[3].command")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[3].args")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[3].env")))
		Expect(errs).To(ContainElement(HaveField("Field", "agents.mcp_servers[3].url")))
	})

	It("rejects literal secrets in MCP env, headers, args, and URL without echoing the secret", func() {
		secret1 := "ghp_1234567890abcdef"
		secret2 := "raw-password-value"
		secret3 := "sk-ant-literal99999"
		secret4 := "raw-url-secret-value"
		errs := ValidateAgentsConfig(AgentsConfig{
			Enabled: []string{AgentClaude},
			MCPServers: []AgentMCPServerConfig{
				{
					Name:      "local",
					Transport: MCPTransportStdio,
					Command:   "npx",
					Args:      []string{"--tok=" + secret1},
					Env: map[string]string{
						"DB_PASSWORD": secret2,
						"PLAIN":       secret3,
					},
				},
				{
					Name:      "remote",
					Transport: MCPTransportHTTP,
					URL:       "https://api.example.com/mcp?token=" + secret4,
					Headers: map[string]string{
						"Authorization": "Bearer " + secret2,
					},
				},
			},
		}, "agents.yaml")

		Expect(errs).To(HaveLen(5))
		for _, e := range errs {
			Expect(e.Message).NotTo(ContainSubstring(secret1))
			Expect(e.Message).NotTo(ContainSubstring(secret2))
			Expect(e.Message).NotTo(ContainSubstring(secret3))
			Expect(e.Message).NotTo(ContainSubstring(secret4))
		}
	})

	It("enforces secret policy for Cookie, Proxy-Authorization, secret CLI flags in args, and allows AUTH_MODE", func() {
		errs := ValidateAgentsConfig(AgentsConfig{
			Enabled: []string{AgentClaude},
			MCPServers: []AgentMCPServerConfig{
				{
					Name:      "local",
					Transport: MCPTransportStdio,
					Command:   "npx",
					Args:      []string{"--token", "plain-tok-val", "--api-key=plain-key-val", "--auth-token", "${env:MY_TOK}"},
					Env: map[string]string{
						"AUTH_MODE": "oauth",
					},
				},
				{
					Name:      "remote",
					Transport: MCPTransportHTTP,
					URL:       "https://api.example.com/mcp",
					Headers: map[string]string{
						"Cookie":              "session=secret-cookie",
						"Proxy-Authorization": "Basic secret-proxy",
					},
				},
			},
		}, "agents.yaml")

		Expect(errs).To(HaveLen(5))
		for _, e := range errs {
			Expect(e.Message).NotTo(ContainSubstring("plain-tok-val"))
			Expect(e.Message).NotTo(ContainSubstring("plain-key-val"))
			Expect(e.Message).NotTo(ContainSubstring("secret-cookie"))
			Expect(e.Message).NotTo(ContainSubstring("secret-proxy"))
		}
	})

	It("rejects MCP server names starting with a hyphen or underscore", func() {
		errs := ValidateAgentsConfig(AgentsConfig{
			Enabled: []string{AgentClaude},
			MCPServers: []AgentMCPServerConfig{
				{Name: "-bad", Transport: MCPTransportStdio, Command: "mcp"},
				{Name: "--scope", Transport: MCPTransportStdio, Command: "mcp"},
				{Name: "_bad", Transport: MCPTransportStdio, Command: "mcp"},
			},
		}, "agents.yaml")
		Expect(errs).To(HaveLen(3))
	})

	It("does not override agents config when agents.yaml has no agents key", func() {
		teamDir := settings.GetTeamConfigDir()
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(teamDir, "agents.yaml"), []byte(`agents:
  enabled: [claude]
  preset: strict
`), 0o600)).To(Succeed())

		// User tier has an agents.yaml with no top-level agents key.
		writeAgentsYAML("# empty user agents.yaml\n")

		loaded, err := LoadCrossPlatformSettings(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Agents.Enabled).To(Equal([]string{"claude"}))
		Expect(loaded.Agents.Preset).To(Equal("strict"))
	})

	It("allows a higher-precedence tier to override agents from a non-agents.yaml file", func() {
		teamDir := settings.GetTeamConfigDir()
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(teamDir, "agents.yaml"), []byte(`agents:
  enabled: [claude]
  preset: strict
`), 0o600)).To(Succeed())

		// User tier overrides agents via another config file (dotfiles.yaml) without its own agents.yaml.
		userDir := settings.GetUserConfigDir()
		Expect(os.MkdirAll(userDir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(userDir, "dotfiles.yaml"), []byte(`agents:
  enabled: [codex]
  preset: balanced
`), 0o600)).To(Succeed())

		loaded, err := LoadCrossPlatformSettings(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Agents.Enabled).To(Equal([]string{"codex"}))
		Expect(loaded.Agents.Preset).To(Equal("balanced"))
	})
})
