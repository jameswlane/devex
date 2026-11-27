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

var hexSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

var _ = Describe("AI Agent Harnesses catalog", func() {
	var (
		homeDir   string
		settings  config.CrossPlatformSettings
		harnesses map[string]types.CrossPlatformApp
		files     []string
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

		harnesses = map[string]types.CrossPlatformApp{}
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
			if app.Category != "AI Agent Harnesses" {
				return nil
			}
			harnesses[app.Name] = app
			files = append(files, filepath.Base(path))
			return nil
		})).To(Succeed())
		sort.Strings(files)
	})

	It("ships and strictly validates every AI agent harness definition", func() {
		Expect(files).To(Equal([]string{
			"amux.yaml",
			"claude-squad.yaml",
			"cmux.yaml",
			"conductor.yaml",
			"herdr.yaml",
			"openchamber.yaml",
			"orca.yaml",
			"paseo.yaml",
			"superset.yaml",
			"t3-code.yaml",
		}))

		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "diagnostics: %+v", result.Diagnostics)
		Expect(result.Warnings).To(BeZero())

		for name, app := range harnesses {
			Expect(app.Default).To(BeFalse(), "%s must opt in with default: false", name)
			Expect(app.Category).To(Equal("AI Agent Harnesses"), name)
			Expect(app.Linux.CleanupFiles).To(BeEmpty(), "%s must not list user data in cleanup_files", name)
			Expect(app.Linux.ConfigFiles).To(BeEmpty(), "%s must not list user config in config_files", name)

			checkCurlpipe := func(label string, cfg types.OSConfig) {
				if cfg.InstallMethod != "curlpipe" {
					return
				}
				if cfg.DownloadURL != "" && installers.IsReleaseArchive(cfg.DownloadURL) {
					Expect(hexSHA256.MatchString(cfg.Verification.SHA256)).To(BeTrue(), "%s (%s) release archive must pin a 64-hex sha256", name, label)
					Expect(cfg.Destination).To(HavePrefix("/"), "%s (%s) release archive must declare an absolute destination", name, label)
				} else if cfg.DownloadURL != "" {
					Expect(cfg.InstallScope).To(Equal("user"), "%s (%s) script curlpipe must use install_scope: user", name, label)
					Expect(cfg.DownloadURL).To(HavePrefix("https://"), "%s (%s) curlpipe must use HTTPS download_url", name, label)
				}
				Expect(cfg.InstalledBinary).NotTo(BeEmpty(), "%s (%s) curlpipe must declare installed_binary", name, label)
				Expect(cfg.UninstallCommand).NotTo(BeEmpty(), "%s (%s) curlpipe must declare uninstall_command", name, label)
			}
			checkCurlpipe("linux", app.Linux)
			for _, alt := range app.Linux.Alternatives {
				checkCurlpipe("linux-alt", alt)
			}
			checkCurlpipe("macos", app.MacOS)
			checkCurlpipe("windows", app.Windows)
		}

		// Selecting AI Coding Agents must not pull in harnesses.
		agentApps, err := agentapi.ResolveAppsByCategory(settings, []string{"AI Coding Agents"})
		Expect(err).NotTo(HaveOccurred())
		for _, app := range agentApps {
			Expect(harnesses).NotTo(HaveKey(app.Name))
		}
	})

	DescribeTable("resolves each Linux-supported AI agent harness on Ubuntu 24.04 amd64",
		func(name, wantMethod, wantScope string, wantGraphical, wantSelfUpdating bool, wantDeps []string) {
			app, ok := harnesses[name]
			Expect(ok).To(BeTrue(), "expected %s in AI Agent Harnesses catalog", name)

			Expect(app.RequiresGraphicalDesktop()).To(Equal(wantGraphical), name)
			if wantGraphical {
				Expect(app.DesktopEnvironments).To(Equal([]string{types.DesktopEnvironmentGraphical}), name)
			} else {
				Expect(app.DesktopEnvironments).To(Equal([]string{"all"}), name)
			}

			ubuntu := platform.DetectionResult{
				OS:               "linux",
				Distribution:     "ubuntu",
				DistributionLike: []string{"debian"},
				Version:          "24.04",
				Architecture:     "amd64",
			}
			resolved := app.LinuxConfigForPlatform(ubuntu)
			Expect(resolved.InstallMethod).To(Equal(wantMethod), name)
			Expect(resolved.InstallScope).To(Equal(wantScope), name)
			Expect(resolved.SelfUpdating).To(Equal(wantSelfUpdating), name)
			Expect(resolved.OfficialSupport).To(BeTrue(), name)

			if wantMethod == "deb" {
				Expect(resolved.DownloadURL).To(HavePrefix("https://"), name)
				Expect(hexSHA256.MatchString(resolved.Verification.SHA256)).To(BeTrue(), "%s .deb must pin verification.sha256", name)
				Expect(resolved.UninstallCommand).To(Equal(resolved.InstallCommand), name)
			}
			if wantMethod == "mise" {
				packages, err := installers.CatalogPackageArguments("mise", resolved.InstallCommand)
				Expect(err).NotTo(HaveOccurred())
				Expect(packages).To(Equal(strings.Fields(resolved.InstallCommand)))
				uninstallArgs, err := installers.CatalogPackageArguments("mise", resolved.UninstallCommand)
				Expect(err).NotTo(HaveOccurred())
				Expect(uninstallArgs).To(Equal([]string{resolved.UninstallCommand}))
			}

			plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.DependencyOrder).To(ContainElement(name))
			for _, dep := range wantDeps {
				Expect(plan.DependencyOrder).To(ContainElement(dep), "%s plan must include dependency %s", name, dep)
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

			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"install", "--dry-run", name})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Would install " + name))

			if wantGraphical {
				restoreHeadless := platform.SetGraphicalDesktopForTesting(false)
				defer restoreHeadless()

				headlessPlan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
				Expect(err).NotTo(HaveOccurred())
				Expect(headlessPlan.Apps).To(BeEmpty())
				Expect(headlessPlan.Warnings).To(ContainElement(agentapi.HeadlessSkipMessage(name)))

				var headlessOut bytes.Buffer
				headlessCmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
				headlessCmd.SetOut(&headlessOut)
				headlessCmd.SetErr(&headlessOut)
				headlessCmd.SetArgs([]string{"install", "--dry-run", name})
				Expect(headlessCmd.Execute()).To(Succeed())
				Expect(headlessOut.String()).To(ContainSubstring(agentapi.HeadlessSkipMessage(name)))
				Expect(headlessOut.String()).NotTo(ContainSubstring("Would install"))
			}
		},
		Entry("Orca", "Orca", "deb", "", true, true, nil),
		Entry("Herdr", "Herdr", "curlpipe", "user", false, true, nil),
		Entry("T3 Code", "T3 Code", "curlpipe", "user", false, true, nil),
		Entry("Claude Squad", "Claude Squad", "curlpipe", "", false, false, []string{"tmux", "git", "GitHub CLI"}),
		Entry("Superset", "Superset", "deb", "", true, true, []string{"git"}),
		Entry("OpenChamber", "OpenChamber", "mise", "", false, false, []string{"Node.js"}),
		Entry("Paseo", "Paseo", "mise", "", false, false, []string{"Node.js"}),
		Entry("amux", "amux", "curlpipe", "user", false, false, []string{"git", "tmux", "Rust"}),
	)

	DescribeTable("skips macOS-only harnesses cleanly on Linux with a clear message",
		func(name string) {
			app, ok := harnesses[name]
			Expect(ok).To(BeTrue(), "expected %s in AI Agent Harnesses catalog", name)
			Expect(app.IsSupported()).To(BeFalse())
			Expect(app.Linux).To(Equal(types.OSConfig{}))
			Expect(app.MacOS.InstallMethod).To(Equal("brew"))
			Expect(app.MacOS.BrewCask).To(BeTrue())
			Expect(app.MacOS.OfficialSupport).To(BeTrue())

			wantSkip := agentapi.UnsupportedOSSkipMessage(name, "linux", []string{"macOS"})

			plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.Apps).To(BeEmpty())
			Expect(plan.DependencyOrder).To(BeEmpty())
			Expect(plan.Warnings).To(ContainElement(wantSkip))

			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"install", "--dry-run", name})
			Expect(cmd.Execute()).To(Succeed())
			Expect(out.String()).To(ContainSubstring(wantSkip))
			Expect(out.String()).NotTo(ContainSubstring("Would install"))

			var jsonOut bytes.Buffer
			jsonCmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			jsonCmd.SetOut(&jsonOut)
			jsonCmd.SetErr(&bytes.Buffer{})
			jsonCmd.SetArgs([]string{"install", "--dry-run", "--format", "json", name})
			Expect(jsonCmd.Execute()).To(Succeed())

			var decoded struct {
				Apps     []agentapi.PlannedApp `json:"apps"`
				Warnings []string              `json:"warnings"`
				Roots    []string              `json:"roots"`
				Digest   string                `json:"digest"`
			}
			Expect(json.Unmarshal(jsonOut.Bytes(), &decoded)).To(Succeed())
			Expect(decoded.Apps).To(BeEmpty())
			Expect(decoded.Roots).To(BeEmpty())
			Expect(decoded.Warnings).To(Equal([]string{wantSkip}))
			Expect(decoded.Digest).To(MatchRegexp(`^sha256:[0-9a-f]{64}$`))

			// On macOS, the same harness is supported.
			restoreDarwin := types.PinCatalogOSForTesting("darwin")
			defer restoreDarwin()
			Expect(app.IsSupported()).To(BeTrue())
		},
		Entry("Conductor", "Conductor"),
		Entry("cmux", "cmux"),
	)

	It("leaves the T3 Code background service to upstream's own service and uninstall commands", func() {
		app := harnesses["T3 Code"]
		Expect(app.Linux.PostInstall).NotTo(BeEmpty())
		for _, hook := range app.Linux.PostInstall {
			Expect(hook.Scope).To(Or(BeEmpty(), Equal("user")))
			Expect(hook.Shell).NotTo(ContainSubstring("systemd/user"), "upstream `t3 service install` owns t3code.service")
			for _, line := range strings.Split(hook.Shell, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "echo ") {
					continue
				}
				Expect(trimmed).NotTo(ContainSubstring("systemctl"), "T3 Code install must not touch systemd")
				Expect(trimmed).NotTo(ContainSubstring("t3 service install"), "T3 Code install must not enable the service")
			}
			Expect(hook.Shell).To(ContainSubstring("t3 service install"), "must print how to enable the service")
		}
		Expect(app.Linux.UninstallCommand).To(ContainSubstring(`"$HOME/.local/bin/t3" uninstall --yes`))
		Expect(app.Linux.UninstallCommand).NotTo(ContainSubstring("rm -rf"), "t3 uninstall keeps ~/.t3/userdata and service.env")
	})

	It("installs a user systemd service for amux bound to localhost without auto-enabling, and cleans it up on uninstall", func() {
		for _, tc := range []struct {
			name        string
			unitName    string
			localhost   string
			keepDataDir string
		}{
			{name: "amux", unitName: "amux-server.service", localhost: "127.0.0.1", keepDataDir: ".amux"},
		} {
			app := harnesses[tc.name]
			Expect(app.Linux.PostInstall).NotTo(BeEmpty(), tc.name)
			shell := app.Linux.PostInstall[0].Shell
			Expect(app.Linux.PostInstall[0].Scope).To(Or(BeEmpty(), Equal("user")), tc.name)
			Expect(shell).To(ContainSubstring(".local/share/systemd/user"), tc.name)
			Expect(shell).To(ContainSubstring(tc.unitName), tc.name)
			Expect(shell).To(ContainSubstring(tc.localhost), tc.name)
			Expect(shell).NotTo(ContainSubstring("/etc/systemd/system"), tc.name)
			// Must NOT execute systemctl enable/start (only print instructions to enable it).
			for _, line := range strings.Split(shell, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "echo ") || strings.HasPrefix(trimmed, "printf ") || strings.HasPrefix(trimmed, "#") {
					continue
				}
				Expect(trimmed).NotTo(ContainSubstring("systemctl --user enable"), "%s must not auto-enable service", tc.name)
				Expect(trimmed).NotTo(ContainSubstring("systemctl --user start"), "%s must not auto-start service", tc.name)
			}
			Expect(shell).To(ContainSubstring("systemctl --user enable --now "+tc.unitName), "%s must print how to enable the service", tc.name)

			uninstall := app.Linux.UninstallCommand
			Expect(uninstall).To(ContainSubstring("systemctl --user stop "+tc.unitName), tc.name)
			Expect(uninstall).To(ContainSubstring("systemctl --user disable "+tc.unitName), tc.name)
			Expect(uninstall).To(ContainSubstring(".local/share/systemd/user/"+tc.unitName), tc.name)
			Expect(uninstall).To(ContainSubstring("daemon-reload"), tc.name)
			Expect(uninstall).NotTo(ContainSubstring("rm -rf \"$HOME/"+tc.keepDataDir+"\""), "%s uninstall must preserve user data", tc.name)
		}
	})

	It("never auto-enables services for Herdr, OpenChamber, or Paseo and documents localhost binding", func() {
		for _, name := range []string{"Herdr", "OpenChamber", "Paseo"} {
			app := harnesses[name]
			for _, hook := range app.Linux.PostInstall {
				Expect(hook.Scope).To(Or(BeEmpty(), Equal("user")), name)
				Expect(hook.Shell).NotTo(ContainSubstring("/etc/systemd/system"), name)
				Expect(hook.Shell).NotTo(ContainSubstring("systemctl --user enable --now"), name)
				Expect(hook.Shell).NotTo(ContainSubstring("systemctl enable"), name)
			}
		}
	})

	It("reports Superset as unsupported on linux/arm64 while Orca supports both amd64 and arm64", func() {
		DeferCleanup(types.PinCatalogArchForTesting("arm64"))

		superset := harnesses["Superset"]
		Expect(superset.IsSupported()).To(BeFalse())
		_, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"Superset"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Superset is not supported on linux/arm64"))

		// IsSupported reads the host distribution; resolve Ubuntu explicitly so
		// macOS and Windows runners check the same selection.
		orca := harnesses["Orca"]
		ubuntuArm64 := platform.DetectionResult{OS: "linux", Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "arm64"}
		resolved := orca.LinuxConfigForPlatform(ubuntuArm64)
		Expect(resolved.InstallMethod).To(Equal("deb"))
		Expect(resolved.DownloadURL).To(ContainSubstring("arm64.deb"))
		Expect(hexSHA256.MatchString(resolved.Verification.SHA256)).To(BeTrue())
	})
})
