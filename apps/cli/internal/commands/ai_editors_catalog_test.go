package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

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

var hexFingerprint40 = regexp.MustCompile(`^[0-9A-Fa-f]{40}$`)

var _ = Describe("AI Code Editors catalog", func() {
	var (
		homeDir  string
		settings config.CrossPlatformSettings
		editors  map[string]types.CrossPlatformApp
		files    []string
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogArchForTesting("amd64"))
		DeferCleanup(platform.SetGraphicalDesktopForTesting(true))

		homeDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), homeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")

		settings = config.CrossPlatformSettings{HomeDir: homeDir}
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())

		loaded, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		settings = loaded

		editors = map[string]types.CrossPlatformApp{}
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
			if app.Category != "AI Code Editors" {
				return nil
			}
			editors[app.Name] = app
			files = append(files, filepath.Base(path))
			return nil
		})).To(Succeed())
		sort.Strings(files)
	})

	It("ships and strictly validates every AI code editor definition", func() {
		Expect(files).To(Equal([]string{
			"antigravity.yaml",
			"cursor.yaml",
			"kiro.yaml",
			"windsurf.yaml",
			"zed.yaml",
		}))

		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "diagnostics: %+v", result.Diagnostics)
		Expect(result.Warnings).To(BeZero())

		for name, app := range editors {
			Expect(app.Default).To(BeFalse(), "%s must opt in with default: false", name)
			Expect(app.Category).To(Equal("AI Code Editors"), name)
			Expect(app.DesktopEnvironments).To(Equal([]string{types.DesktopEnvironmentGraphical}), name)
			Expect(app.RequiresGraphicalDesktop()).To(BeTrue(), name)
			Expect(app.Linux.InstallMethod).NotTo(BeEmpty(), "%s must define a Linux install method", name)
			Expect(app.Linux.OfficialSupport).To(BeTrue(), "%s Linux primary must have official_support: true", name)
			Expect(app.Linux.CleanupFiles).To(BeEmpty(), "%s must not list user data in cleanup_files", name)
			Expect(app.Linux.ConfigFiles).To(BeEmpty(), "%s must not list user config in config_files", name)
			Expect(app.MacOS).To(Equal(types.OSConfig{}), "%s must not declare an unverified macOS section", name)
			Expect(app.Windows).To(Equal(types.OSConfig{}), "%s must not declare an unverified Windows section", name)

			switch app.Linux.InstallMethod {
			case "apt":
				Expect(app.Linux.AptSources).NotTo(BeEmpty(), "%s apt editor must declare apt_sources", name)
				for _, src := range app.Linux.AptSources {
					Expect(src.KeySource).To(HavePrefix("https://"), name)
					Expect(hexFingerprint40.MatchString(src.KeyFingerprint)).To(BeTrue(), "%s fingerprint %q must be 40 hex chars", name, src.KeyFingerprint)
					Expect(src.RequireDearmor).To(BeTrue(), "%s armored key must set require_dearmor: true", name)
					Expect(src.SourceRepo).To(ContainSubstring("signed-by="+src.KeyName), "%s source_repo must reference key_name", name)
					Expect(src.SourceName).To(HavePrefix("/etc/apt/sources.list.d/"), name)
					Expect(src.KeyName).To(HavePrefix("/etc/apt/keyrings/"), name)
				}
			case "curlpipe":
				Expect(app.Linux.InstalledBinary).NotTo(BeEmpty(), name)
				Expect(app.Linux.UninstallCommand).NotTo(BeEmpty(), name)
				if app.Linux.IsHookDrivenCurlpipe() {
					// A hook-driven install must pin and verify what it downloads.
					Expect(app.Linux.PostInstall).NotTo(BeEmpty(), name)
					Expect(app.Linux.PostInstall[0].Shell).To(ContainSubstring("sha256sum -c"), name)
				} else {
					Expect(app.Linux.InstallScope).To(Equal("user"), name)
					Expect(app.Linux.DownloadURL).To(HavePrefix("https://"), name)
				}
			case "deb":
				Expect(app.Linux.DownloadURL).To(HavePrefix("https://"), name)
				Expect(app.Linux.Verification.SHA256).To(MatchRegexp(`^[0-9a-f]{64}$`), name)
				for _, alt := range app.Linux.Alternatives {
					Expect(alt.Verification.SHA256).To(MatchRegexp(`^[0-9a-f]{64}$`), name)
				}
			}
		}

		// Selecting AI Coding Agents must not pull in GUI editors.
		agentApps, err := agentapi.ResolveAppsByCategory(settings, []string{"AI Coding Agents"})
		Expect(err).NotTo(HaveOccurred())
		for _, app := range agentApps {
			Expect(editors).NotTo(HaveKey(app.Name))
		}
	})

	DescribeTable("resolves each AI code editor on Ubuntu 24.04 amd64 and skips on headless hosts",
		func(name, wantMethod, wantTarget, wantBinary, wantScope string, wantSelfUpdating bool, wantUninstall string) {
			app, ok := editors[name]
			Expect(ok).To(BeTrue(), "expected %s in AI Code Editors catalog", name)

			ubuntu := platform.DetectionResult{
				OS:               "linux",
				Distribution:     "ubuntu",
				DistributionLike: []string{"debian"},
				Version:          "24.04",
				Architecture:     "amd64",
			}
			resolved := app.LinuxConfigForPlatform(ubuntu)
			Expect(resolved.InstallMethod).To(Equal(wantMethod))
			Expect(resolved.InstalledBinary).To(Equal(wantBinary))
			Expect(resolved.InstallScope).To(Equal(wantScope))
			Expect(resolved.SelfUpdating).To(Equal(wantSelfUpdating))
			if wantUninstall == "" {
				Expect(resolved.UninstallCommand).NotTo(BeEmpty())
			} else {
				Expect(resolved.UninstallCommand).To(Equal(wantUninstall))
			}

			switch wantMethod {
			case "apt":
				Expect(resolved.InstallCommand).To(Equal(wantTarget))
			case "curlpipe", "deb":
				Expect(resolved.DownloadURL).To(Equal(wantTarget))
			}

			// Graphical desktop: planned and dry-run prints "Would install <name>".
			plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.DependencyOrder).To(ContainElement(name))
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

			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"install", "--dry-run", name})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Would install " + name))

			// Headless host: skipped cleanly with exit 0, clear message, and no "Would install".
			restoreHeadless := platform.SetGraphicalDesktopForTesting(false)
			defer restoreHeadless()

			headlessPlan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(headlessPlan.Apps).To(BeEmpty())
			Expect(headlessPlan.DependencyOrder).To(BeEmpty())
			Expect(headlessPlan.Warnings).To(ContainElement(agentapi.HeadlessSkipMessage(name)))

			var headlessOut bytes.Buffer
			headlessCmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			headlessCmd.SetOut(&headlessOut)
			headlessCmd.SetErr(&headlessOut)
			headlessCmd.SetArgs([]string{"install", "--dry-run", name})
			Expect(headlessCmd.Execute()).To(Succeed())
			Expect(headlessOut.String()).To(ContainSubstring(agentapi.HeadlessSkipMessage(name)))
			Expect(headlessOut.String()).NotTo(ContainSubstring("Would install"))
		},
		Entry("Cursor", "Cursor", "apt", "cursor", "cursor", "", false, "cursor"),
		Entry("Windsurf", "Windsurf", "apt", "devin-desktop", "devin-desktop", "", false, "devin-desktop"),
		Entry("Zed", "Zed", "curlpipe", "https://zed.dev/install.sh", "zed", "user", true, `rm -rf "$HOME/.local/zed.app" && rm -f "$HOME/.local/bin/zed" "$HOME/.local/share/applications/dev.zed.Zed.desktop"`),
		Entry("Kiro", "Kiro", "deb", "https://prod.download.desktop.kiro.dev/releases/stable/linux-x64/signed/1.4.154/deb/kiro-ide-1.4.154-stable-linux-x64.deb", "/usr/bin/kiro", "", true, "kiro"),
		// Antigravity's uninstall behaviour is covered in ai_editors_hooks_test.go.
		Entry("Antigravity", "Antigravity", "curlpipe", "", "antigravity-ide", "", false, ""),
	)

	It("preseeds cursor/add-cursor-repo=false in Cursor's resolved Ubuntu pre_install hook", func() {
		cursor, ok := editors["Cursor"]
		Expect(ok).To(BeTrue())
		ubuntu := platform.DetectionResult{
			OS:               "linux",
			Distribution:     "ubuntu",
			DistributionLike: []string{"debian"},
			Version:          "24.04",
			Architecture:     "amd64",
		}
		resolved := cursor.LinuxConfigForPlatform(ubuntu)
		Expect(resolved.PreInstall).NotTo(BeEmpty())
		found := false
		for _, hook := range resolved.PreInstall {
			if hook.Scope == "system" &&
				strings.Contains(hook.Shell, "cursor cursor/add-cursor-repo boolean false") &&
				strings.Contains(hook.Shell, "debconf-set-selections") {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "expected Cursor pre_install system hook to preseed cursor/add-cursor-repo=false, got: %+v", resolved.PreInstall)
	})

	It("emits deterministic JSON plan warnings and digest when editors are skipped on headless hosts", func() {
		DeferCleanup(platform.SetGraphicalDesktopForTesting(false))

		snap1, err := agentapi.ResolveExecution(context.Background(), settings, agentapi.PlanRequest{Names: []string{"Cursor", "Zed", "Windsurf"}}, agentapi.ExecutionPolicy{InstallConcurrency: 1}, agentapi.ExecutionTrust{})
		Expect(err).NotTo(HaveOccurred())
		snap2, err := agentapi.ResolveExecution(context.Background(), settings, agentapi.PlanRequest{Names: []string{"Cursor", "Zed", "Windsurf"}}, agentapi.ExecutionPolicy{InstallConcurrency: 1}, agentapi.ExecutionTrust{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snap1.Digest).To(Equal(snap2.Digest))
		Expect(snap1.Plan.Apps).To(BeEmpty())
		Expect(snap1.Plan.Warnings).To(Equal([]string{
			agentapi.HeadlessSkipMessage("Cursor"),
			agentapi.HeadlessSkipMessage("Zed"),
			agentapi.HeadlessSkipMessage("Windsurf"),
		}))

		runJSON := func() struct {
			Apps     []agentapi.PlannedApp `json:"apps"`
			Warnings []string              `json:"warnings"`
			Roots    []string              `json:"roots"`
			Digest   string                `json:"digest"`
		} {
			var jsonOut bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&jsonOut)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"install", "--dry-run", "--format", "json", "cursor", "zed", "windsurf"})
			Expect(cmd.Execute()).To(Succeed())

			var decoded struct {
				Apps     []agentapi.PlannedApp `json:"apps"`
				Warnings []string              `json:"warnings"`
				Roots    []string              `json:"roots"`
				Digest   string                `json:"digest"`
			}
			Expect(json.Unmarshal(jsonOut.Bytes(), &decoded)).To(Succeed())
			return decoded
		}

		first := runJSON()
		second := runJSON()
		Expect(first.Apps).To(BeEmpty())
		Expect(first.Roots).To(BeEmpty())
		Expect(first.Warnings).To(Equal(snap1.Plan.Warnings))
		Expect(first.Digest).To(MatchRegexp(`^sha256:[0-9a-f]{64}$`))
		Expect(first.Digest).To(Equal(second.Digest))
	})

	It("exits 0 with skip messages on a non-dry-run install when all requested editors require a graphical desktop on a headless host", func() {
		DeferCleanup(platform.SetGraphicalDesktopForTesting(false))
		GinkgoT().Setenv("DEVEX_NONINTERACTIVE", "1")

		var out bytes.Buffer
		cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"install", "cursor", "zed", "windsurf"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(out.String()).To(ContainSubstring(agentapi.HeadlessSkipMessage("Cursor")))
		Expect(out.String()).To(ContainSubstring(agentapi.HeadlessSkipMessage("Zed")))
		Expect(out.String()).To(ContainSubstring(agentapi.HeadlessSkipMessage("Windsurf")))
	})

	It("reports Windsurf as unsupported on linux/arm64 with a clear message", func() {
		DeferCleanup(types.PinCatalogArchForTesting("arm64"))

		windsurf, ok := editors["Windsurf"]
		Expect(ok).To(BeTrue())
		Expect(windsurf.IsSupported()).To(BeFalse())
		Expect(windsurf.Linux.IsArchitectureSupported("arm64")).To(BeFalse())
		Expect(windsurf.Linux.IsArchitectureSupported("amd64")).To(BeTrue())

		_, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"Windsurf"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Windsurf is not supported on linux/arm64"))
		Expect(err.Error()).To(ContainSubstring("amd64"))

		var out bytes.Buffer
		cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"install", "--dry-run", "windsurf"})
		err = cmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Windsurf is not supported on linux/arm64"))
	})
	It("supports Cursor only on the architectures its apt repo serves", func() {
		cursor := editors["Cursor"]
		Expect(cursor.Linux.IsArchitectureSupported("amd64")).To(BeTrue())
		Expect(cursor.Linux.IsArchitectureSupported("arm64")).To(BeTrue())
		Expect(cursor.Linux.IsArchitectureSupported("arm")).To(BeFalse())
	})
})
