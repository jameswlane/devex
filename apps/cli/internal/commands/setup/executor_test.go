package setup_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands/setup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("SetupExecutor", func() {
	var (
		executor         *setup.SetupExecutor
		setupConfig      *types.SetupConfig
		settings         config.CrossPlatformSettings
		detectedPlatform platform.DetectionResult
	)

	BeforeEach(func() {
		// Create a basic setup configuration for testing
		setupConfig = &types.SetupConfig{
			Metadata: types.SetupMetadata{
				Name:        "Test Setup",
				Description: "Test setup configuration",
				Version:     "1.0.0",
			},
			Steps: []types.SetupStep{
				{
					ID:    "step1",
					Title: "First Step",
					Type:  types.StepTypeInfo,
					Info: &types.InfoContent{
						Message: "Welcome to setup",
						Style:   types.InfoStyleInfo,
					},
					Navigation: types.StepNavigation{
						AllowBack: false,
					},
				},
				{
					ID:    "step2",
					Title: "Second Step",
					Type:  types.StepTypeQuestion,
					Question: &types.Question{
						Type:     types.QuestionTypeText,
						Variable: "username",
						Prompt:   "Enter your username:",
						Validation: &types.Validation{
							Required: true,
							Min:      intPtr(3),
						},
					},
					Navigation: types.StepNavigation{
						AllowBack: true,
					},
				},
				{
					ID:    "step3",
					Title: "Third Step",
					Type:  types.StepTypeInfo,
					Info: &types.InfoContent{
						Message: "Hello {{.username}}!",
						Style:   types.InfoStyleSuccess,
					},
					Navigation: types.StepNavigation{
						AllowBack: true,
					},
				},
			},
		}

		// Create basic settings
		settings = config.CrossPlatformSettings{}

		// Create platform detection result
		detectedPlatform = platform.DetectionResult{
			OS:           "linux",
			Distribution: "debian",
			DesktopEnv:   "gnome",
			Architecture: "amd64",
		}

		// Create executor
		executor = setup.NewSetupExecutor(setupConfig, settings, nil, detectedPlatform)
	})

	Describe("NewSetupExecutor", func() {
		It("should create executor with initial state", func() {
			Expect(executor).NotTo(BeNil())
			Expect(executor.GetCurrentStep()).NotTo(BeNil())
			Expect(executor.GetCurrentStep().ID).To(Equal("step1"))
		})

		It("should initialize system info from platform detection", func() {
			state := executor.GetState()
			Expect(state.SystemInfo["os"]).To(Equal("linux"))
			Expect(state.SystemInfo["distribution"]).To(Equal("debian"))
			Expect(state.SystemInfo["desktop"]).To(Equal("gnome"))
		})
	})

	Describe("GetCurrentStep", func() {
		It("should return the first step initially", func() {
			step := executor.GetCurrentStep()
			Expect(step).NotTo(BeNil())
			Expect(step.ID).To(Equal("step1"))
		})

		It("should return nil when at end", func() {
			// Advance to end
			err := executor.NextStep()
			Expect(err).NotTo(HaveOccurred())
			err = executor.NextStep()
			Expect(err).NotTo(HaveOccurred())
			err = executor.NextStep()
			Expect(err).NotTo(HaveOccurred())

			step := executor.GetCurrentStep()
			Expect(step).To(BeNil())
		})
	})

	Describe("NextStep", func() {
		It("should advance to next step in sequence", func() {
			err := executor.NextStep()
			Expect(err).NotTo(HaveOccurred())

			step := executor.GetCurrentStep()
			Expect(step.ID).To(Equal("step2"))
		})

		It("should reach end of steps", func() {
			err := executor.NextStep()
			Expect(err).NotTo(HaveOccurred())
			err = executor.NextStep()
			Expect(err).NotTo(HaveOccurred())
			err = executor.NextStep()
			Expect(err).NotTo(HaveOccurred())

			Expect(executor.IsComplete()).To(BeTrue())
		})

		Context("with conditional steps", func() {
			BeforeEach(func() {
				// Add conditional step
				setupConfig.Steps = append(setupConfig.Steps, types.SetupStep{
					ID:    "conditional",
					Title: "Conditional Step",
					Type:  types.StepTypeInfo,
					ShowIf: &types.Condition{
						Operator: types.OperatorEquals,
						Variable: "username",
						Value:    "admin",
					},
				})
				executor = setup.NewSetupExecutor(setupConfig, settings, nil, detectedPlatform)
			})

			It("should skip conditional step when condition is false", func() {
				executor.SetAnswer("username", "user")

				// Advance through steps
				err := executor.NextStep() // step1 -> step2
				Expect(err).NotTo(HaveOccurred())
				err = executor.NextStep() // step2 -> step3 (skip conditional)
				Expect(err).NotTo(HaveOccurred())

				step := executor.GetCurrentStep()
				Expect(step.ID).To(Equal("step3"))
			})

			It("should show conditional step when condition is true", func() {
				executor.SetAnswer("username", "admin")

				// Advance through steps
				err := executor.NextStep() // step1 -> step2
				Expect(err).NotTo(HaveOccurred())
				err = executor.NextStep() // step2 -> step3
				Expect(err).NotTo(HaveOccurred())
				err = executor.NextStep() // step3 -> conditional
				Expect(err).NotTo(HaveOccurred())

				step := executor.GetCurrentStep()
				Expect(step.ID).To(Equal("conditional"))
			})
		})
	})

	Describe("PrevStep", func() {
		BeforeEach(func() {
			// Advance to second step
			err := executor.NextStep()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should go back to previous step", func() {
			err := executor.PrevStep()
			Expect(err).NotTo(HaveOccurred())

			step := executor.GetCurrentStep()
			Expect(step.ID).To(Equal("step1"))
		})

		It("should not allow going back from step with AllowBack=false", func() {
			// Go back to step1 (which allows going back from step2)
			err := executor.PrevStep()
			Expect(err).NotTo(HaveOccurred())

			step := executor.GetCurrentStep()
			Expect(step.ID).To(Equal("step1"))

			// Now try to go back from step1 (which has AllowBack: false)
			err = executor.PrevStep()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot go back"))
		})
	})

	Describe("SetAnswer and GetAnswer", func() {
		It("should store and retrieve answers", func() {
			executor.SetAnswer("username", "testuser")

			value, ok := executor.GetAnswer("username")
			Expect(ok).To(BeTrue())
			Expect(value).To(Equal("testuser"))
		})

		It("should return false for non-existent answers", func() {
			_, ok := executor.GetAnswer("nonexistent")
			Expect(ok).To(BeFalse())
		})
	})

	Describe("ValidateAnswer", func() {
		var question *types.Question

		BeforeEach(func() {
			question = &types.Question{
				Type:     types.QuestionTypeText,
				Variable: "test",
				Prompt:   "Test:",
				Validation: &types.Validation{
					Required: true,
					Min:      intPtr(3),
					Max:      intPtr(10),
					Pattern:  "^[a-z]+$",
					Message:  "Must be 3-10 lowercase letters",
				},
			}
		})

		It("should accept valid answer", func() {
			err := executor.ValidateAnswer(question, "hello")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject empty answer when required", func() {
			err := executor.ValidateAnswer(question, "")
			Expect(err).To(HaveOccurred())
		})

		It("should reject answer below minimum length", func() {
			err := executor.ValidateAnswer(question, "ab")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Must be 3-10 lowercase letters"))
		})

		It("should reject answer above maximum length", func() {
			err := executor.ValidateAnswer(question, "verylongtext")
			Expect(err).To(HaveOccurred())
		})

		It("should reject answer not matching pattern", func() {
			err := executor.ValidateAnswer(question, "Hello123")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Must be 3-10 lowercase letters"))
		})
	})

	Describe("InterpolateString", func() {
		BeforeEach(func() {
			executor.SetAnswer("username", "john")
			executor.SetAnswer("email", "john@example.com")
		})

		It("should interpolate simple variables", func() {
			result, err := executor.InterpolateString("Hello {{.username}}!")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("Hello john!"))
		})

		It("should interpolate multiple variables", func() {
			result, err := executor.InterpolateString("User: {{.username}}, Email: {{.email}}")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("User: john, Email: john@example.com"))
		})

		It("should interpolate system info", func() {
			result, err := executor.InterpolateString("OS: {{.os}}, Desktop: {{.desktop}}")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal("OS: linux, Desktop: gnome"))
		})

		It("should handle invalid template", func() {
			result, err := executor.InterpolateString("Hello {{.username")
			Expect(err).To(HaveOccurred())
			Expect(result).To(Equal("Hello {{.username"))
		})
	})

	Describe("LoadOptions", func() {
		It("should return static options when provided", func() {
			question := &types.Question{
				Options: []types.QuestionOption{
					{Label: "Option 1", Value: "opt1"},
					{Label: "Option 2", Value: "opt2"},
				},
			}

			options, err := executor.LoadOptions(question)
			Expect(err).NotTo(HaveOccurred())
			Expect(options).To(HaveLen(2))
			Expect(options[0].Label).To(Equal("Option 1"))
		})

		It("should filter options based on ShowIf condition", func() {
			executor.SetAnswer("advanced", true)

			question := &types.Question{
				Options: []types.QuestionOption{
					{Label: "Basic", Value: "basic"},
					{
						Label: "Advanced",
						Value: "advanced",
						ShowIf: &types.Condition{
							Operator: types.OperatorEquals,
							Variable: "advanced",
							Value:    true,
						},
					},
				},
			}

			options, err := executor.LoadOptions(question)
			Expect(err).NotTo(HaveOccurred())
			Expect(options).To(HaveLen(2))
		})
	})

	Describe("GetProgress", func() {
		It("should return 0% at first step", func() {
			progress := executor.GetProgress()
			Expect(progress).To(BeNumerically("~", 0, 1))
		})

		It("should return 100% when complete", func() {
			// Advance to end
			executor.NextStep()
			executor.NextStep()
			executor.NextStep()

			progress := executor.GetProgress()
			Expect(progress).To(BeNumerically("~", 100, 1))
		})

		It("should return intermediate progress", func() {
			executor.NextStep()

			progress := executor.GetProgress()
			Expect(progress).To(BeNumerically("~", 33, 5))
		})
	})

	Describe("IsComplete", func() {
		It("should return false initially", func() {
			Expect(executor.IsComplete()).To(BeFalse())
		})

		It("should return true when all steps complete", func() {
			executor.NextStep()
			executor.NextStep()
			executor.NextStep()

			Expect(executor.IsComplete()).To(BeTrue())
		})
	})
})

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
