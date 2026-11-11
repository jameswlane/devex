package setup

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Completion change inventory", func() {
	var model *SetupModel

	aptDefault := func(name string) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: name}}
	}

	BeforeEach(func() {
		model = &SetupModel{
			step: StepComplete,
			system: SystemInfo{shells: []string{"bash"}, languages: []string{"Go"},
				detectedPlatform: platform.DetectionResult{OS: "linux", Distribution: "ubuntu"}},
			selections: UISelections{selectedLangs: map[int]bool{0: true}},
			settings:   config.CrossPlatformSettings{ApplicationOverrides: []types.CrossPlatformApp{aptDefault("bat"), aptDefault("gh"), aptDefault("plocate")}},
		}
	})

	It("lists each planned app under its actual outcome and every configuration change", func() {
		outcomes := model.installation.outcomeRecorder()
		outcomes.Record("bat", tui.OutcomeInstalled, nil)
		outcomes.Record("gh", tui.OutcomeSkipped, nil)
		outcomes.Record("plocate", tui.OutcomeFailed, errors.New("boom"))
		outcomes.Record("mise", tui.OutcomeInstalled, nil)
		outcomes.Record("mise-go", tui.OutcomeCanceled, context.Canceled)
		model.installation.addChange("Shell configuration applied for bash")
		model.installation.addChange("Git identity kept: Ada <ada@example.com>")

		view := model.View()
		Expect(view).To(ContainSubstring("What changed:"))
		Expect(view).To(ContainSubstring("Installed (2): bat, mise"))
		Expect(view).To(ContainSubstring("Already installed, skipped (1): gh"))
		Expect(view).To(ContainSubstring("Failed (1): plocate"))
		Expect(view).To(ContainSubstring("Canceled (1): mise-go"))
		Expect(view).To(ContainSubstring("Shell configuration applied for bash"))
		Expect(view).To(ContainSubstring("Git identity kept: Ada <ada@example.com>"))
		Expect(view).NotTo(ContainSubstring("Essential development tools"))
	})

	It("reports planned apps with no recorded outcome as not run, and Confirmation skips as skipped", func() {
		model.selections.alreadyInstalled = map[string]bool{"gh": true}
		model.installation.outcomeRecorder().Record("bat", tui.OutcomeInstalled, nil)

		inventory := model.renderChangeInventory()
		Expect(inventory).To(ContainSubstring("Already installed, skipped (1): gh"))
		Expect(inventory).To(ContainSubstring("Not run (3): plocate, mise, mise-go"))
		Expect(inventory).To(ContainSubstring("No configuration changes"))
	})

	It("lists default tools the user skipped separately from install outcomes", func() {
		model.selections.excludedDefaults = map[string]bool{"gh": true}
		inventory := model.renderChangeInventory()
		Expect(inventory).To(ContainSubstring("Skipped by your choice (1): gh"))
		Expect(inventory).To(ContainSubstring("Not run (4): bat, plocate, mise, mise-go"))

		model.selections.skipDefaultApps = true
		Expect(model.renderChangeInventory()).To(ContainSubstring("Skipped by your choice (3): bat, gh, plocate"))
	})

	It("keeps the first outcome recorded for an app", func() {
		recorder := tui.NewOutcomeRecorder()
		recorder.Record("bat", tui.OutcomeInstalled, nil)
		recorder.Record("bat", tui.OutcomeNotRun, errors.New("later"))
		Expect(recorder.Results()).To(Equal([]tui.AppResult{{Name: "bat", Outcome: tui.OutcomeInstalled}}))
	})
})
