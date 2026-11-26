package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type fakeAgentPluginExecutor struct {
	called        bool
	pluginName    string
	args          []string
	allowDownload bool
	specPath      string
	specBytes     []byte
	specMode      os.FileMode
	output        string
	err           error
}

func (f *fakeAgentPluginExecutor) ExecuteAgentPlugin(_ context.Context, pluginName string, args []string, streams sdk.OperationIO, allowDownload bool) error {
	f.called = true
	f.pluginName = pluginName
	f.args = append([]string(nil), args...)
	f.allowDownload = allowDownload
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--config" {
			f.specPath = args[i+1]
			info, statErr := os.Stat(f.specPath)
			if statErr == nil {
				f.specMode = info.Mode().Perm()
			}
			f.specBytes, _ = os.ReadFile(f.specPath)
		}
	}
	if f.output != "" && streams.Stdout != nil {
		_, _ = streams.Stdout.Write([]byte(f.output))
	}
	return f.err
}

var _ = Describe("Agent apply and diff commands", func() {
	var (
		home     string
		settings config.CrossPlatformSettings
		fake     *fakeAgentPluginExecutor
	)

	BeforeEach(func() {
		viper.Reset()
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_AGENTS", "")
		GinkgoT().Setenv("DEVEX_AGENTS_PRESET", "")
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		GinkgoT().Setenv("DEVEX_DISABLE_UPDATE_CHECK", "true")
		GinkgoT().Setenv("DEVEX_OFFLINE", "true")

		fake = &fakeAgentPluginExecutor{output: "created: /tmp/example\n"}
		settings = config.CrossPlatformSettings{
			HomeDir: home,
			Agents: config.AgentsConfig{
				Enabled:         []string{"claude", "codex"},
				Preset:          "balanced",
				Instructions:    "Prefer small diffs.",
				AllowedCommands: []string{"git status", "go test ./..."},
				DeniedCommands:  []string{"git push --force"},
				Overrides: map[string]config.AgentOverrideConfig{
					"claude": {
						Model:           "claude-opus-5-5",
						Preset:          "strict",
						AllowedCommands: []string{"gh pr view"},
						DeniedCommands:  []string{"rm -rf /"},
					},
					"codex": {
						Model: "gpt-5",
					},
				},
			},
		}
	})

	AfterEach(func() {
		viper.Reset()
	})

	It("prints an actionable message and exits 0 when no agents are configured", func() {
		emptySettings := config.CrossPlatformSettings{HomeDir: home}
		cmd := NewAgentCmdWithDeps(emptySettings, nil, agentsandbox.Probe{}, fake)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"apply"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(fake.called).To(BeFalse())
		Expect(out.String()).To(And(
			ContainSubstring("No coding agents configured"),
			ContainSubstring(filepath.Join(emptySettings.GetUserConfigDir(), "agents.yaml")),
			ContainSubstring(agentsDocsURL),
		))
	})

	It("writes a private resolved spec JSON, runs tool-agents apply, and removes the temp file", func() {
		cmd := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"apply"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(fake.called).To(BeTrue())
		Expect(fake.pluginName).To(Equal("tool-agents"))
		Expect(fake.allowDownload).To(BeTrue())
		Expect(fake.args).To(HaveLen(3))
		Expect(fake.args[0]).To(Equal("apply"))
		Expect(fake.args[1]).To(Equal("--config"))
		Expect(fake.specMode).To(Equal(os.FileMode(0o600)))

		var spec resolvedAgentPluginSpec
		Expect(json.Unmarshal(fake.specBytes, &spec)).To(Succeed())
		Expect(spec.Agents).To(HaveLen(2))
		Expect(spec.Agents[0]).To(Equal(resolvedAgentSpec{
			Name:            "claude",
			Preset:          "strict",
			Instructions:    "Prefer small diffs.",
			Model:           "claude-opus-5-5",
			AllowedCommands: []string{"git status", "go test ./...", "gh pr view"},
			DeniedCommands:  []string{"git push --force", "rm -rf /"},
		}))
		Expect(spec.Agents[1]).To(Equal(resolvedAgentSpec{
			Name:            "codex",
			Preset:          "balanced",
			Instructions:    "Prefer small diffs.",
			Model:           "gpt-5",
			AllowedCommands: []string{"git status", "go test ./..."},
			DeniedCommands:  []string{"git push --force"},
		}))

		// Temporary spec file must be deleted after execution.
		_, statErr := os.Stat(fake.specPath)
		Expect(os.IsNotExist(statErr)).To(BeTrue())
		Expect(out.String()).To(ContainSubstring("created: /tmp/example"))
	})

	It("routes devex agent diff to the plugin diff command with allowDownload=false", func() {
		cmd := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"diff"})

		Expect(cmd.Execute()).To(Succeed())
		Expect(fake.called).To(BeTrue())
		Expect(fake.allowDownload).To(BeFalse())
		Expect(fake.args[0]).To(Equal("diff"))
		_, statErr := os.Stat(fake.specPath)
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	It("routes apply --dry-run through root persistent flag to plugin diff with zero downloads", func() {
		root := NewRootCmd("test", mocks.NewMockRepository(), settings)
		// Replace the agent command with one wired to our fake executor.
		for _, sub := range root.Commands() {
			if sub.Name() == "agent" {
				root.RemoveCommand(sub)
				break
			}
		}
		root.AddCommand(NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake))
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"agent", "apply", "--dry-run"})

		Expect(root.Execute()).To(Succeed())
		Expect(fake.called).To(BeTrue())
		Expect(fake.args[0]).To(Equal("diff"))
		Expect(fake.allowDownload).To(BeFalse())
		_, statErr := os.Stat(fake.specPath)
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	It("enforces precedence: flags > env > config > defaults for --agents and --preset", func() {
		// 1. Env overrides config.
		GinkgoT().Setenv("DEVEX_AGENTS", "codex")
		GinkgoT().Setenv("DEVEX_AGENTS_PRESET", "strict")

		cmdEnv := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		var out1 bytes.Buffer
		cmdEnv.SetOut(&out1)
		cmdEnv.SetErr(&out1)
		cmdEnv.SetArgs([]string{"diff"})
		Expect(cmdEnv.Execute()).To(Succeed())

		var specEnv resolvedAgentPluginSpec
		Expect(json.Unmarshal(fake.specBytes, &specEnv)).To(Succeed())
		Expect(specEnv.Agents).To(HaveLen(1))
		Expect(specEnv.Agents[0].Name).To(Equal("codex"))
		Expect(specEnv.Agents[0].Preset).To(Equal("strict"))

		// 2. CLI flags override env AND per-agent config preset.
		fake2 := &fakeAgentPluginExecutor{}
		cmdFlag := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake2)
		var out2 bytes.Buffer
		cmdFlag.SetOut(&out2)
		cmdFlag.SetErr(&out2)
		cmdFlag.SetArgs([]string{"diff", "--agents", "claude", "--preset", "balanced"})
		Expect(cmdFlag.Execute()).To(Succeed())

		var specFlag resolvedAgentPluginSpec
		Expect(json.Unmarshal(fake2.specBytes, &specFlag)).To(Succeed())
		Expect(specFlag.Agents).To(HaveLen(1))
		Expect(specFlag.Agents[0].Name).To(Equal("claude"))
		Expect(specFlag.Agents[0].Preset).To(Equal("balanced"))
	})

	It("honours DEVEX_AGENTS and DEVEX_AGENTS_PRESET through the root command's flag binding", func() {
		GinkgoT().Setenv("DEVEX_AGENTS", "codex")
		GinkgoT().Setenv("DEVEX_AGENTS_PRESET", "strict")
		root := NewRootCmd("test", mocks.NewMockRepository(), settings)
		for _, sub := range root.Commands() {
			if sub.Name() == "agent" {
				root.RemoveCommand(sub)
				break
			}
		}
		root.AddCommand(NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake))
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"agent", "diff"})

		Expect(root.Execute()).To(Succeed())
		var spec resolvedAgentPluginSpec
		Expect(json.Unmarshal(fake.specBytes, &spec)).To(Succeed())
		Expect(spec.Agents).To(HaveLen(1))
		Expect(spec.Agents[0].Name).To(Equal("codex"))
		Expect(spec.Agents[0].Preset).To(Equal("strict"))
	})

	It("rejects agents not in enabled list, unknown agents, and forbidden presets without running plugin", func() {
		cmd1 := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		cmd1.SetArgs([]string{"diff", "--agents", "gemini"})
		Expect(cmd1.Execute()).To(MatchError(ContainSubstring(`agent "gemini" is not enabled`)))
		Expect(fake.called).To(BeFalse())

		cmd2 := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		cmd2.SetArgs([]string{"diff", "--agents", "bogus"})
		Expect(cmd2.Execute()).To(MatchError(ContainSubstring(`unsupported agent "bogus"`)))
		Expect(fake.called).To(BeFalse())

		cmd3 := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		cmd3.SetArgs([]string{"apply", "--preset", "yolo"})
		Expect(cmd3.Execute()).To(MatchError(ContainSubstring(config.ErrAgentsSkipPermissions)))
		Expect(fake.called).To(BeFalse())
	})
})
