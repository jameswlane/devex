package setup

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func desktopAppForTest(name, category string) types.CrossPlatformApp {
	return types.CrossPlatformApp{
		Name:        name,
		Category:    category,
		Description: name + " description",
		Linux: types.OSConfig{
			InstallMethod:  "apt",
			InstallCommand: strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		},
	}
}

func desktopSettingsForTest(apps ...types.CrossPlatformApp) config.CrossPlatformSettings {
	return config.CrossPlatformSettings{ApplicationOverrides: apps}
}

func desktopPlatformForTest() platform.DetectionResult {
	return platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"}
}

var _ = Describe("Desktop application selection", func() {
	It("filters the setup list to compatible optional GUI applications", func() {
		gnomeTweaks := desktopAppForTest("GNOME Tweaks", "Optional Apps")
		gnomeTweaks.DesktopEnvironments = []string{"gnome"}
		dolphin := desktopAppForTest("Dolphin", "Optional Apps")
		dolphin.DesktopEnvironments = []string{"kde"}
		defaultBrowser := desktopAppForTest("Default Browser", "Browsers")
		defaultBrowser.Default = true
		macOnly := desktopAppForTest("Mac Only", "Browsers")
		macOnly.Linux = types.OSConfig{}
		macOnly.MacOS = types.OSConfig{InstallMethod: "brew", InstallCommand: "mac-only"}

		settings := desktopSettingsForTest(
			desktopAppForTest("Ruby", "Programming Languages"),
			desktopAppForTest("PostgreSQL", "Databases"),
			desktopAppForTest("ripgrep", "Utilities"),
			defaultBrowser,
			dolphin,
			macOnly,
			gnomeTweaks,
			desktopAppForTest("Firefox", "Browsers"),
		)
		model := NewSetupModel(nil, settings, desktopPlatformForTest())

		Expect(model.system.desktopApps).To(Equal([]string{"Firefox", "GNOME Tweaks"}))
		Expect(model.system.desktopAppDetails).To(HaveLen(2))
		Expect(model.system.desktopAppDetails[1].Description).To(Equal("GNOME Tweaks description"))
	})

	It("groups categories and sorts apps case-insensitively", func() {
		settings := desktopSettingsForTest(
			desktopAppForTest("zebra", "Utility"),
			desktopAppForTest("apple", "Utility"),
			desktopAppForTest("Firefox", "Browsers"),
		)
		model := NewSetupModel(nil, settings, desktopPlatformForTest())
		model.step = StepDesktopApps
		model.cursor = 0
		model.height = 40

		view := model.View()
		browsers := strings.Index(view, "Browsers")
		firefox := strings.Index(view, "Firefox")
		utility := strings.Index(view, "Utility")
		apple := strings.Index(view, "apple")
		zebra := strings.Index(view, "zebra")

		Expect(browsers).To(BeNumerically(">=", 0))
		Expect(firefox).To(BeNumerically(">", browsers))
		Expect(utility).To(BeNumerically(">", firefox))
		Expect(apple).To(BeNumerically(">", utility))
		Expect(zebra).To(BeNumerically(">", apple))
		Expect(view).To(ContainSubstring("Optional GUI applications for GNOME. Default apps are installed automatically."))
		Expect(view).To(ContainSubstring("Firefox description"))
	})

	It("keeps selections attached to app names when ordering changes", func() {
		settings := desktopSettingsForTest(
			desktopAppForTest("Alpha", "Browsers"),
			desktopAppForTest("Zebra", "Utility"),
		)
		model := NewSetupModel(nil, settings, desktopPlatformForTest())
		model.step = StepDesktopApps
		model.cursor = 1

		model.handleSpace()
		model.system.desktopAppDetails[0].Category = "Zulu"
		model.system.desktopAppDetails[1].Category = "Browsers"

		Expect(model.desktopAppEntries()[0].Name).To(Equal("Zebra"))
		Expect(model.getSelectedDesktopApps()).To(Equal([]string{"Zebra"}))
	})

	It("keeps the header, help, and cursor visible in a 24-row viewport", func() {
		apps := make([]types.CrossPlatformApp, 45)
		for i := range apps {
			apps[i] = desktopAppForTest(fmt.Sprintf("App %02d", i+1), "Optional Apps")
		}
		model := NewSetupModel(nil, desktopSettingsForTest(apps...), desktopPlatformForTest())
		model.step = StepDesktopApps
		model.cursor = len(apps) - 1

		updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		view := updated.(*SetupModel).View()

		Expect(view).To(ContainSubstring("Select Desktop Applications"))
		Expect(view).To(ContainSubstring("Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"))
		Expect(view).To(ContainSubstring("> [ ] App 45"))
		Expect(view).To(ContainSubstring("↑ more"))
		Expect(strings.Split(view, "\n")).To(HaveLen(24))
	})
})
