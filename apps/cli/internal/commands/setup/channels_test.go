package setup

import (
	"context"
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func channelSetupSettings(homeDir string) config.CrossPlatformSettings {
	return config.CrossPlatformSettings{
		HomeDir: homeDir,
		ApplicationOverrides: []types.CrossPlatformApp{
			{
				Name:        "mise",
				Description: "Polyglot runtime manager",
				Category:    "Dependency",
				Default:     false,
				Linux: types.OSConfig{
					InstallMethod:   "curlpipe",
					DownloadURL:     "https://mise.run",
					InstallScope:    "user",
					OfficialSupport: true,
				},
			},
			{
				Name:        "Kimi Code",
				Description: "Moonshot AI coding agent CLI",
				Category:    CategoryAICodingAgents,
				Default:     false,
				Linux: types.OSConfig{
					InstallMethod:   "curlpipe",
					DownloadURL:     "https://code.kimi.com/kimi-code/install.sh",
					InstallScope:    "user",
					OfficialSupport: true,
					Alternatives: []types.OSConfig{
						{
							InstallMethod:    "mise",
							InstallCommand:   "node@lts npm:@moonshot-ai/kimi-code@latest",
							UninstallCommand: "npm:@moonshot-ai/kimi-code",
							OfficialSupport:  true,
							Dependencies:     []string{"mise"},
						},
					},
				},
			},
			{
				Name:                "Ghostty",
				Description:         "GPU-accelerated terminal",
				Category:            "Optional Apps",
				Default:             false,
				DesktopEnvironments: []string{"gnome"},
				Linux: types.OSConfig{
					InstallMethod:    "apt",
					InstallCommand:   "ghostty",
					UninstallCommand: "ghostty",
					OfficialSupport:  true,
					Alternatives: []types.OSConfig{
						{
							InstallMethod:    "snap",
							InstallCommand:   "ghostty --classic",
							UninstallCommand: "ghostty",
							OfficialSupport:  false,
						},
					},
				},
			},
		},
	}
}

var _ = Describe("Setup TUI Install Channels", func() {
	var (
		homeDir  string
		settings config.CrossPlatformSettings
		repo     *mocks.MockRepository
		plat     platform.DetectionResult
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		plat = platform.DetectionResult{OS: "linux", Distribution: "ubuntu", Version: "26.04", DesktopEnv: "gnome"}
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogPlatformForTesting(plat))
		GinkgoT().Setenv("SHELL", "/bin/bash")

		homeDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), homeDir)
		settings = channelSetupSettings(homeDir)
		repo = mocks.NewMockRepository()
	})

	It("shows channel and official marker on AI agents and switches channels with 'c'", func() {
		model := NewSetupModel(repo, settings, plat)
		model.step = StepAIAgents
		model.cursor = 0

		view := model.View()
		Expect(view).To(ContainSubstring("Kimi Code"))
		Expect(view).To(ContainSubstring("[curlpipe (official) | alt: mise (official)]"))
		Expect(view).To(ContainSubstring("'c' to switch channel"))

		// Select Kimi Code, verify default plan uses curlpipe without mise prerequisite.
		model.selections.selectedAIAgents["Kimi Code"] = true
		planDefault := model.installPlan()
		var kimiDefault *types.CrossPlatformApp
		hasMiseDefault := false
		for i := range planDefault {
			if planDefault[i].app.Name == "Kimi Code" {
				appCopy := planDefault[i].app
				kimiDefault = &appCopy
			}
			if planDefault[i].app.Name == "mise" {
				hasMiseDefault = true
			}
		}
		Expect(kimiDefault).NotTo(BeNil())
		Expect(kimiDefault.SelectedChannel()).To(Equal("curlpipe"))
		Expect(hasMiseDefault).To(BeFalse())

		// Press 'c' to switch highlighted Kimi Code to mise.
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
		model = updated.(*SetupModel)

		Expect(model.selections.selectedChannels["Kimi Code"]).To(Equal("mise"))
		Expect(model.View()).To(ContainSubstring("[mise (official) | alt: curlpipe (official)]"))

		// Plan now uses mise channel and pulls in mise dependency automatically.
		planMise := model.installPlan()
		var kimiMise *types.CrossPlatformApp
		hasMiseSwitched := false
		for i := range planMise {
			if planMise[i].app.Name == "Kimi Code" {
				appCopy := planMise[i].app
				kimiMise = &appCopy
			}
			if planMise[i].app.Name == "mise" {
				hasMiseSwitched = true
			}
		}
		Expect(kimiMise).NotTo(BeNil())
		Expect(kimiMise.SelectedChannel()).To(Equal("mise"))
		Expect(kimiMise.GetOSConfig().InstallMethod).To(Equal("mise"))
		Expect(hasMiseSwitched).To(BeTrue())

		// Confirmation view reflects the selected channel.
		model.step = StepConfirmation
		confirmView := model.View()
		Expect(confirmView).To(ContainSubstring("Kimi Code"))
		Expect(confirmView).To(ContainSubstring("mise (official)"))
	})

	It("shows channel and official marker on desktop apps and cycles channels with 'c'", func() {
		model := NewSetupModel(repo, settings, plat)
		model.step = StepDesktopApps
		model.cursor = 0

		view := model.View()
		Expect(view).To(ContainSubstring("Ghostty"))
		Expect(view).To(ContainSubstring("[apt (official) | alt: snap]"))
		Expect(view).To(ContainSubstring("'c' to switch channel"))

		model.selections.selectedDesktopApps["Ghostty"] = true
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
		model = updated.(*SetupModel)

		Expect(model.selections.selectedChannels["Ghostty"]).To(Equal("snap"))
		Expect(model.View()).To(ContainSubstring("[snap | alt: apt (official)]"))

		plan := model.installPlan()
		var ghostty *types.CrossPlatformApp
		for i := range plan {
			if plan[i].app.Name == "Ghostty" {
				appCopy := plan[i].app
				ghostty = &appCopy
			}
		}
		Expect(ghostty).NotTo(BeNil())
		Expect(ghostty.SelectedChannel()).To(Equal("snap"))
		Expect(ghostty.GetOSConfig().InstallMethod).To(Equal("snap"))
	})

	It("refuses channel switches in interactive setup before installed-state skip checks and before installing (S2)", func() {
		aptGhostty, err := settings.ApplicationOverrides[2].WithChannel("apt")
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.AddApp(aptGhostty.Name)).To(Succeed())
		Expect(inventory.Record(context.Background(), repo, aptGhostty, inventory.RecordOptions{
			Query: func(context.Context, string, string) (bool, string, error) { return true, "1.0.0", nil },
		})).To(Succeed())

		oldCheck := installedAppsCheck
		DeferCleanup(func() { installedAppsCheck = oldCheck })
		installedAppsCheck = func(_ context.Context, apps []types.CrossPlatformApp) map[string]bool {
			res := map[string]bool{}
			for _, app := range apps {
				res[app.Name] = true
			}
			return res
		}

		model := NewSetupModel(repo, settings, plat)
		model.selections.skipDefaultApps = true
		model.selections.selectedDesktopApps["Ghostty"] = true
		model.selections.selectedChannels["Ghostty"] = "snap"
		model.step = StepConfirmation
		model.refreshInstalledApps()

		expectedMsg := `Ghostty is installed through channel "apt"; run 'devex uninstall --app "Ghostty"' first, then install it with --channel snap`

		// Error is shown in the plan and is NOT masked by "already installed, will skip".
		confirmView := model.View()
		Expect(confirmView).To(ContainSubstring(expectedMsg))
		Expect(confirmView).NotTo(ContainSubstring("already installed, will skip"))

		// Pressing Enter on Confirmation refuses to start installation.
		updated, cmd := model.handleEnter()
		Expect(updated.step).To(Equal(StepConfirmation))
		Expect(cmd).To(BeNil())

		// Direct installSelectedApps call records the refusal error without installing.
		model.installSelectedApps()
		Expect(model.HasErrors()).To(BeTrue())
		Expect(model.installation.getErrors()).To(ContainElement(ContainSubstring(expectedMsg)))
	})

	It("refuses channel switches in automated setup before installing anything (S2)", func() {
		curlKimi, err := settings.ApplicationOverrides[1].WithChannel("curlpipe")
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.AddApp(curlKimi.Name)).To(Succeed())
		Expect(inventory.Record(context.Background(), repo, curlKimi, inventory.RecordOptions{
			Query: func(context.Context, string, string) (bool, string, error) { return true, "1.0.0", nil },
		})).To(Succeed())

		oldInstall := installAutomatedApp
		oldCheck := installedAppsCheck
		DeferCleanup(func() {
			installAutomatedApp = oldInstall
			installedAppsCheck = oldCheck
		})
		installed := false
		installAutomatedApp = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			installed = true
			return nil
		}
		installedAppsCheck = func(context.Context, []types.CrossPlatformApp) map[string]bool {
			return map[string]bool{"Kimi Code": true}
		}

		viper.Set("no-defaults", true)
		viper.Set("setup.ai_agents.apps", []string{"Kimi Code"})
		viper.Set("channel", "mise")

		err = RunAutomatedSetup(context.Background(), repo, settings)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`Kimi Code is installed through channel "curlpipe"; run 'devex uninstall --app "Kimi Code"' first, then install it with --channel mise`))
		Expect(installed).To(BeFalse())
	})

	It("surfaces unknown --channel and invalid per-app channel errors with offered channels before installing (S7)", func() {
		viper.Set("no-defaults", true)
		viper.Set("setup.ai_agents.apps", []string{"Kimi Code"})
		viper.Set("channel", "bogus")

		err := RunAutomatedSetup(context.Background(), repo, settings)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`Kimi Code has no channel "bogus" on linux`))
		Expect(err.Error()).To(ContainSubstring(`curlpipe (official, default)`))
		Expect(err.Error()).To(ContainSubstring(`mise (official)`))

		// Invalid per-app channels: preference is also surfaced with offered channels.
		viper.Set("channel", "")
		viper.Set("channels", map[string]any{"kimi-code": "bogus"})
		err = RunAutomatedSetup(context.Background(), repo, settings)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`Kimi Code has no channel "bogus" on linux`))
		Expect(err.Error()).To(ContainSubstring(`available channels:`))

		// Interactive confirmation view also surfaces the selection error and blocks Enter.
		model := NewSetupModel(repo, settings, plat)
		model.selections.skipDefaultApps = true
		model.selections.selectedAIAgents["Kimi Code"] = true
		model.step = StepConfirmation
		Expect(model.View()).To(ContainSubstring(`Kimi Code has no channel "bogus" on linux`))
		updated, cmd := model.handleEnter()
		Expect(updated.step).To(Equal(StepConfirmation))
		Expect(cmd).To(BeNil())
	})

	It("registers apps and records inventory receipts with the selected channel after automated installs (S8)", func() {
		oldBatch := preparePackageBatch
		oldInstall := installAutomatedApp
		DeferCleanup(func() {
			preparePackageBatch = oldBatch
			installAutomatedApp = oldInstall
		})
		preparePackageBatch = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) (tui.PackageBatch, error) {
			return tui.PackageBatch{}, nil
		}
		installAutomatedApp = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			return nil
		}

		miseKimi, err := settings.ApplicationOverrides[1].WithChannel("mise")
		Expect(err).NotTo(HaveOccurred())

		outcomes := tui.NewOutcomeRecorder()
		failures, err := installAutomatedApps(context.Background(), []types.CrossPlatformApp{miseKimi}, nil, outcomes, settings, repo)
		Expect(err).NotTo(HaveOccurred())
		Expect(failures).To(BeEmpty())

		tracked, err := repo.GetApp("Kimi Code")
		Expect(err).NotTo(HaveOccurred())
		Expect(tracked).NotTo(BeNil())

		def, err := repo.Get("installed-app-definition:kimi code")
		Expect(err).NotTo(HaveOccurred())
		var saved types.CrossPlatformApp
		Expect(json.Unmarshal([]byte(def), &saved)).To(Succeed())
		Expect(saved.SelectedChannel()).To(Equal("mise"))
		Expect(saved.GetOSConfig().InstallMethod).To(Equal("mise"))

		receipt, ok := inventory.ReadReceipt(repo, "Kimi Code")
		Expect(ok).To(BeTrue())
		Expect(receipt.Channel).To(Equal("mise"))
		Expect(receipt.Manager).To(Equal("mise"))
	})
})
