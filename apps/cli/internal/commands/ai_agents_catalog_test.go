package commands_test

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var _ = Describe("AI Coding Agents catalog", func() {
	var (
		homeDir  string
		settings config.CrossPlatformSettings
		agents   map[string]types.CrossPlatformApp
		files    []string
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogArchForTesting("amd64"))

		homeDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), homeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")

		settings = config.CrossPlatformSettings{HomeDir: homeDir}
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())

		loaded, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		settings = loaded

		agents = map[string]types.CrossPlatformApp{}
		files = nil
		Expect(fs.WalkDir(shipconfig.Files, "applications/ai-agents", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
				return walkErr
			}
			data, readErr := fs.ReadFile(shipconfig.Files, path)
			if readErr != nil {
				return readErr
			}
			var app types.CrossPlatformApp
			if unmarshalErr := yaml.Unmarshal(data, &app); unmarshalErr != nil {
				return unmarshalErr
			}
			if app.Category != "AI Coding Agents" {
				return nil
			}
			agents[app.Name] = app
			files = append(files, filepath.Base(path))
			return nil
		})).To(Succeed())
		sort.Strings(files)
	})

	It("ships and strictly validates every AI coding agent definition", func() {
		Expect(files).To(Equal([]string{
			"aider.yaml",
			"claude-code.yaml",
			"codex.yaml",
			"copilot-cli.yaml",
			"gemini-cli.yaml",
			"goose.yaml",
			"kimi-code.yaml",
			"lm-studio.yaml",
			"ollama.yaml",
			"opencode.yaml",
		}))

		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "diagnostics: %+v", result.Diagnostics)
		Expect(result.Warnings).To(BeZero())

		for name, app := range agents {
			Expect(app.Default).To(BeFalse(), "%s must opt in with default: false", name)
			wantCategory := "AI Coding Agents"
			if name == "Ollama" || name == "LM Studio" {
				// Grouped under setup's "Local models" heading, not agent CLIs.
				wantCategory = "Local Models"
			}
			Expect(app.Category).To(Equal(wantCategory), name)
			Expect(app.Linux.InstallMethod).NotTo(BeEmpty(), "%s must define a Linux install method", name)
			Expect(app.Linux.OfficialSupport).To(BeTrue(), "%s Linux primary must have official_support: true", name)
			Expect(app.Linux.CleanupFiles).To(BeEmpty(), "%s must not list user data in cleanup_files", name)
			Expect(app.Linux.ConfigFiles).To(BeEmpty(), "%s must not list user config in config_files", name)

			checkCurlpipe := func(label string, cfg types.OSConfig) {
				if cfg.InstallMethod != "curlpipe" {
					return
				}
				if cfg.DownloadURL == "" && (len(cfg.PreInstall) > 0 || len(cfg.PostInstall) > 0) {
					Expect(cfg.InstallScope).To(BeEmpty(), "%s (%s) hook-driven system curlpipe must not set install_scope", name, label)
				} else {
					Expect(cfg.InstallScope).To(Equal("user"), "%s (%s) curlpipe must use install_scope: user", name, label)
					Expect(cfg.DownloadURL).To(HavePrefix("https://"), "%s (%s) curlpipe must use HTTPS download_url", name, label)
				}
				Expect(cfg.InstalledBinary).NotTo(BeEmpty(), "%s (%s) curlpipe must declare installed_binary", name, label)
				Expect(cfg.UninstallCommand).NotTo(BeEmpty(), "%s (%s) curlpipe must declare uninstall_command", name, label)
			}
			checkCurlpipe("linux", app.Linux)
			checkCurlpipe("macos", app.MacOS)
			checkCurlpipe("windows", app.Windows)

			legacy := app.ToLegacyAppConfig()
			Expect(legacy.SelfUpdating).To(Equal(app.Linux.SelfUpdating), name)
			Expect(legacy.InstallScope).To(Equal(app.Linux.InstallScope), name)
		}
	})

	DescribeTable("resolves each AI coding agent install plan on Ubuntu 24.04",
		func(name, wantMethod, wantTarget, wantScope string, wantSelfUpdating bool, wantDeps, wantSandboxPrereqs []string, wantUninstall string) {
			app, ok := agents[name]
			Expect(ok).To(BeTrue(), "expected %s in ai-agents catalog", name)

			ubuntu := platform.DetectionResult{
				OS:               "linux",
				Distribution:     "ubuntu",
				DistributionLike: []string{"debian"},
				Version:          "24.04",
				Architecture:     "amd64",
			}
			resolved := app.LinuxConfigForPlatform(ubuntu)
			Expect(resolved.InstallMethod).To(Equal(wantMethod))
			Expect(resolved.InstallScope).To(Equal(wantScope))
			Expect(resolved.SelfUpdating).To(Equal(wantSelfUpdating))
			Expect(resolved.UninstallCommand).To(Equal(wantUninstall))
			Expect(resolved.SandboxPrerequisites).To(Equal(wantSandboxPrereqs))

			switch wantMethod {
			case "curlpipe":
				Expect(resolved.DownloadURL).To(Equal(wantTarget))
				if wantTarget != "" {
					Expect(resolved.InstallScope).To(Equal("user"))
				}
			case "deb":
				Expect(resolved.DownloadURL).To(Equal(wantTarget))
			case "mise":
				Expect(resolved.InstallCommand).To(Equal(wantTarget))
				packages, err := installers.CatalogPackageArguments("mise", resolved.InstallCommand)
				Expect(err).NotTo(HaveOccurred())
				Expect(packages).To(Equal(strings.Fields(wantTarget)))
				uninstallArgs, err := installers.CatalogPackageArguments("mise", resolved.UninstallCommand)
				Expect(err).NotTo(HaveOccurred())
				Expect(uninstallArgs).To(Equal([]string{wantUninstall}))
			}

			plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.DependencyOrder).To(ContainElement(name))
			for _, prereq := range wantSandboxPrereqs {
				Expect(plan.DependencyOrder).To(ContainElement(prereq))
			}
			for _, dep := range wantDeps {
				if strings.EqualFold(dep, "Node.js") {
					Expect(plan.DependencyOrder).To(Equal([]string{dep, name}))
				}
			}

			var planned *agentapi.PlannedApp
			for i := range plan.Apps {
				if plan.Apps[i].Name == name {
					planned = &plan.Apps[i]
					break
				}
			}
			Expect(planned).NotTo(BeNil())
			Expect(planned.Method).To(Equal(wantMethod))
			Expect(planned.Plugin).To(Equal("package-manager-" + wantMethod))
			if len(wantDeps) > 0 {
				Expect(planned.Dependencies).To(Equal(wantDeps))
			}

			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"install", "--dry-run", name})
			Expect(cmd.Execute()).To(Succeed())
			for _, prereq := range wantSandboxPrereqs {
				Expect(out.String()).To(ContainSubstring("Would install " + prereq))
			}
			Expect(out.String()).To(ContainSubstring("Would install " + name))
		},
		Entry("Claude Code", "Claude Code", "curlpipe", "https://claude.ai/install.sh", "user", true, nil, []string{"Bubblewrap", "socat"}, `rm -f "$HOME/.local/bin/claude" && rm -rf "$HOME/.local/share/claude"`),
		Entry("Codex CLI", "Codex CLI", "mise", "codex@latest", "", false, []string{"mise"}, []string{"Bubblewrap"}, "codex"),
		Entry("Gemini CLI", "Gemini CLI", "mise", "node@24 npm:@google/gemini-cli@latest", "", true, []string{"Node.js"}, nil, "npm:@google/gemini-cli"),
		Entry("GitHub Copilot CLI", "GitHub Copilot CLI", "curlpipe", "https://raw.githubusercontent.com/github/copilot-cli/main/install.sh", "user", true, nil, []string{"Bubblewrap"}, `rm -f "$HOME/.local/bin/copilot"`),
		Entry("OpenCode", "OpenCode", "curlpipe", "https://opencode.ai/install", "user", true, nil, nil, `rm -f "$HOME/.opencode/bin/opencode"`),
		Entry("Kimi Code", "Kimi Code", "curlpipe", "https://code.kimi.com/kimi-code/install.sh", "user", false, nil, nil, `rm -f "$HOME/.kimi-code/bin/kimi" "$HOME/.kimi-code/bin/kimi.bak"`),
		Entry("Aider", "Aider", "mise", "uv@latest pipx:aider-chat[python=3.12]@latest", "", false, []string{"mise"}, nil, "pipx:aider-chat"),
		Entry("Goose", "Goose", "mise", "aqua:aaif-goose/goose@latest", "", false, []string{"mise"}, nil, "aqua:aaif-goose/goose"),
		Entry("LM Studio", "LM Studio", "deb", "https://lmstudio.ai/download/latest/linux/x64?format=deb", "", false, nil, nil, "lm-studio"),
	)

	It("resolves Ollama with system hook-driven install, curl/zstd dependencies, and full cleanup uninstall", func() {
		app, ok := agents["Ollama"]
		Expect(ok).To(BeTrue(), "expected Ollama in ai-agents catalog")

		ubuntu := platform.DetectionResult{
			OS:               "linux",
			Distribution:     "ubuntu",
			DistributionLike: []string{"debian"},
			Version:          "24.04",
			Architecture:     "amd64",
		}
		resolved := app.LinuxConfigForPlatform(ubuntu)
		Expect(resolved.InstallMethod).To(Equal("curlpipe"))
		Expect(resolved.InstallScope).To(BeEmpty())
		Expect(resolved.InstalledBinary).To(Equal("/usr/local/bin/ollama"), "absolute so a mise ollama on PATH is not mistaken for the system install")
		Expect(resolved.DownloadURL).To(BeEmpty())
		Expect(resolved.Dependencies).To(Equal([]string{"curl", "zstd"}))
		Expect(resolved.PostInstall).To(HaveLen(1))
		Expect(resolved.PostInstall[0].Scope).To(Equal("system"))
		Expect(resolved.PostInstall[0].Shell).NotTo(ContainSubstring("install.sh"))
		Expect(resolved.UninstallCommand).To(And(
			ContainSubstring("systemctl stop ollama"),
			ContainSubstring("systemctl disable ollama"),
			ContainSubstring("/etc/systemd/system/ollama.service"),
			ContainSubstring("/usr/local/bin/ollama"),
			ContainSubstring("/usr/local/lib/ollama"),
			ContainSubstring("userdel ollama"),
			ContainSubstring("groupdel ollama"),
			ContainSubstring("/usr/share/ollama"),
		))
		Expect(app.MacOS.InstallMethod).To(Equal("brew"))
		Expect(app.MacOS.InstallCommand).To(Equal("ollama-app"))
		Expect(app.MacOS.BrewCask).To(BeTrue())
		Expect(app.Windows.InstallMethod).To(Equal("winget"))
		Expect(app.Windows.InstallCommand).To(Equal("Ollama.Ollama"))

		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"Ollama"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.DependencyOrder).To(ContainElements("curl", "zstd", "Ollama"))

		var out bytes.Buffer
		cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"install", "--dry-run", "Ollama"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Would install Ollama"))
	})

	It("resolves LM Studio AppImage alternative on non-Debian Linux", func() {
		app, ok := agents["LM Studio"]
		Expect(ok).To(BeTrue(), "expected LM Studio in ai-agents catalog")

		fedora := platform.DetectionResult{
			OS:           "linux",
			Distribution: "fedora",
			Version:      "41",
			Architecture: "amd64",
		}
		resolved := app.LinuxConfigForPlatform(fedora)
		Expect(resolved.InstallMethod).To(Equal("appimage"))
		Expect(resolved.DownloadURL).To(Equal("https://lmstudio.ai/download/latest/linux/x64"))
		Expect(resolved.InstalledBinary).To(Equal("LM-Studio.AppImage"))
		Expect(app.MacOS.InstallMethod).To(Equal("brew"))
		Expect(app.MacOS.InstallCommand).To(Equal("lm-studio"))
		Expect(app.MacOS.BrewCask).To(BeTrue())
		Expect(app.Windows.InstallMethod).To(Equal("winget"))
		Expect(app.Windows.InstallCommand).To(Equal("ElementLabs.LMStudio"))
	})
})
