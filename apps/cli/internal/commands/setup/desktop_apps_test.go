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

	It("honors graphical desktop requirements in setup filtering", func() {
		cursor := desktopAppForTest("Cursor", "AI Code Editors")
		cursor.DesktopEnvironments = []string{types.DesktopEnvironmentGraphical}
		settings := desktopSettingsForTest(cursor)

		// Active GNOME desktop shows the graphical editor.
		Expect(getAvailableDesktopApps(settings, platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"})).To(HaveLen(1))

		// Explicit headless / none hides it.
		Expect(getAvailableDesktopApps(settings, platform.DetectionResult{OS: "linux", DesktopEnv: "none"})).To(BeEmpty())

		// Unknown DE defers to the graphical desktop detector (covers Sway/SSH vs headless).
		restoreHeadless := platform.SetGraphicalDesktopForTesting(false)
		Expect(getAvailableDesktopApps(settings, platform.DetectionResult{OS: "linux", DesktopEnv: "unknown"})).To(BeEmpty())
		restoreHeadless()

		restoreDesktop := platform.SetGraphicalDesktopForTesting(true)
		defer restoreDesktop()
		Expect(getAvailableDesktopApps(settings, platform.DetectionResult{OS: "linux", DesktopEnv: "unknown"})).To(HaveLen(1))
	})

	It("includes curlpipe download_url editors like Zed and filters arch-restricted editors like Windsurf", func() {
		zed := types.CrossPlatformApp{
			Name:                "Zed",
			Category:            "AI Code Editors",
			Description:         "Zed editor",
			DesktopEnvironments: []string{types.DesktopEnvironmentGraphical},
			Linux: types.OSConfig{
				InstallMethod: "curlpipe",
				DownloadURL:   "https://zed.dev/install.sh",
				InstallScope:  "user",
			},
		}
		windsurf := types.CrossPlatformApp{
			Name:                "Windsurf",
			Category:            "AI Code Editors",
			Description:         "Windsurf editor",
			DesktopEnvironments: []string{types.DesktopEnvironmentGraphical},
			Linux: types.OSConfig{
				InstallMethod:  "apt",
				InstallCommand: "devin-desktop",
				PlatformRequirements: []types.PlatformRequirement{
					{OS: "debian", Arch: "amd64"},
					{OS: "ubuntu", Arch: "amd64"},
				},
			},
		}
		settings := desktopSettingsForTest(zed, windsurf)

		amd64Apps := getAvailableDesktopApps(settings, platform.DetectionResult{
			OS: "linux", Distribution: "ubuntu", DesktopEnv: "gnome", Architecture: "amd64",
		})
		Expect(amd64Apps).To(HaveLen(2))
		Expect([]string{amd64Apps[0].Name, amd64Apps[1].Name}).To(Equal([]string{"Windsurf", "Zed"}))

		arm64Apps := getAvailableDesktopApps(settings, platform.DetectionResult{
			OS: "linux", Distribution: "ubuntu", DesktopEnv: "gnome", Architecture: "arm64",
		})
		Expect(arm64Apps).To(HaveLen(1))
		Expect(arm64Apps[0].Name).To(Equal("Zed"))
	})

	It("uses HasGraphicalDesktop in NewSetupModel when Linux DesktopEnv is unknown or empty", func() {
		cursor := desktopAppForTest("Cursor", "AI Code Editors")
		cursor.DesktopEnvironments = []string{types.DesktopEnvironmentGraphical}
		settings := desktopSettingsForTest(cursor)

		restoreDesktop := platform.SetGraphicalDesktopForTesting(true)
		for _, de := range []string{"unknown", ""} {
			model := NewSetupModel(nil, settings, platform.DetectionResult{OS: "linux", DesktopEnv: de, Architecture: "amd64"})
			Expect(model.system.hasDesktop).To(BeTrue(), "expected hasDesktop=true for DesktopEnv=%q", de)
			Expect(model.system.desktopApps).To(Equal([]string{"Cursor"}))
		}
		restoreDesktop()

		restoreHeadless := platform.SetGraphicalDesktopForTesting(false)
		defer restoreHeadless()
		for _, de := range []string{"unknown", ""} {
			model := NewSetupModel(nil, settings, platform.DetectionResult{OS: "linux", DesktopEnv: de, Architecture: "amd64"})
			Expect(model.system.hasDesktop).To(BeFalse(), "expected hasDesktop=false for DesktopEnv=%q", de)
			Expect(model.system.desktopApps).To(BeEmpty())
		}
	})
})

var _ = Describe("custom setup system info", func() {
	It("reports has_desktop for an unnamed Linux desktop only on a graphical host", func() {
		unknown := platform.DetectionResult{OS: "linux", DesktopEnv: "unknown"}

		DeferCleanup(platform.SetGraphicalDesktopForTesting(true))
		Expect(buildSystemInfo(unknown)["has_desktop"]).To(BeTrue())
		Expect(buildSystemInfo(platform.DetectionResult{OS: "linux", DesktopEnv: "none"})["has_desktop"]).To(BeFalse())

		DeferCleanup(platform.SetGraphicalDesktopForTesting(false))
		Expect(buildSystemInfo(unknown)["has_desktop"]).To(BeFalse())
		Expect(buildSystemInfo(platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"})["has_desktop"]).To(BeTrue())
	})
})
