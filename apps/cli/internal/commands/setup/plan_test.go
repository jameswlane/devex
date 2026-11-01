package setup

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Install plan and Confirmation screen", func() {
	var (
		model            *SetupModel
		originalCheck    func(context.Context, []types.CrossPlatformApp) map[string]bool
		installedOnStart map[string]bool
	)

	aptDefault := func(name string) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: name}}
	}

	BeforeEach(func() {
		installedOnStart = map[string]bool{}
		originalCheck = installedAppsCheck
		installedAppsCheck = func(_ context.Context, apps []types.CrossPlatformApp) map[string]bool {
			result := map[string]bool{}
			for _, app := range apps {
				if installedOnStart[app.Name] {
					result[app.Name] = true
				}
			}
			return result
		}
		DeferCleanup(func() { installedAppsCheck = originalCheck })

		model = &SetupModel{
			step: StepConfirmation,
			system: SystemInfo{
				shells:           []string{"bash"},
				languages:        []string{"Elixir"},
				detectedPlatform: platform.DetectionResult{OS: "linux", Distribution: "ubuntu"},
			},
			selections: UISelections{selectedLangs: map[int]bool{0: true}},
			repo:       mocks.NewMockRepository(),
			settings: config.CrossPlatformSettings{ApplicationOverrides: []types.CrossPlatformApp{
				aptDefault("bat"),
				aptDefault("gh"),
				{Name: "macos-only", Default: true, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "x"}},
			}},
		}
	})

	It("installs exactly what the Confirmation screen lists", func() {
		model.refreshInstalledApps()
		view := model.View()

		for _, app := range model.buildAppList() {
			Expect(view).To(ContainSubstring(app.Name))
		}
	})

	It("groups defaults, dependencies with their reason, and selections", func() {
		model.refreshInstalledApps()
		view := model.View()

		Expect(view).To(ContainSubstring("📦 Default tools (2)"))
		Expect(view).To(ContainSubstring("🔗 Required by your selections (1)"))
		Expect(view).To(ContainSubstring("mise"))
		Expect(view).To(ContainSubstring("for languages: Elixir"))
		Expect(view).To(ContainSubstring("✨ Your selections"))
	})

	It("leaves out defaults that cannot install on this platform", func() {
		Expect(model.View()).NotTo(ContainSubstring("macos-only"))
		for _, app := range model.buildAppList() {
			Expect(app.Name).NotTo(Equal("macos-only"))
		}
	})

	It("marks apps that are already installed and excludes them from the total", func() {
		installedOnStart["gh"] = true
		model.refreshInstalledApps()
		view := model.View()

		Expect(view).To(ContainSubstring("gh"))
		Expect(view).To(ContainSubstring("already installed, will skip"))
		Expect(view).To(ContainSubstring("1 already installed"))
	})

	It("lets the user skip the default tools with 'd'", func() {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		model = updated.(*SetupModel)

		Expect(model.selections.skipDefaultApps).To(BeTrue())
		Expect(model.View()).NotTo(ContainSubstring("📦 Default tools"))
		Expect(model.View()).To(ContainSubstring("Default tools skipped"))
		for _, app := range model.buildAppList() {
			Expect(app.Default).To(BeFalse())
		}
	})

	It("shows the package list update only when something installs through apt", func() {
		Expect(model.View()).To(ContainSubstring("apt-get update"))

		model.selections.skipDefaultApps = true
		Expect(model.View()).NotTo(ContainSubstring("apt-get update"))
	})
})
