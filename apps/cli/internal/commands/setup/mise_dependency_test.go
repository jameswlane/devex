package setup

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// miseCatalogEntry mirrors config/applications/development/mise.yaml.
func miseCatalogEntry() types.CrossPlatformApp {
	cfg := types.OSConfig{
		InstallMethod:   types.InstallMethodPlugin,
		InstallCommand:  "package-manager-mise ensure-installed",
		InstalledBinary: "mise",
		SelfUpdating:    true,
		OfficialSupport: true,
	}
	return types.CrossPlatformApp{Name: "mise", Category: "Development Tools", Linux: cfg, MacOS: cfg}
}

// miseLanguage is a programming language entry installed through mise.
func miseLanguage(name, tool string, dependencies ...string) types.CrossPlatformApp {
	return types.CrossPlatformApp{
		Name:         name,
		Category:     "Programming Languages",
		AllPlatforms: types.OSConfig{InstallMethod: "mise", InstallCommand: tool, Dependencies: dependencies},
	}
}

var _ = Describe("Prerequisites declared in YAML dependencies (#757)", func() {
	BeforeEach(func() { DeferCleanup(types.PinCatalogOSForTesting("linux")) })

	var model *SetupModel

	newModel := func(languages []types.CrossPlatformApp, apps ...types.CrossPlatformApp) *SetupModel {
		names := make([]string, 0, len(languages))
		for _, language := range languages {
			names = append(names, language.Name)
		}
		m := &SetupModel{
			step: StepConfirmation,
			system: SystemInfo{
				shells:           []string{"bash"},
				languages:        names,
				detectedPlatform: platform.DetectionResult{OS: "linux", Distribution: "ubuntu"},
			},
			selections: UISelections{selectedLangs: map[int]bool{}, selectedAIAgents: map[string]bool{}},
			repo:       mocks.NewMockRepository(),
			settings: config.CrossPlatformSettings{
				ApplicationOverrides: apps,
				ProgrammingLanguages: languages,
			},
		}
		for _, app := range apps {
			if app.Category == CategoryAICodingAgents {
				m.system.aiAgentApps = append(m.system.aiAgentApps, app)
			}
		}
		return m
	}

	planNames := func(plan []planItem) []string {
		names := make([]string, 0, len(plan))
		for _, item := range plan {
			names = append(names, item.app.Name)
		}
		return names
	}

	It("plans the mise catalog entry before a language that lists it, as a plugin step", func() {
		model = newModel([]types.CrossPlatformApp{miseLanguage("Go", "go@latest", "mise")}, miseCatalogEntry())
		model.selections.selectedLangs[0] = true

		plan := model.installPlan()
		Expect(planNames(plan)).To(Equal([]string{"mise", "mise-go"}))
		Expect(plan[0].group).To(Equal(planDependency))
		Expect(plan[0].reason).To(Equal("languages: Go"))
		Expect(plan[0].pluginStep).To(Equal(&pluginStep{plugin: "package-manager-mise", command: "ensure-installed", args: []string{}}))
		Expect(plan[1].pluginStep).To(BeNil())
	})

	It("names every language that needs mise, including through a language dependency", func() {
		model = newModel([]types.CrossPlatformApp{
			miseLanguage("Elixir", "elixir@latest", "Erlang"),
			miseLanguage("Erlang", "erlang@latest", "mise"),
			miseLanguage("Go", "go@latest", "mise"),
		}, miseCatalogEntry())
		model.selections.selectedLangs[0] = true
		model.selections.selectedLangs[2] = true

		plan := model.installPlan()
		Expect(planNames(plan)).To(Equal([]string{"mise", "mise-erlang", "mise-elixir", "mise-go"}))
		Expect(plan[0].reason).To(Equal("languages: Elixir, Go"))
	})

	It("reads language dependencies from the catalog before any configuration is written", func() {
		golang := miseLanguage("Go", "go@latest", "mise")
		model = newModel(nil, miseCatalogEntry(), golang)
		model.system.languages = []string{"Go"}
		model.selections.selectedLangs[0] = true

		plan := model.installPlan()
		Expect(planNames(plan)[0]).To(Equal("mise"))
		Expect(plan[0].reason).To(Equal("languages: Go"))
	})

	It("never plans mise for a language config that installs another way", func() {
		apt := types.CrossPlatformApp{Name: "Go", Category: "Programming Languages", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "golang"}}
		model = newModel([]types.CrossPlatformApp{apt}, miseCatalogEntry())
		model.selections.selectedLangs[0] = true

		Expect(planNames(model.installPlan())).NotTo(ContainElement("mise"))
	})

	It("plans nothing for a dependency the catalog doesn't define", func() {
		model = newModel([]types.CrossPlatformApp{miseLanguage("Go", "go@latest", "mise")})
		model.selections.selectedLangs[0] = true

		Expect(planNames(model.installPlan())).To(Equal([]string{"mise-go"}))
	})

	It("plans mise once when a language and an AI agent both need it", func() {
		agent := types.CrossPlatformApp{Name: "Agent", Category: CategoryAICodingAgents, Linux: types.OSConfig{
			InstallMethod: "mise", InstallCommand: "node@24 npm:agent@latest", Dependencies: []string{"mise"},
		}}
		model = newModel([]types.CrossPlatformApp{miseLanguage("Go", "go@latest", "mise")}, miseCatalogEntry(), agent)
		model.selections.selectedLangs[0] = true
		model.selections.selectedAIAgents["Agent"] = true

		plan := model.installPlan()
		Expect(planNames(plan)).To(Equal([]string{"mise", "mise-go", "Agent"}))
		Expect(plan[0].reason).To(Equal("languages: Go"))
	})

	It("pulls in mise through an AI agent's language dependency without planning the language", func() {
		agent := types.CrossPlatformApp{Name: "Agent", Category: CategoryAICodingAgents, Linux: types.OSConfig{
			InstallMethod: "mise", InstallCommand: "node@24 npm:agent@latest", Dependencies: []string{"Node.js"},
		}}
		model = newModel([]types.CrossPlatformApp{miseLanguage("Node.js", "node@lts", "mise")}, miseCatalogEntry(), agent)
		model.selections.selectedAIAgents["Agent"] = true

		plan := model.installPlan()
		Expect(planNames(plan)).To(Equal([]string{"mise", "Agent"}))
		Expect(plan[0].reason).To(Equal("AI agents: Agent"))
	})

	It("doesn't plan dependencies that aren't plugin-step prerequisites", func() {
		git := types.CrossPlatformApp{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}}
		agent := types.CrossPlatformApp{Name: "Agent", Category: CategoryAICodingAgents, Linux: types.OSConfig{
			InstallMethod: "curlpipe", DownloadURL: "https://example.invalid/install.sh", InstallScope: "user", Dependencies: []string{"git"},
		}}
		model = newModel(nil, git, agent)
		model.selections.selectedAIAgents["Agent"] = true

		Expect(planNames(model.installPlan())).To(Equal([]string{"Agent"}))
	})

	It("shows the plugin's own description of the mise step on the Confirmation screen", func() {
		original := describePluginStep
		DeferCleanup(func() { describePluginStep = original })
		var described []pluginStep
		describePluginStep = func(_ context.Context, _ *bootstrap.PluginBootstrap, step pluginStep) (sdk.Description, error) {
			described = append(described, step)
			return sdk.Description{Actions: []sdk.PlannedAction{{Kind: sdk.ActionShell, Target: "sh <verified mise installer>", Scope: sdk.ScopeUser, Detail: "installs mise to ~/.local/bin/mise"}}}, nil
		}
		originalCheck := installedAppsCheck
		DeferCleanup(func() { installedAppsCheck = originalCheck })
		installedAppsCheck = func(context.Context, []types.CrossPlatformApp) map[string]bool { return map[string]bool{} }

		model = newModel([]types.CrossPlatformApp{miseLanguage("Go", "go@latest", "mise")}, miseCatalogEntry())
		model.selections.selectedLangs[0] = true
		model.refreshInstalledApps()

		Expect(described).To(HaveLen(1))
		view := model.View()
		Expect(view).To(ContainSubstring("for languages: Go"))
		Expect(view).To(ContainSubstring("plugin: package-manager-mise ensure-installed"))
		Expect(view).To(ContainSubstring("installs mise to ~/.local/bin/mise (user)"))
	})
})
