package commands_test

import (
	"bytes"
	"context"
	"io/fs"
	"path/filepath"
	"sort"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var _ = Describe("AI Agent Connectors catalog", func() {
	var (
		settings   config.CrossPlatformSettings
		connectors map[string]types.CrossPlatformApp
		files      []string
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogArchForTesting("amd64"))

		homeDir := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), homeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")
		settings = config.CrossPlatformSettings{HomeDir: homeDir}
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())
		loaded, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		settings = loaded

		connectors = map[string]types.CrossPlatformApp{}
		files = nil
		Expect(fs.WalkDir(shipconfig.Files, "applications/ai-agent-connectors", func(path string, entry fs.DirEntry, walkErr error) error {
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
			connectors[app.Name] = app
			files = append(files, filepath.Base(path))
			return nil
		})).To(Succeed())
		sort.Strings(files)
	})

	It("ships and strictly validates every connector as an opt-in AI Agent Connectors entry", func() {
		Expect(files).To(Equal([]string{
			"agoragentic.yaml",
			"amp-acp.yaml",
			"claude-agent-acp.yaml",
			"codex-acp.yaml",
			"deepagents-acp.yaml",
			"glm-acp-agent.yaml",
			"minimax-code-acp.yaml",
			"pi-acp.yaml",
		}))

		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "diagnostics: %+v", result.Diagnostics)
		Expect(result.Warnings).To(BeZero())

		for name, app := range connectors {
			Expect(app.Default).To(BeFalse(), name)
			Expect(app.Category).To(Equal("AI Agent Connectors"), name)
			Expect(app.Linux.OfficialSupport).To(BeTrue(), name)
			Expect(app.Linux.InstalledBinary).NotTo(BeEmpty(), name)
			Expect(app.Linux.ConfigFiles).To(BeEmpty(), name)
			Expect(app.Linux.CleanupFiles).To(BeEmpty(), name)
		}
	})

	DescribeTable("installs each connector together with the agent it wraps on Ubuntu 24.04",
		func(name, wantMethod, wantTarget, wantUninstall string, wantDeps []string) {
			app, ok := connectors[name]
			Expect(ok).To(BeTrue(), name)
			cfg := app.LinuxConfigForPlatform(platform.DetectionResult{
				OS: "linux", Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "amd64",
			})
			Expect(cfg.InstallMethod).To(Equal(wantMethod))
			if wantMethod == "mise" {
				Expect(cfg.InstallCommand).To(Equal(wantTarget))
			} else {
				Expect(cfg.DownloadURL).To(Equal(wantTarget))
			}
			Expect(cfg.UninstallCommand).To(Equal(wantUninstall))
			Expect(cfg.Dependencies).To(Equal(wantDeps))
			for _, dep := range wantDeps {
				_, err := settings.GetApplicationByName(dep)
				Expect(err).NotTo(HaveOccurred(), "%s depends on %q, which must be a catalog app", name, dep)
			}

			plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.DependencyOrder).To(ContainElement(name))
			for _, dep := range wantDeps {
				Expect(plan.DependencyOrder).To(ContainElement(dep))
			}

			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"install", "--dry-run", name})
			Expect(cmd.Execute()).To(Succeed(), out.String())
			Expect(out.String()).To(ContainSubstring("Would install " + name))
		},
		Entry("GLM Agent", "GLM Agent", "mise", "node@24 npm:glm-acp-agent@latest", "npm:glm-acp-agent", []string{"Node.js"}),
		Entry("Agoragentic", "Agoragentic", "mise", "node@24 npm:agoragentic-mcp@latest", "npm:agoragentic-mcp", []string{"Node.js"}),
		Entry("pi-acp", "pi-acp", "mise", "node@24 npm:pi-acp@latest", "npm:pi-acp", []string{"Node.js", "Pi"}),
		Entry("Claude Agent ACP", "Claude Agent ACP", "mise", "node@24 npm:@agentclientprotocol/claude-agent-acp@latest", "npm:@agentclientprotocol/claude-agent-acp", []string{"Node.js", "Claude Code"}),
		Entry("Codex ACP", "Codex ACP", "mise", "node@24 npm:@agentclientprotocol/codex-acp@latest", "npm:@agentclientprotocol/codex-acp", []string{"Node.js", "Codex CLI"}),
		Entry("MiniMax Code ACP", "MiniMax Code ACP", "mise", "node@24 npm:@minimax-ai/code-acp@latest", "npm:@minimax-ai/code-acp", []string{"Node.js", "MiniMax Code"}),
		Entry("DeepAgents ACP", "DeepAgents ACP", "mise", "node@24 npm:deepagents-acp@latest", "npm:deepagents-acp", []string{"Node.js"}),
		Entry("amp-acp", "amp-acp", "curlpipe", "https://github.com/tao12345666333/amp-acp/releases/download/v0.13.0/amp-acp-linux-x86_64.tar.gz", "set -eu\nSUDO=\"\"\nif [ \"$(id -u)\" -ne 0 ]; then SUDO=\"sudo\"; fi\n$SUDO rm -f /usr/local/bin/amp-acp\n", []string{"Amp"}),
	)

	It("pins amp-acp's release archive for both Linux architectures", func() {
		app := connectors["amp-acp"]
		Expect(app.Linux.Verification.SHA256).To(Equal("205c819383b6fd389b408120ebea9cdde5c2957530b5ce4ebc912998d4118f8b"))
		Expect(app.Linux.Alternatives).To(HaveLen(1))
		Expect(app.Linux.Alternatives[0].DownloadURL).To(HaveSuffix("amp-acp-linux-aarch64.tar.gz"))
		Expect(app.Linux.Alternatives[0].Verification.SHA256).To(Equal("ad7e1c23ec29bf47f757d656fc4c9a957dd60533da034213a6bc7d33d1813d32"))
	})

	DescribeTable("plans connector upgrades and uninstalls without touching the wrapped agent or config",
		func(name, wantUpgrade, wantUninstall string) {
			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"upgrade", "--dry-run", "--non-interactive", name})
			Expect(cmd.Execute()).To(Succeed(), out.String())
			Expect(out.String()).To(ContainSubstring(wantUpgrade))

			repo := mocks.NewMockRepository()
			Expect(repo.AddApp(name)).To(Succeed())
			out.Reset()
			cmd = commands.NewRootCmd("test", repo, settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"uninstall", "--app", name, "--dry-run"})
			Expect(cmd.Execute()).To(Succeed(), out.String())
			Expect(out.String()).To(ContainSubstring("Would uninstall " + name + " via " + wantUninstall))
			Expect(out.String()).NotTo(ContainSubstring("Would remove configuration"))
		},
		Entry("GLM Agent", "GLM Agent", `devex-plugin-package-manager-mise "update" "npm:glm-acp-agent"`, "mise: npm:glm-acp-agent"),
		Entry("pi-acp", "pi-acp", `devex-plugin-package-manager-mise "update" "npm:pi-acp"`, "mise: npm:pi-acp"),
		Entry("Claude Agent ACP", "Claude Agent ACP", `devex-plugin-package-manager-mise "update" "npm:@agentclientprotocol/claude-agent-acp"`, "mise: npm:@agentclientprotocol/claude-agent-acp"),
		Entry("Codex ACP", "Codex ACP", `devex-plugin-package-manager-mise "update" "npm:@agentclientprotocol/codex-acp"`, "mise: npm:@agentclientprotocol/codex-acp"),
	)
})
