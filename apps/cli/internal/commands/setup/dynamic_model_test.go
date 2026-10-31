package setup

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Dynamic wizard option defaults", func() {
	BeforeEach(func() {
		original, present := os.LookupEnv("SHELL")
		DeferCleanup(func() {
			if present {
				Expect(os.Setenv("SHELL", original)).To(Succeed())
			} else {
				Expect(os.Unsetenv("SHELL")).To(Succeed())
			}
		})
	})

	shellQuestion := func() types.SetupStep {
		return types.SetupStep{
			ID: "shell", Type: types.StepTypeQuestion,
			Navigation: types.StepNavigation{AllowBack: true},
			Question: &types.Question{Type: types.QuestionTypeSelect, Variable: "selected_shell",
				OptionsSource: &types.OptionsSource{Type: types.SourceTypeSystem, SystemType: "shells"}},
		}
	}
	newModel := func(steps ...types.SetupStep) *DynamicSetupModel {
		return NewDynamicSetupModel(&types.SetupConfig{Steps: steps}, nil, config.CrossPlatformSettings{}, platform.DetectionResult{OS: "linux"}, nil)
	}
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	DescribeTable("accepts the current shell on entering a question", func(shellEnv, expected string) {
		if shellEnv == "" {
			Expect(os.Unsetenv("SHELL")).To(Succeed())
		} else {
			Expect(os.Setenv("SHELL", shellEnv)).To(Succeed())
		}
		model := newModel(types.SetupStep{ID: "welcome", Type: types.StepTypeInfo}, shellQuestion())
		model.Update(enter)
		Expect(model.options[model.cursor].Value).To(Equal(expected))
		model.Update(enter)
		Expect(model.executor.GetState().Answers["selected_shell"]).To(Equal(expected))
	},
		Entry("bash", "/bin/bash", "bash"),
		Entry("zsh", "/bin/zsh", "zsh"),
		Entry("fish", "/usr/bin/fish", "fish"),
		Entry("unset", "", "bash"),
		Entry("unsupported", "/bin/dash", "bash"),
	)

	It("initializes the first question and allows navigating the loaded options", func() {
		Expect(os.Setenv("SHELL", "/bin/bash")).To(Succeed())
		model := newModel(shellQuestion())
		Expect(model.options[model.cursor].Value).To(Equal("bash"))
		model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model.Update(enter)
		Expect(model.executor.GetState().Answers["selected_shell"]).To(Equal("fish"))
	})

	It("initializes defaults after an action completes", func() {
		Expect(os.Setenv("SHELL", "/usr/bin/fish")).To(Succeed())
		model := newModel(types.SetupStep{ID: "action", Type: types.StepTypeAction}, shellQuestion())
		model.Update(ActionCompleteMsg{})
		model.Update(enter)
		Expect(model.executor.GetState().Answers["selected_shell"]).To(Equal("fish"))
	})

	It("initializes defaults after a question and on navigating back", func() {
		Expect(os.Setenv("SHELL", "/bin/bash")).To(Succeed())
		first := shellQuestion()
		first.ID = "first"
		model := newModel(first, shellQuestion())
		model.Update(enter)
		Expect(model.options[model.cursor].Value).To(Equal("bash"))
		model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
		Expect(model.executor.GetCurrentStep().ID).To(Equal("first"))
		Expect(model.options[model.cursor].Value).To(Equal("bash"))
	})

	It("selects every default in a static multiselect question", func() {
		model := newModel(types.SetupStep{ID: "tools", Type: types.StepTypeQuestion,
			Question: &types.Question{Type: types.QuestionTypeMultiSelect, Variable: "tools", Options: []types.QuestionOption{
				{Value: "one"}, {Value: "two", Default: true}, {Value: "three", Default: true},
			}},
		})
		Expect(model.cursor).To(Equal(1))
		model.Update(enter)
		Expect(model.executor.GetState().Answers["tools"]).To(Equal([]string{"two", "three"}))
	})

	It("uses the first option when none is marked as default", func() {
		model := newModel(types.SetupStep{ID: "tools", Type: types.StepTypeQuestion,
			Question: &types.Question{Type: types.QuestionTypeSelect, Variable: "tool", Options: []types.QuestionOption{
				{Value: "one"}, {Value: "two"},
			}},
		})
		model.Update(enter)
		Expect(model.executor.GetState().Answers["tool"]).To(Equal("one"))
	})
})
