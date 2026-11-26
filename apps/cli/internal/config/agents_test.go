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
})
