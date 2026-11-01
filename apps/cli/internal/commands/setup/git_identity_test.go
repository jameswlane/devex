package setup

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

// Keep every spec in this suite independent of the developer's real git config.
var _ = BeforeEach(func() {
	original := readGlobalGitIdentity
	readGlobalGitIdentity = func(context.Context) (string, string) { return "", "" }
	DeferCleanup(func() { readGlobalGitIdentity = original })
})

var _ = Describe("Existing git identity", func() {
	useIdentity := func(name, email string) {
		readGlobalGitIdentity = func(context.Context) (string, string) { return name, email }
	}

	advanceFromTheme := func() *SetupModel {
		model := &SetupModel{
			step: StepTheme,
			system: SystemInfo{
				themes:           []string{"Tokyo Night"},
				detectedPlatform: platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"},
			},
			repo:     mocks.NewMockRepository(),
			settings: config.CrossPlatformSettings{},
		}
		updated, _ := model.nextStep()
		return updated
	}

	It("skips the Git step when a global name and email exist", func() {
		useIdentity("Jane Doe", "jane@example.com")

		model := advanceFromTheme()

		Expect(model.step).To(Equal(StepConfirmation))
		Expect(model.git.gitFullName).To(Equal("Jane Doe"))
		Expect(model.git.gitEmail).To(Equal("jane@example.com"))
		Expect(model.gitIdentityUnchanged()).To(BeTrue())
		Expect(model.View()).To(ContainSubstring("Jane Doe <jane@example.com> (existing, unchanged)"))
	})

	It("shows the Git step pre-filled when only one value exists", func() {
		useIdentity("Jane Doe", "")

		model := advanceFromTheme()

		Expect(model.step).To(Equal(StepGitConfig))
		Expect(model.git.gitFullName).To(Equal("Jane Doe"))
		Expect(model.git.gitEmail).To(BeEmpty())
	})

	It("shows the Git step when the existing email is invalid", func() {
		useIdentity("Jane Doe", "not-an-email")

		Expect(advanceFromTheme().step).To(Equal(StepGitConfig))
	})

	It("shows the empty Git step when git has no identity or is missing", func() {
		model := advanceFromTheme()

		Expect(model.step).To(Equal(StepGitConfig))
		Expect(model.git.gitFullName).To(BeEmpty())
		Expect(model.git.gitEmail).To(BeEmpty())
	})

	It("opens the pre-filled Git step when going back from Confirmation", func() {
		useIdentity("Jane Doe", "jane@example.com")
		model := advanceFromTheme()

		model, _ = model.prevStep()

		Expect(model.step).To(Equal(StepGitConfig))
		Expect(model.git.gitFullName).To(Equal("Jane Doe"))
	})

	It("treats an edited identity as a change to apply", func() {
		useIdentity("Jane Doe", "jane@example.com")
		model := advanceFromTheme()

		model.git.gitEmail = "jane@work.example"

		Expect(model.gitIdentityUnchanged()).To(BeFalse())
		Expect(model.View()).NotTo(ContainSubstring("(existing, unchanged)"))
	})

	It("does not read git config more than once", func() {
		calls := 0
		readGlobalGitIdentity = func(context.Context) (string, string) {
			calls++
			return "", ""
		}
		model := advanceFromTheme()
		model.step = StepTheme
		_, _ = model.nextStep()

		Expect(calls).To(Equal(1))
	})
})
