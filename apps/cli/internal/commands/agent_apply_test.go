package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/localmodels"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
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
		if runtime.GOOS != "windows" {
			// Windows does not map Unix permission bits onto files.
			Expect(fake.specMode).To(Equal(os.FileMode(0o600)))
		}

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

	It("omits mcp_servers from the plugin spec JSON when none are configured for backwards compatibility", func() {
		cmd := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"diff"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(string(fake.specBytes)).NotTo(ContainSubstring(`"mcp_servers"`))
	})

	It("resolves per-agent mcp_servers subsets and devex_mcp entry into the plugin spec", func() {
		origLookPath := agentLookPath
		origExecutable := agentExecutable
		DeferCleanup(func() {
			agentLookPath = origLookPath
			agentExecutable = origExecutable
		})

		fakeBin := filepath.Join(home, "bin", "devex")
		Expect(os.MkdirAll(filepath.Dir(fakeBin), 0o750)).To(Succeed())
		Expect(os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0o700)).To(Succeed())

		// Case 1: LookPath("devex") matches os.Executable() -> uses "devex".
		agentExecutable = func() (string, error) { return fakeBin, nil }
		agentLookPath = func(string) (string, error) { return fakeBin, nil }

		settings.Agents.DevExMCP = true
		settings.Agents.MCPServers = []config.AgentMCPServerConfig{
			{
				Name:      "github",
				Transport: config.MCPTransportHTTP,
				URL:       "https://api.githubcopilot.com/mcp/",
				Headers:   map[string]string{"Authorization": "Bearer ${env:GITHUB_TOKEN}"},
				Agents:    []string{"claude"},
			},
			{
				Name:      "filesystem",
				Transport: config.MCPTransportStdio,
				Command:   "npx",
				Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			},
		}

		cmd := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"apply"})
		Expect(cmd.Execute()).To(Succeed())

		var spec resolvedAgentPluginSpec
		Expect(json.Unmarshal(fake.specBytes, &spec)).To(Succeed())
		Expect(spec.Agents).To(HaveLen(2))
		// claude gets github, filesystem, and devex
		Expect(spec.Agents[0].MCPServers).To(HaveLen(3))
		Expect(spec.Agents[0].MCPServers[0].Name).To(Equal("github"))
		Expect(spec.Agents[0].MCPServers[1].Name).To(Equal("filesystem"))
		Expect(spec.Agents[0].MCPServers[2]).To(Equal(resolvedMCPServerSpec{
			Name:      "devex",
			Transport: config.MCPTransportStdio,
			Command:   "devex",
			Args:      []string{"mcp", "serve"},
		}))
		// codex gets filesystem and devex (not github)
		Expect(spec.Agents[1].MCPServers).To(HaveLen(2))
		Expect(spec.Agents[1].MCPServers[0].Name).To(Equal("filesystem"))
		Expect(spec.Agents[1].MCPServers[1].Name).To(Equal("devex"))

		// Case 2: LookPath("devex") does NOT match os.Executable() -> uses absolute path.
		agentLookPath = func(string) (string, error) { return "", os.ErrNotExist }
		fake2 := &fakeAgentPluginExecutor{}
		cmd2 := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake2)
		cmd2.SetArgs([]string{"diff"})
		Expect(cmd2.Execute()).To(Succeed())

		var spec2 resolvedAgentPluginSpec
		Expect(json.Unmarshal(fake2.specBytes, &spec2)).To(Succeed())
		Expect(spec2.Agents[0].MCPServers[2].Command).To(Equal(fakeBin))
	})

	It("hints to update tool-agents when an older plugin rejects mcp_servers as an unknown field", func() {
		settings.Agents.DevExMCP = true
		fake.err = errors.New(`decode agents spec: json: unknown field "mcp_servers"`)
		cmd := NewAgentCmdWithDeps(settings, nil, agentsandbox.Probe{}, fake)
		cmd.SetArgs([]string{"apply"})
		err := cmd.Execute()
		Expect(err).To(MatchError(And(
			ContainSubstring("does not support mcp_servers"),
			ContainSubstring("devex plugin update tool-agents"),
		)))
	})

	It("supports --init to write minimal agents.yaml when absent while leaving an existing file untouched", func() {
		emptySettings := config.CrossPlatformSettings{HomeDir: home}
		agentsPath := filepath.Join(emptySettings.GetUserConfigDir(), "agents.yaml")

		// Dry-run writes nothing and does not invoke tool-agents.
		viper.Set("dry-run", true)
		dryCmd := NewAgentCmdWithDeps(emptySettings, nil, agentsandbox.Probe{}, fake)
		var dryOut bytes.Buffer
		dryCmd.SetOut(&dryOut)
		dryCmd.SetArgs([]string{"apply", "--init", "--agents", "claude-code,aider", "--preset", "balanced"})
		Expect(dryCmd.Execute()).To(Succeed())
		Expect(fake.called).To(BeFalse())
		Expect(dryOut.String()).To(ContainSubstring("Would write"))
		Expect(dryOut.String()).To(ContainSubstring("preset: balanced"))
		_, statErr := os.Stat(agentsPath)
		Expect(os.IsNotExist(statErr)).To(BeTrue())

		// Real --init writes minimal agents.yaml and applies it.
		viper.Set("dry-run", false)
		initCmd := NewAgentCmdWithDeps(emptySettings, nil, agentsandbox.Probe{}, fake)
		var initOut bytes.Buffer
		initCmd.SetOut(&initOut)
		initCmd.SetArgs([]string{"apply", "--init", "--agents", "claude-code,codex,aider", "--preset", "balanced"})
		Expect(initCmd.Execute()).To(Succeed())
		Expect(fake.called).To(BeTrue())
		written, readErr := os.ReadFile(agentsPath)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(written)).To(ContainSubstring("- claude"))
		Expect(string(written)).To(ContainSubstring("- codex"))
		Expect(string(written)).NotTo(ContainSubstring("aider"))
		Expect(string(written)).To(ContainSubstring("preset: balanced"))

		// Second --init keeps the existing file untouched and applies the loaded
		// settings, which merge every configuration tier.
		fake2 := &fakeAgentPluginExecutor{}
		loadedSettings := emptySettings
		loadedSettings.Agents = config.AgentsConfig{Enabled: []string{"claude", "codex"}, Preset: "balanced"}
		keepCmd := NewAgentCmdWithDeps(loadedSettings, nil, agentsandbox.Probe{}, fake2)
		var keepOut bytes.Buffer
		keepCmd.SetOut(&keepOut)
		keepCmd.SetArgs([]string{"apply", "--init", "--agents", "gemini-cli", "--preset", "strict"})
		Expect(keepCmd.Execute()).To(Succeed())
		Expect(keepOut.String()).To(ContainSubstring("Kept your existing agents configuration"))
		Expect(fake2.called).To(BeTrue())
		after, readErr := os.ReadFile(agentsPath)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(after)).To(Equal(string(written)))
	})

	It("does not write a user agents.yaml with --init when another tier already configures agents", func() {
		tierSettings := config.CrossPlatformSettings{HomeDir: home, Agents: config.AgentsConfig{Enabled: []string{"codex"}}}
		agentsPath := filepath.Join(tierSettings.GetUserConfigDir(), "agents.yaml")
		viper.Set("dry-run", false)
		tierFake := &fakeAgentPluginExecutor{}
		cmd := NewAgentCmdWithDeps(tierSettings, nil, agentsandbox.Probe{}, tierFake)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"apply", "--init", "--agents", "claude-code"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Kept your existing agents configuration"))
		Expect(tierFake.called).To(BeTrue())
		_, statErr := os.Stat(agentsPath)
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})

	It("applies the harness IDs --init resolved from catalog slugs in DEVEX_AGENTS", func() {
		initSettings := config.CrossPlatformSettings{HomeDir: home}
		viper.Set("dry-run", false)
		GinkgoT().Setenv("DEVEX_AGENTS", "claude-code")
		envFake := &fakeAgentPluginExecutor{}
		cmd := NewAgentCmdWithDeps(initSettings, nil, agentsandbox.Probe{}, envFake)
		cmd.SetOut(io.Discard)
		cmd.SetArgs([]string{"apply", "--init"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(envFake.called).To(BeTrue())
		Expect(string(envFake.specBytes)).To(ContainSubstring(`"name": "claude"`))
	})

	It("renders local_models plan and GPU line in diff and apply --dry-run through root command with zero writes and zero pulls", func() {
		agentsYAML := filepath.Join(home, ".config", "devex", "config", "agents.yaml")
		Expect(os.MkdirAll(filepath.Dir(agentsYAML), 0o750)).To(Succeed())
		Expect(os.WriteFile(agentsYAML, []byte("agents:\n  local_models:\n    - qwen2.5-coder:7b\n    - llama3.2\n"), 0o600)).To(Succeed())

		loaded, loadErr := config.LoadCrossPlatformSettings(home)
		Expect(loadErr).NotTo(HaveOccurred())
		loaded.HomeDir = home
		Expect(loaded.Agents.LocalModels).To(Equal([]string{"qwen2.5-coder:7b", "llama3.2"}))

		ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2:latest"}]}`))
				return
			}
			http.NotFound(w, r)
		}))
		DeferCleanup(ollamaSrv.Close)

		registrySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v2/library/qwen2.5-coder/manifests/7b" {
				_, _ = w.Write([]byte(`{"config":{"size":487},"layers":[{"size":4683074048}]}`))
				return
			}
			http.NotFound(w, r)
		}))
		DeferCleanup(registrySrv.Close)

		var runCalls [][]string
		lmProbe := &localmodels.Probe{
			GOOS:       "linux",
			HTTPClient: ollamaSrv.Client(),
			LookPath: func(name string) (string, error) {
				if name == "ollama" {
					return "/usr/local/bin/ollama", nil
				}
				return "", exec.ErrNotFound
			},
			Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				runCalls = append(runCalls, append([]string{name}, args...))
				return nil, nil
			},
		}
		lmOpts := localmodels.PlanOptions{
			OllamaBaseURL:   ollamaSrv.URL,
			RegistryBaseURL: registrySrv.URL,
		}

		// Clear DEVEX_OFFLINE so registry manifest sizes are fetched from httptest.
		GinkgoT().Setenv("DEVEX_OFFLINE", "")

		for _, args := range [][]string{
			{"agent", "diff"},
			{"agent", "apply", "--dry-run"},
		} {
			viper.Reset()
			root := NewRootCmd("test", mocks.NewMockRepository(), loaded)
			for _, sub := range root.Commands() {
				if sub.Name() == "agent" {
					root.RemoveCommand(sub)
					break
				}
			}
			root.AddCommand(NewAgentCmdFromDeps(AgentCmdDeps{
				Settings:               loaded,
				Executor:               fake,
				LocalModelsProbe:       lmProbe,
				LocalModelsPlanOptions: lmOpts,
			}))
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(args)

			Expect(root.Execute()).To(Succeed())
			Expect(fake.called).To(BeFalse())
			Expect(runCalls).To(BeEmpty())
			Expect(out.String()).To(And(
				ContainSubstring("Local Models:"),
				ContainSubstring("GPU: CPU only"),
				ContainSubstring("qwen2.5-coder:7b: pull (4.7 GB)"),
				ContainSubstring("llama3.2: already present"),
			))
		}
	})

	It("pulls only missing local_models on agent apply and surfaces actionable ollama errors", func() {
		ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2:latest"}]}`))
				return
			}
			http.NotFound(w, r)
		}))
		DeferCleanup(ollamaSrv.Close)

		modelSettings := config.CrossPlatformSettings{
			HomeDir: home,
			Agents: config.AgentsConfig{
				LocalModels: []string{"llama3.2", "qwen2.5-coder:7b"},
			},
		}

		var pulled [][]string
		probe := &localmodels.Probe{
			GOOS:       "linux",
			HTTPClient: ollamaSrv.Client(),
			LookPath:   func(string) (string, error) { return "/usr/local/bin/ollama", nil },
			Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				pulled = append(pulled, append([]string{name}, args...))
				return []byte("success\n"), nil
			},
		}
		cmd := NewAgentCmdFromDeps(AgentCmdDeps{
			Settings:               modelSettings,
			Executor:               fake,
			LocalModelsProbe:       probe,
			LocalModelsPlanOptions: localmodels.PlanOptions{OllamaBaseURL: ollamaSrv.URL},
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"apply"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(pulled).To(Equal([][]string{{"/usr/local/bin/ollama", "pull", "qwen2.5-coder:7b"}}))

		missingProbe := &localmodels.Probe{
			GOOS:     "linux",
			LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		}
		cmdMissing := NewAgentCmdFromDeps(AgentCmdDeps{
			Settings:         modelSettings,
			Executor:         fake,
			LocalModelsProbe: missingProbe,
		})
		cmdMissing.SetOut(&out)
		cmdMissing.SetErr(&out)
		cmdMissing.SetArgs([]string{"apply"})
		Expect(cmdMissing.Execute()).To(MatchError(ContainSubstring("Ollama is not installed: run `devex install ollama`")))
	})
	It("still previews local_models when the tool-agents step fails, and skips pulls offline", func() {
		ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"models":[]}`))
		}))
		DeferCleanup(ollamaSrv.Close)

		combined := config.CrossPlatformSettings{
			HomeDir: home,
			Agents: config.AgentsConfig{
				Enabled:     []string{"claude"},
				LocalModels: []string{"qwen2.5-coder:7b"},
			},
		}
		var pulled [][]string
		probe := &localmodels.Probe{
			GOOS:       "linux",
			HTTPClient: ollamaSrv.Client(),
			LookPath: func(name string) (string, error) {
				if name == "ollama" {
					return "/usr/local/bin/ollama", nil
				}
				return "", exec.ErrNotFound
			},
			Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				pulled = append(pulled, append([]string{name}, args...))
				return nil, nil
			},
		}
		failing := &fakeAgentPluginExecutor{err: errors.New("tool-agents plugin is not installed")}
		newCmd := func(planOffline bool, args ...string) (*cobra.Command, *bytes.Buffer) {
			cmd := NewAgentCmdFromDeps(AgentCmdDeps{
				Settings:               combined,
				Executor:               failing,
				LocalModelsProbe:       probe,
				LocalModelsPlanOptions: localmodels.PlanOptions{OllamaBaseURL: ollamaSrv.URL, Offline: planOffline},
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(args)
			return cmd, &out
		}

		cmd, out := newCmd(true, "diff")
		Expect(cmd.Execute()).To(MatchError(ContainSubstring("tool-agents plugin is not installed")))
		Expect(out.String()).To(ContainSubstring("qwen2.5-coder:7b: pull (size unknown)"))

		viper.Set("offline", true)
		DeferCleanup(viper.Reset)
		failing.err = nil
		// Only the CLI offline setting is on here, so it alone must stop the pull.
		cmd, out = newCmd(false, "apply")
		Expect(cmd.Execute()).To(Succeed())
		Expect(pulled).To(BeEmpty())
		Expect(out.String()).To(ContainSubstring("skipped pulling in offline mode"))
	})
	It("handles local_models from an existing agents.yaml under apply --init", func() {
		ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"models":[]}`))
		}))
		DeferCleanup(ollamaSrv.Close)

		existing := config.CrossPlatformSettings{
			HomeDir: home,
			Agents:  config.AgentsConfig{LocalModels: []string{"qwen2.5-coder:7b"}},
		}
		var pulled [][]string
		probe := &localmodels.Probe{
			GOOS:       "linux",
			HTTPClient: ollamaSrv.Client(),
			LookPath: func(name string) (string, error) {
				if name == "ollama" {
					return "/usr/local/bin/ollama", nil
				}
				return "", exec.ErrNotFound
			},
			Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				pulled = append(pulled, append([]string{name}, args...))
				return nil, nil
			},
		}
		run := func(offline bool, args ...string) string {
			cmd := NewAgentCmdFromDeps(AgentCmdDeps{
				Settings:               existing,
				Executor:               fake,
				LocalModelsProbe:       probe,
				LocalModelsPlanOptions: localmodels.PlanOptions{OllamaBaseURL: ollamaSrv.URL, Offline: offline},
			})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(args)
			Expect(cmd.Execute()).To(Succeed())
			return out.String()
		}

		viper.Set("dry-run", true)
		Expect(run(true, "apply", "--init")).To(ContainSubstring("qwen2.5-coder:7b: pull (size unknown)"))
		Expect(pulled).To(BeEmpty())

		// The preview validates like the real apply: duplicates are rejected.
		dup := NewAgentCmdFromDeps(AgentCmdDeps{
			Settings: config.CrossPlatformSettings{HomeDir: home, Agents: config.AgentsConfig{LocalModels: []string{"llama3.2", "llama3.2:latest"}}},
			Executor: fake, LocalModelsProbe: probe,
		})
		dup.SetOut(io.Discard)
		dup.SetErr(io.Discard)
		dup.SetArgs([]string{"apply", "--init"})
		Expect(dup.Execute()).To(MatchError(ContainSubstring("duplicate local model")))

		viper.Set("dry-run", false)
		DeferCleanup(viper.Reset)
		run(false, "apply", "--init")
		Expect(pulled).To(Equal([][]string{{"/usr/local/bin/ollama", "pull", "qwen2.5-coder:7b"}}))
		Expect(fake.called).To(BeFalse())
	})
})
