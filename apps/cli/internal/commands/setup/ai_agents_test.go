package setup

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentdoctor"
	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func aiAgentCatalogSettings(homeDir string) config.CrossPlatformSettings {
	return config.CrossPlatformSettings{
		HomeDir: homeDir,
		ApplicationOverrides: []types.CrossPlatformApp{
			miseCatalogEntry(),
			{
				Name:        "Bubblewrap",
				Description: "Unprivileged sandboxing tool",
				Category:    "Dependency",
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "apt", InstallCommand: "bubblewrap"},
			},
			{
				Name:        "socat",
				Description: "Multipurpose relay",
				Category:    "Dependency",
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "apt", InstallCommand: "socat"},
			},
			{
				Name:        "Claude Code",
				Description: "Anthropic's agentic coding CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux: types.OSConfig{
					InstallMethod:        "curlpipe",
					DownloadURL:          "https://claude.ai/install.sh",
					InstallScope:         "user",
					InstalledBinary:      "claude",
					SandboxPrerequisites: []string{"Bubblewrap", "socat"},
				},
				MacOS:   types.OSConfig{InstallMethod: "brew", InstallCommand: "claude-code", BrewCask: true},
				Windows: types.OSConfig{InstallMethod: "winget", InstallCommand: "Anthropic.ClaudeCode"},
			},
			{
				Name:        "Codex CLI",
				Description: "OpenAI's local coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux: types.OSConfig{
					InstallMethod:        "mise",
					InstallCommand:       "codex@latest",
					Dependencies:         []string{"mise"},
					SandboxPrerequisites: []string{"Bubblewrap"},
				},
				MacOS:   types.OSConfig{InstallMethod: "brew", InstallCommand: "codex", BrewCask: true},
				Windows: types.OSConfig{InstallMethod: "mise", InstallCommand: "node@lts npm:@openai/codex@latest"},
			},
			{
				Name:        "Gemini CLI",
				Description: "Google's open-source terminal AI coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "node@lts npm:@google/gemini-cli@latest"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "node@lts npm:@google/gemini-cli@latest"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "node@lts npm:@google/gemini-cli@latest"},
			},
			{
				Name:        "Aider",
				Description: "AI pair programming in your terminal",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:aider-chat[python=3.12]@latest"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:aider-chat[python=3.12]@latest"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:aider-chat[python=3.12]@latest"},
			},
			{
				Name:        "fast-agent",
				Description: "MCP-native AI coding agent and workflow CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:fast-agent-mcp[python=3.12]@latest"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:fast-agent-mcp[python=3.12]@latest"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:fast-agent-mcp[python=3.12]@latest"},
			},
			{
				Name:        "Minion Code",
				Description: "AI coding assistant built on the Minion agent framework",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:minion-code[python=3.12]@latest"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:minion-code[python=3.12]@latest"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:minion-code[python=3.12]@latest"},
			},
			{
				Name:        "Mistral Vibe",
				Description: "Mistral AI's open-source terminal coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:mistral-vibe[python=3.14]@latest"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:mistral-vibe[python=3.14]@latest"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:mistral-vibe[python=3.14]@latest"},
			},
			{
				Name:        "Amp",
				Description: "Amp's frontier agentic coding CLI for the terminal",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://ampcode.com/install.sh", InstallScope: "user", InstalledBinary: "amp"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://ampcode.com/install.sh", InstallScope: "user", InstalledBinary: "amp"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:@ampcode/cli@latest"},
			},
			{
				Name:        "Autohand Code",
				Description: "Open-source terminal-native AI coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:autohand-cli@latest", InstalledBinary: "autohand"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:autohand-cli@latest", InstalledBinary: "autohand"},
				Windows:     types.OSConfig{InstallMethod: "winget", InstallCommand: "Autohand.AutohandCodeCLI"},
			},
			{
				Name:        "Cortex Code",
				Description: "Snowflake's terminal AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://ai.snowflake.com/static/cc-scripts/install.sh", InstallScope: "user", InstalledBinary: "cortex"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://ai.snowflake.com/static/cc-scripts/install.sh", InstallScope: "user", InstalledBinary: "cortex"},
			},
			{
				Name:        "crow-cli",
				Description: "Minimal ACP-native AI coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:crow-cli[python=3.13]@latest"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:crow-cli[python=3.13]@latest"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "uv@latest pipx:crow-cli[python=3.13]@latest"},
			},
			{
				Name:        "Cursor CLI",
				Description: "Cursor's standalone terminal AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://cursor.com/install", InstallScope: "user", InstalledBinary: "cursor-agent"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://cursor.com/install", InstallScope: "user", InstalledBinary: "cursor-agent"},
			},
			{
				Name:        "Devin",
				Description: "Cognition's terminal AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:devin@latest", InstalledBinary: "devin"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:devin@latest", InstalledBinary: "devin"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:devin@latest"},
			},
			{
				Name:        "fx",
				Description: "Vercel Labs' tiny native Zig coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://fx.sh/setup.sh", InstallScope: "user", InstalledBinary: "fx"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://fx.sh/setup.sh", InstallScope: "user", InstalledBinary: "fx"},
			},
			{
				Name:        "Harn",
				Description: "Programming language and runtime for AI coding agents",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://harnlang.com/install.sh", InstallScope: "user", InstalledBinary: "harn"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://harnlang.com/install.sh", InstallScope: "user", InstalledBinary: "harn"},
			},
			{
				Name:        "Junie",
				Description: "JetBrains' LLM-agnostic terminal AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://junie.jetbrains.com/install.sh", InstallScope: "user", InstalledBinary: "junie"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://junie.jetbrains.com/install.sh", InstallScope: "user", InstalledBinary: "junie"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:@jetbrains/junie@latest"},
			},
			{
				Name:        "Kilo",
				Description: "Open-source terminal AI coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://kilo.ai/cli/install", InstallScope: "user", InstalledBinary: "kilo"},
				MacOS:       types.OSConfig{InstallMethod: "brew", InstallCommand: "Kilo-Org/tap/kilo"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24 npm:@kilocode/cli@latest"},
			},
			{
				Name:        "Kimchi",
				Description: "Multi-model terminal AI coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "github:getkimchi/kimchi@latest", InstalledBinary: "kimchi"},
				MacOS:       types.OSConfig{InstallMethod: "brew", InstallCommand: "getkimchi/tap/kimchi"},
			},
			{
				Name:        "Kiro CLI",
				Description: "Amazon's terminal AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://cli.kiro.dev/install", InstallScope: "user", InstalledBinary: "kiro-cli"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://cli.kiro.dev/install", InstallScope: "user", InstalledBinary: "kiro-cli"},
			},
			{
				Name:        "Muse Code",
				Description: "Meta's terminal AI coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://dev.meta.ai/install.sh", InstallScope: "user", InstalledBinary: "muse"},
				MacOS:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://dev.meta.ai/install.sh", InstallScope: "user", InstalledBinary: "muse"},
			},
			{
				Name:        "Poolside",
				Description: "Poolside's terminal and ACP AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "github:poolsideai/pool@latest"},
				MacOS:       types.OSConfig{InstallMethod: "mise", InstallCommand: "github:poolsideai/pool@latest"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "github:poolsideai/pool@latest"},
			},
			{
				Name:        "Stakpak",
				Description: "Open-source DevOps AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "mise", InstallCommand: "github:stakpak/agent@latest"},
				MacOS:       types.OSConfig{InstallMethod: "brew", InstallCommand: "stakpak/stakpak/stakpak"},
				Windows:     types.OSConfig{InstallMethod: "mise", InstallCommand: "github:stakpak/agent@latest"},
			},
			{
				Name:        "VT Code",
				Description: "Open-source Rust terminal coding agent",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux:       types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://raw.githubusercontent.com/vinhnx/vtcode/main/scripts/install.sh", InstallScope: "user", InstalledBinary: "vtcode"},
				MacOS:       types.OSConfig{InstallMethod: "brew", InstallCommand: "vinhnx/tap/vtcode"},
			},
		},
	}
}

var _ = Describe("AI Coding Agents Setup Flow", func() {
	var (
		homeDir  string
		settings config.CrossPlatformSettings
		repo     *mocks.MockRepository
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		GinkgoT().Setenv("SHELL", "/bin/bash")

		homeDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), homeDir)
		settings = aiAgentCatalogSettings(homeDir)
		repo = mocks.NewMockRepository()
	})

	Describe("Step navigation and multi-select UI", func() {
		It("navigates forward and backward across Linux bash, Linux zsh, and Windows paths", func() {
			// Linux bash with nothing selected -> skips preset step.
			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux", Distribution: "ubuntu"})
			model.step = StepDatabases
			Expect(model.selections.selectedAIAgents).To(BeEmpty())

			model, _ = model.nextStep()
			Expect(model.step).To(Equal(StepAIAgents))

			view := model.View()
			Expect(view).To(ContainSubstring("Agent CLIs"))
			Expect(view).NotTo(ContainSubstring("AI editors"))
			Expect(view).NotTo(ContainSubstring("Local models"))
			Expect(view).To(ContainSubstring("Claude Code"))

			// Enter with nothing selected skips preset and advances to Shell.
			model, _ = model.handleEnter()
			Expect(model.step).To(Equal(StepShell))

			// Going back from Shell returns to AI Agents.
			model, _ = model.prevStep()
			Expect(model.step).To(Equal(StepAIAgents))

			// Going back from AI Agents returns to Databases.
			model, _ = model.prevStep()
			Expect(model.step).To(Equal(StepDatabases))

			// Select a harness agent (Claude Code) and advance -> reaches Preset step.
			model, _ = model.nextStep()
			Expect(model.step).To(Equal(StepAIAgents))
			model.selections.selectedAIAgents["Claude Code"] = true
			model, _ = model.handleEnter()
			Expect(model.step).To(Equal(StepAIAgentPreset))
			Expect(model.View()).To(ContainSubstring("Leave permission modes unchanged"))
			Expect(model.View()).To(ContainSubstring("balanced"))
			Expect(model.View()).To(ContainSubstring("strict"))

			// Advance to Shell, select zsh, verify zsh -> OhMyZsh -> Theme and full reverse back to Preset -> AIAgents.
			model, _ = model.handleEnter()
			Expect(model.step).To(Equal(StepShell))
			model.selections.selectedShell = 0 // zsh
			model, _ = model.nextStep()
			Expect(model.step).To(Equal(StepOhMyZsh))
			model, _ = model.nextStep()
			Expect(model.step).To(Equal(StepTheme))
			model, _ = model.prevStep()
			Expect(model.step).To(Equal(StepOhMyZsh))
			model, _ = model.prevStep()
			Expect(model.step).To(Equal(StepShell))
			model, _ = model.prevStep()
			Expect(model.step).To(Equal(StepAIAgentPreset))
			model, _ = model.prevStep()
			Expect(model.step).To(Equal(StepAIAgents))

			// Windows path skips Shell and goes directly to Theme (and back from Theme).
			winModel := NewSetupModel(repo, settings, platform.DetectionResult{OS: "windows"})
			winModel.step = StepDatabases
			winModel, _ = winModel.nextStep()
			Expect(winModel.step).To(Equal(StepAIAgents))
			winModel, _ = winModel.handleEnter()
			Expect(winModel.step).To(Equal(StepTheme))
			winModel, _ = winModel.prevStep()
			Expect(winModel.step).To(Equal(StepAIAgents))

			winModel.selections.selectedAIAgents["Claude Code"] = true
			winModel, _ = winModel.handleEnter()
			Expect(winModel.step).To(Equal(StepAIAgentPreset))
			winModel, _ = winModel.handleEnter()
			Expect(winModel.step).To(Equal(StepTheme))
			winModel, _ = winModel.prevStep()
			Expect(winModel.step).To(Equal(StepAIAgentPreset))
		})

		It("offers the preset step only when a selected app maps to a harness ID", func() {
			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.step = StepAIAgents
			// Select non-harness agents only -> preset step is skipped.
			for _, name := range []string{
				"Aider", "Amp", "Autohand Code", "Cortex Code", "crow-cli", "Cursor CLI",
				"Devin", "fast-agent", "fx", "Harn", "Junie", "Kilo", "Kimchi",
				"Kiro CLI", "Minion Code", "Mistral Vibe", "Muse Code", "Poolside",
				"Stakpak", "VT Code",
			} {
				model.step = StepAIAgents
				model.selections.selectedAIAgents = map[string]bool{name: true}
				model, _ = model.handleEnter()
				Expect(model.step).To(Equal(StepShell), name)
			}

			// Select Claude Code (harness agent) -> preset step is shown.
			model, _ = model.prevStep()
			Expect(model.step).To(Equal(StepAIAgents))
			model.selections.selectedAIAgents["Claude Code"] = true
			model, _ = model.handleEnter()
			Expect(model.step).To(Equal(StepAIAgentPreset))

			// Space toggles preset options.
			model, _ = model.handleDown()
			model, _ = model.handleSpace()
			Expect(model.selections.aiAgentPreset).To(Equal(config.AgentPresetBalanced))
			model, _ = model.handleDown()
			model, _ = model.handleSpace()
			Expect(model.selections.aiAgentPreset).To(Equal(config.AgentPresetStrict))
		})

		It("shows additional group headings only when apps exist in those categories", func() {
			settings.ApplicationOverrides = append(settings.ApplicationOverrides,
				types.CrossPlatformApp{Name: "Herdr", Category: CategoryAIAgentHarnesses, Linux: types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://herdr.dev/install.sh", InstallScope: "user", InstalledBinary: "herdr"}},
				types.CrossPlatformApp{Name: "Cursor", Category: CategoryAIEditors, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "cursor"}},
				types.CrossPlatformApp{Name: "Ollama", Category: CategoryLocalModels, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "ollama"}},
			)
			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.step = StepAIAgents
			model.cursor = 0
			// The list scrolls once the catalog has more agents than fit, so
			// collect every window while moving down through all of them.
			var views strings.Builder
			for i := 0; i < 200; i++ {
				views.WriteString(model.View())
				previous := model.cursor
				model, _ = model.handleDown()
				if model.cursor == previous {
					break
				}
			}
			view := views.String()
			model.cursor = 0
			Expect(view).To(ContainSubstring("Agent CLIs"))
			Expect(view).To(ContainSubstring("Agent harnesses"))
			Expect(view).To(ContainSubstring("AI editors"))
			Expect(view).To(ContainSubstring("Local models"))
			Expect(view).To(ContainSubstring("Herdr"))
			Expect(view).To(ContainSubstring("Cursor"))
			Expect(view).To(ContainSubstring("Ollama"))

			// Toggling via keyboard space selects the first entry (Aider, alphabetical in Agent CLIs).
			updated, _ := model.Update(tea.KeyMsg{Type: tea.KeySpace})
			model = updated.(*SetupModel)
			Expect(model.selections.selectedAIAgents).To(HaveKeyWithValue("Aider", true))
		})
	})

	Describe("Install plan and sandbox prerequisites", func() {
		It("adds selected agents and expands Linux sandbox prerequisites before them", func() {
			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.selections.selectedAIAgents["Claude Code"] = true
			model.selections.selectedAIAgents["Codex CLI"] = true
			model.selections.aiAgentPreset = config.AgentPresetBalanced

			apps := model.buildAppList()
			names := make([]string, 0, len(apps))
			for _, app := range apps {
				names = append(names, app.Name)
			}
			Expect(names).To(ContainElements("mise", "Bubblewrap", "socat", "Claude Code", "Codex CLI"))
			Expect(indexOf(names, "Bubblewrap")).To(BeNumerically("<", indexOf(names, "Claude Code")))
			Expect(indexOf(names, "socat")).To(BeNumerically("<", indexOf(names, "Claude Code")))
			Expect(indexOf(names, "Bubblewrap")).To(BeNumerically("<", indexOf(names, "Codex CLI")))

			// Deduplication: Bubblewrap appears once even though both Claude Code and Codex CLI require it.
			bwrapCount := 0
			for _, n := range names {
				if n == "Bubblewrap" {
					bwrapCount++
				}
			}
			Expect(bwrapCount).To(Equal(1))

			planView := model.renderInstallPlan()
			Expect(planView).To(ContainSubstring("Bubblewrap"))
			Expect(planView).To(ContainSubstring("socat"))
			Expect(planView).To(ContainSubstring("Claude Code"))
			Expect(planView).To(ContainSubstring("agents.yaml"))
			Expect(planView).To(ContainSubstring("preset: balanced"))
		})

		It("leaves agents and agents.yaml out of the plan when nothing is selected", func() {
			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			apps := model.buildAppList()
			for _, app := range apps {
				Expect(app.Category).NotTo(Equal(CategoryAICodingAgents))
				Expect(app.Name).NotTo(Equal("Bubblewrap"))
				Expect(app.Name).NotTo(Equal("socat"))
			}
			Expect(model.renderInstallPlan()).NotTo(ContainSubstring("agents.yaml"))
		})
	})

	Describe("agents.yaml writing, apply, warnings, and doctor summary", func() {
		var (
			origApply  func(context.Context, *bootstrap.PluginBootstrap, config.AgentsConfig) error
			origProbe  func() agentdoctor.Probe
			applyCalls []config.AgentsConfig
			applyErr   error
		)

		BeforeEach(func() {
			applyCalls = nil
			applyErr = nil
			origApply = applyAIAgentsConfig
			origProbe = newAgentDoctorProbe
			applyAIAgentsConfig = func(_ context.Context, _ *bootstrap.PluginBootstrap, cfg config.AgentsConfig) error {
				applyCalls = append(applyCalls, cfg)
				return applyErr
			}
			DeferCleanup(func() {
				applyAIAgentsConfig = origApply
				newAgentDoctorProbe = origProbe
			})
		})

		fakeProbeWithSecret := func(secret string) agentdoctor.Probe {
			binDir := filepath.Join(homeDir, "fake-bin")
			Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
			for _, bin := range []string{"claude", "gemini"} {
				p := filepath.Join(binDir, bin)
				Expect(os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755)).To(Succeed())
			}
			return agentdoctor.Probe{
				GOOS: "linux",
				Now:  func() time.Time { return time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC) },
				Getenv: func(key string) string {
					switch key {
					case "HOME":
						return homeDir
					case "PATH":
						return binDir
					case "ANTHROPIC_API_KEY":
						return secret
					default:
						return ""
					}
				},
				Stat: os.Stat,
				ReadFile: func(path string) ([]byte, error) {
					return nil, os.ErrNotExist
				},
				Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
					if len(args) > 0 && args[0] == "--version" {
						// Deliberately echo secret in version output to prove redaction works.
						return []byte("1.2.3 token=" + secret + "\n"), nil
					}
					return nil, nil
				},
				EvalSymlinks: func(path string) (string, error) { return path, nil },
				UserHomeDir:  func() (string, error) { return homeDir, nil },
				SandboxProbe: agentsandbox.Probe{GOOS: "linux"},
			}
		}

		It("writes minimal agents.yaml when absent, applies it, and renders redacted doctor + login summary", func() {
			const secretVal = "sk-ant-super-secret-value-1234567890"
			newAgentDoctorProbe = func() agentdoctor.Probe { return fakeProbeWithSecret(secretVal) }

			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.selections.selectedAIAgents["Claude Code"] = true
			model.selections.selectedAIAgents["Gemini CLI"] = true
			model.selections.aiAgentPreset = config.AgentPresetStrict
			model.installation.outcomeRecorder().Record("Claude Code", tui.OutcomeInstalled, nil)
			model.installation.outcomeRecorder().Record("Gemini CLI", tui.OutcomeInstalled, nil)

			recordSelectedAgentsInstalled(model)
			model.handleAIAgentsConfiguration(context.Background(), nil)

			agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
			data, err := os.ReadFile(agentsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("- claude"))
			Expect(string(data)).To(ContainSubstring("- gemini"))
			Expect(string(data)).To(ContainSubstring("preset: strict"))

			Expect(applyCalls).To(HaveLen(1))
			Expect(applyCalls[0].Enabled).To(Equal([]string{"claude", "gemini"}))
			Expect(applyCalls[0].Preset).To(Equal("strict"))
			Expect(model.HasErrors()).To(BeFalse())

			// Verify both TUI completion view and automated completion stdout include doctor + login lines and NO secret.
			model.step = StepComplete
			tuiView := model.View()
			Expect(tuiView).To(ContainSubstring("Agent Doctor"))
			Expect(tuiView).To(ContainSubstring("claude"))
			Expect(tuiView).To(ContainSubstring("devex agent login claude"))
			Expect(tuiView).To(ContainSubstring("Gemini CLI"))
			Expect(tuiView).To(ContainSubstring("/auth"))
			Expect(tuiView).NotTo(ContainSubstring(secretVal))

			stdout := captureStdout(func() {
				printAutomatedSetupCompletion(model)
			})
			Expect(stdout).To(ContainSubstring("Agent Doctor"))
			Expect(stdout).To(ContainSubstring("devex agent login claude"))
			Expect(stdout).To(ContainSubstring("Gemini CLI"))
			Expect(stdout).NotTo(ContainSubstring(secretVal))
		})

		It("omits preset from written agents.yaml when left unchanged", func() {
			newAgentDoctorProbe = func() agentdoctor.Probe { return fakeProbeWithSecret("") }

			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.selections.selectedAIAgents["Claude Code"] = true
			model.selections.aiAgentPreset = ""

			recordSelectedAgentsInstalled(model)
			model.handleAIAgentsConfiguration(context.Background(), nil)

			agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
			data, err := os.ReadFile(agentsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("- claude"))
			Expect(string(data)).NotTo(ContainSubstring("preset:"))
		})

		It("keeps an existing agents.yaml untouched and notes it in the summary", func() {
			newAgentDoctorProbe = func() agentdoctor.Probe { return fakeProbeWithSecret("") }

			agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
			Expect(os.MkdirAll(filepath.Dir(agentsPath), 0o750)).To(Succeed())
			original := "agents:\n  enabled:\n    - claude\n  instructions: Keep my custom instructions\n"
			Expect(os.WriteFile(agentsPath, []byte(original), 0o600)).To(Succeed())

			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.selections.selectedAIAgents["Claude Code"] = true
			model.selections.selectedAIAgents["Codex CLI"] = true
			model.selections.aiAgentPreset = config.AgentPresetStrict

			recordSelectedAgentsInstalled(model)
			model.handleAIAgentsConfiguration(context.Background(), nil)

			after, err := os.ReadFile(agentsPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(after)).To(Equal(original))
			Expect(model.installation.getChanges()).To(ContainElement(keptAgentsConfigNote))
			Expect(applyCalls).To(BeEmpty())
		})

		It("neither writes nor applies agents.yaml when another tier already configures agents", func() {
			newAgentDoctorProbe = func() agentdoctor.Probe { return fakeProbeWithSecret("") }

			tierSettings := settings
			tierSettings.Agents = config.AgentsConfig{Enabled: []string{"codex"}}
			model := NewSetupModel(repo, tierSettings, platform.DetectionResult{OS: "linux"})
			model.selections.selectedAIAgents["Claude Code"] = true

			Expect(model.renderAIAgentsPreview()).To(ContainSubstring("Keep existing agents configuration unchanged"))
			recordSelectedAgentsInstalled(model)
			model.handleAIAgentsConfiguration(context.Background(), nil)

			_, statErr := os.Stat(filepath.Join(tierSettings.GetUserConfigDir(), "agents.yaml"))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
			Expect(applyCalls).To(BeEmpty())
			Expect(model.installation.getChanges()).To(ContainElement(keptAgentsConfigNote))
		})

		It("configures only agents whose install succeeded", func() {
			newAgentDoctorProbe = func() agentdoctor.Probe { return fakeProbeWithSecret("") }
			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.selections.selectedAIAgents["Claude Code"] = true
			model.selections.selectedAIAgents["Codex CLI"] = true
			model.installation.outcomeRecorder().Record("Claude Code", tui.OutcomeNotRun, nil)

			model.handleAIAgentsConfiguration(context.Background(), nil)

			_, statErr := os.Stat(filepath.Join(settings.GetUserConfigDir(), "agents.yaml"))
			Expect(os.IsNotExist(statErr)).To(BeTrue())
			Expect(applyCalls).To(BeEmpty())
		})

		It("reports tool-agents apply failures as warnings without failing setup", func() {
			newAgentDoctorProbe = func() agentdoctor.Probe { return fakeProbeWithSecret("") }
			applyErr = errors.New("tool-agents plugin unavailable")

			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})
			model.selections.selectedAIAgents["Claude Code"] = true

			recordSelectedAgentsInstalled(model)
			model.handleAIAgentsConfiguration(context.Background(), nil)

			Expect(model.HasErrors()).To(BeFalse())
			Expect(model.CompletionError()).NotTo(HaveOccurred())
			// setup prints this on the normal terminal after the wizard's alternate screen closes.
			var finalModel tea.Model = model
			summarizer, ok := finalModel.(interface{ AIAgentsSummary() string })
			Expect(ok).To(BeTrue())
			Expect(summarizer.AIAgentsSummary()).To(ContainSubstring("tool-agents plugin unavailable"))
			model.step = StepComplete
			Expect(model.View()).To(ContainSubstring("AI coding agent warnings"))
			Expect(model.View()).To(ContainSubstring("tool-agents plugin unavailable"))
		})

		It("writes nothing and applies nothing when no agents are selected", func() {
			newAgentDoctorProbe = func() agentdoctor.Probe { return fakeProbeWithSecret("") }
			model := NewSetupModel(repo, settings, platform.DetectionResult{OS: "linux"})

			recordSelectedAgentsInstalled(model)
			model.handleAIAgentsConfiguration(context.Background(), nil)

			agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
			_, err := os.Stat(agentsPath)
			Expect(os.IsNotExist(err)).To(BeTrue())
			Expect(applyCalls).To(BeEmpty())
			Expect(model.renderAIAgentsSummary()).To(BeEmpty())
		})

		It("writes nothing and applies nothing during dry-run automated setup", func() {
			if runtime.GOOS != "linux" {
				Skip("fixtures model Linux automated setup")
			}
			origCheck := installedAppsCheck
			installedAppsCheck = func(context.Context, []types.CrossPlatformApp) map[string]bool { return map[string]bool{} }
			DeferCleanup(func() { installedAppsCheck = origCheck })

			viper.Set("dry-run", true)
			viper.Set("setup.ai_agents.apps", []string{"Claude Code"})
			viper.Set("setup.ai_agents.preset", "balanced")

			stdout := captureStdout(func() {
				Expect(RunAutomatedSetup(context.Background(), repo, settings)).To(Succeed())
			})
			Expect(stdout).To(ContainSubstring("Claude Code"))
			Expect(stdout).To(ContainSubstring("Bubblewrap"))
			Expect(stdout).To(ContainSubstring("socat"))
			Expect(stdout).To(ContainSubstring("agents.yaml"))
			Expect(stdout).To(ContainSubstring("preset: balanced"))

			agentsPath := filepath.Join(settings.GetUserConfigDir(), "agents.yaml")
			_, err := os.Stat(agentsPath)
			Expect(os.IsNotExist(err)).To(BeTrue())
			Expect(applyCalls).To(BeEmpty())
		})
	})

	Describe("Non-interactive configuration parsing", func() {
		It("matches configured agent names case-insensitively and populates the automated plan", func() {
			if runtime.GOOS != "linux" {
				Skip("fixtures model Linux automated setup")
			}
			viper.Set("setup.ai_agents.apps", []string{
				"claude code", "AIDER", "amp", "autohand code", "cortex code", "crow-cli",
				"cursor cli", "devin", "fast-agent", "FX", "harn", "junie", "kilo",
				"kimchi", "kiro cli", "MINION CODE", "mistral vibe", "muse code",
				"poolside", "stakpak", "vt code",
			})
			viper.Set("setup.ai_agents.preset", "balanced")

			model, err := createAutomatedSetupModel(repo, settings)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.selections.selectedAIAgents).To(Equal(map[string]bool{
				"Claude Code":   true,
				"Aider":         true,
				"Amp":           true,
				"Autohand Code": true,
				"Cortex Code":   true,
				"crow-cli":      true,
				"Cursor CLI":    true,
				"Devin":         true,
				"fast-agent":    true,
				"fx":            true,
				"Harn":          true,
				"Junie":         true,
				"Kilo":          true,
				"Kimchi":        true,
				"Kiro CLI":      true,
				"Minion Code":   true,
				"Mistral Vibe":  true,
				"Muse Code":     true,
				"Poolside":      true,
				"Stakpak":       true,
				"VT Code":       true,
			}))
			Expect(model.selections.aiAgentPreset).To(Equal("balanced"))

			apps := model.buildAppList()
			names := make([]string, 0, len(apps))
			for _, app := range apps {
				names = append(names, app.Name)
			}
			Expect(names).To(ContainElements(
				"Bubblewrap", "socat", "Claude Code", "Aider", "Amp", "Autohand Code",
				"Cortex Code", "crow-cli", "Cursor CLI", "Devin", "fast-agent", "fx",
				"Harn", "Junie", "Kilo", "Kimchi", "Kiro CLI", "Minion Code",
				"Mistral Vibe", "Muse Code", "Poolside", "Stakpak", "VT Code",
			))
		})

		It("skips AI agents by default when no config is set", func() {
			model, err := createAutomatedSetupModel(repo, settings)
			Expect(err).NotTo(HaveOccurred())
			Expect(model.selections.selectedAIAgents).To(BeEmpty())
			for _, app := range model.buildAppList() {
				Expect(app.Category).NotTo(Equal(CategoryAICodingAgents))
			}
		})

		It("rejects unknown agent names with a clear list of valid names", func() {
			if runtime.GOOS != "linux" {
				Skip("fixtures model Linux automated setup")
			}
			viper.Set("setup.ai_agents.apps", []string{"NotAnAgent"})
			_, err := createAutomatedSetupModel(repo, settings)
			Expect(err).To(MatchError(And(
				ContainSubstring(`unknown AI coding agent "NotAnAgent"`),
				ContainSubstring("Claude Code"),
				ContainSubstring("Codex CLI"),
			)))
		})

		It("rejects invalid and forbidden presets", func() {
			viper.Set("setup.ai_agents.preset", "yolo")
			_, err := createAutomatedSetupModel(repo, settings)
			Expect(err).To(MatchError(ContainSubstring(config.ErrAgentsSkipPermissions)))

			viper.Set("setup.ai_agents.preset", "unknown-preset")
			_, err = createAutomatedSetupModel(repo, settings)
			Expect(err).To(MatchError(ContainSubstring("unsupported preset")))
		})
	})

	Describe("Shipped catalog AI coding agents", func() {
		var shippedSettings config.CrossPlatformSettings

		BeforeEach(func() {
			GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
			GinkgoT().Setenv("DEVEX_ENV", "dev")
			base := config.CrossPlatformSettings{HomeDir: homeDir}
			Expect(config.WriteDefaultConfig(base.GetConfigDir())).To(Succeed())
			loaded, err := config.LoadCrossPlatformSettings(homeDir)
			Expect(err).NotTo(HaveOccurred())
			shippedSettings = loaded
		})

		It("exposes all newly added AI coding agents in StepAIAgents and automated setup", func() {
			if runtime.GOOS != "linux" {
				Skip("shipped Linux catalog setup test")
			}
			newAgents := []string{
				"Auggie CLI",
				"Bob",
				"Cline",
				"Codebuddy Code",
				"DeepAgents",
				"DimCode",
				"Dirac",
				"Factory Droid",
				"Grok Build",
				"MiniMax Code",
				"Nova",
				"Pi",
				"Qoder CLI",
				"Qwen Code",
				"siGit Code",
			}

			detected := platform.DetectionResult{OS: "linux", Distribution: "ubuntu"}
			available := getAvailableAIAgentApps(shippedSettings, detected)
			availableNames := make([]string, 0, len(available))
			for _, app := range available {
				availableNames = append(availableNames, app.Name)
			}
			for _, name := range newAgents {
				Expect(availableNames).To(ContainElement(name))
			}

			model := NewSetupModel(repo, shippedSettings, detected)
			model.step = StepAIAgents
			view := model.View()
			Expect(view).To(ContainSubstring("Agent CLIs"))
			Expect(view).To(ContainSubstring("Auggie CLI"))

			// Selecting non-harness agents skips StepAIAgentPreset and advances directly to StepShell.
			for _, name := range newAgents {
				model.selections.selectedAIAgents[name] = true
			}
			model, _ = model.handleEnter()
			Expect(model.step).To(Equal(StepShell))

			viper.Set("setup.ai_agents.apps", newAgents)
			autoModel, err := createAutomatedSetupModel(repo, shippedSettings)
			Expect(err).NotTo(HaveOccurred())
			for _, name := range newAgents {
				Expect(autoModel.selections.selectedAIAgents).To(HaveKeyWithValue(name, true))
			}

			apps := autoModel.buildAppList()
			names := make([]string, 0, len(apps))
			for _, app := range apps {
				names = append(names, app.Name)
			}
			for _, name := range newAgents {
				Expect(names).To(ContainElement(name))
			}
		})
	})
})

func indexOf(items []string, needle string) int {
	for i, item := range items {
		if item == needle {
			return i
		}
	}
	return -1
}

func captureStdout(fn func()) string {
	orig := os.Stdout
	r, w, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	fn()
	Expect(w.Close()).To(Succeed())
	out := <-done
	_ = r.Close()
	return strings.TrimSpace(out)
}

// recordSelectedAgentsInstalled records a successful install for every selected agent.
func recordSelectedAgentsInstalled(model *SetupModel) {
	for _, app := range model.getSelectedAIAgentApps() {
		model.installation.outcomeRecorder().Record(app.Name, tui.OutcomeInstalled, nil)
	}
}
