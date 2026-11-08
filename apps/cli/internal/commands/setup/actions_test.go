package setup

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dynamic desktop installation", func() {
	var settings config.CrossPlatformSettings
	var action *types.StepAction
	var state *types.SetupState
	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("XDG_CACHE_HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
		settings = config.CrossPlatformSettings{HomeDir: home, ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "editor", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "true"}},
			{Name: "browser", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "true"}},
			{Name: "unselected", Default: true, AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "false"}},
		}}
		action = &types.StepAction{Type: types.ActionTypeInstall, Params: map[string]interface{}{"install_desktop_apps": "{{.selected_desktop_apps}}"}}
		state = &types.SetupState{Answers: map[string]interface{}{"selected_desktop_apps": []interface{}{"editor", "browser", "editor"}}}
	})
	newModel := func() *DynamicSetupModel {
		model := NewDynamicSetupModel(&types.SetupConfig{Steps: []types.SetupStep{{ID: "install", Type: types.StepTypeAction, Action: action}}}, mocks.NewMockRepository(), settings, platform.DetectionResult{OS: "linux"}, nil)
		model.executor.state.Answers = state.Answers
		return model
	}
	run := func(model *DynamicSetupModel) {
		_, err := tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(strings.NewReader("\r")), tea.WithOutput(io.Discard)).Run()
		Expect(err).NotTo(HaveOccurred())
	}
	It("runs each selected app and its lifecycle hooks once through the wizard", func() {
		recorder := filepath.Join(settings.HomeDir, "calls")
		for i := range 2 {
			cfg := &settings.ApplicationOverrides[i].AllPlatforms
			name := settings.ApplicationOverrides[i].Name
			cfg.PreInstall = []types.InstallCommand{{Shell: "echo pre-" + name + " >> " + recorder}}
			cfg.PostInstall = []types.InstallCommand{{Shell: "echo post-" + name + " >> " + recorder}}
		}
		model := newModel()
		calls := 0
		model.actionExecutor.installApps = func(ctx context.Context, apps []types.CrossPlatformApp, repo types.Repository, gotSettings config.CrossPlatformSettings) error {
			calls++
			Expect(apps).To(Equal(settings.ApplicationOverrides[:2]))
			Expect(repo).To(BeIdenticalTo(model.executor.repo))
			return tui.NewStreamingInstaller(nil, repo, ctx, gotSettings).InstallApps(ctx, apps, gotSettings)
		}
		run(model)
		Expect(model.CompletionError()).To(Succeed())
		Expect(calls).To(Equal(1))
		content, err := os.ReadFile(recorder)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Fields(string(content))).To(Equal([]string{"pre-editor", "post-editor", "pre-browser", "post-browser"}))
		for _, name := range []string{"editor", "browser"} {
			_, err := model.executor.repo.GetApp(name)
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = model.executor.repo.GetApp("unselected")
		Expect(err).To(HaveOccurred())
	})
	It("aggregates missing definitions and lifecycle failures and returns them from the wizard", func() {
		state.Answers["selected_desktop_apps"] = []string{"editor", "missing", "browser"}
		for i := range 2 {
			settings.ApplicationOverrides[i].AllPlatforms.PreInstall = []types.InstallCommand{{Shell: "false"}}
		}
		model := newModel()
		model.actionExecutor.installApps = func(ctx context.Context, apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
			return tui.NewStreamingInstaller(nil, repo, ctx, settings).InstallApps(ctx, apps, settings)
		}
		run(model)
		Expect(model.CompletionError()).To(MatchError(ContainSubstring("missing")))
		Expect(model.CompletionError().Error()).To(ContainSubstring("editor:"))
		Expect(model.CompletionError().Error()).To(ContainSubstring("browser:"))
		Expect(model.executor.IsComplete()).To(BeFalse())
	})
	It("preserves the command context and wrapped installation errors", func() {
		executor := NewActionExecutor(nil, settings, platform.DetectionResult{}, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		executor.installApps = func(got context.Context, _ []types.CrossPlatformApp, _ types.Repository, _ config.CrossPlatformSettings) error {
			return got.Err()
		}
		Expect(errors.Is(executor.Execute(ctx, action, state), context.Canceled)).To(BeTrue())
	})
	It("does no installation for an empty selection or a missing answer", func() {
		executor := NewActionExecutor(nil, settings, platform.DetectionResult{}, nil)
		executor.installApps = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) error {
			Fail("unexpected installation")
			return nil
		}
		state.Answers["selected_desktop_apps"] = []string{}
		Expect(executor.Execute(context.Background(), action, state)).To(Succeed())
		delete(state.Answers, "selected_desktop_apps")
		Expect(executor.Execute(context.Background(), action, state)).To(MatchError(ContainSubstring("selection")))
	})
	DescribeTable("accepts static selections", func(items interface{}) {
		executor := NewActionExecutor(nil, settings, platform.DetectionResult{}, nil)
		executor.installApps = func(_ context.Context, apps []types.CrossPlatformApp, _ types.Repository, _ config.CrossPlatformSettings) error {
			Expect(apps).To(Equal(settings.ApplicationOverrides[:2]))
			return nil
		}
		action.Params["install_desktop_apps"] = items
		Expect(executor.Execute(context.Background(), action, state)).To(Succeed())
	}, Entry("comma separated", "editor, browser,editor"), Entry("string slice", []string{"editor", "browser"}), Entry("yaml list", []interface{}{"editor", "browser"}))
})
