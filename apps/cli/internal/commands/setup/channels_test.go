package setup

import (
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
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
})
