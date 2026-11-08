package setup

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

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

var _ = Describe("Automated git identity", func() {
	BeforeEach(func() { viper.Reset(); DeferCleanup(viper.Reset) })
	DescribeTable("fills only missing global identity fields",
		func(existingName, existingEmail, expectedName, expectedEmail string) {
			readGlobalGitIdentity = func(context.Context) (string, string) { return existingName, existingEmail }
			viper.Set("setup.git.name", "Setup User")
			viper.Set("setup.git.email", "setup@example.com")
			model, err := createAutomatedSetupModel(mocks.NewMockRepository(), config.CrossPlatformSettings{})
			Expect(err).NotTo(HaveOccurred())
			model.loadAutomatedGitIdentity(context.Background())
			Expect(model.git.gitFullName).To(Equal(expectedName))
			Expect(model.git.gitEmail).To(Equal(expectedEmail))
		},
		Entry("new identity", "", "", "Setup User", "setup@example.com"),
		Entry("complete identity", "Existing User", "existing@example.com", "Existing User", "existing@example.com"),
		Entry("name only", "Existing User", "", "Existing User", "setup@example.com"),
		Entry("email only", "", "existing@example.com", "Setup User", "existing@example.com"),
	)
	It("does not invent an identity when no inputs are supplied", func() {
		model, err := createAutomatedSetupModel(mocks.NewMockRepository(), config.CrossPlatformSettings{})
		Expect(err).NotTo(HaveOccurred())
		model.loadAutomatedGitIdentity(context.Background())
		Expect(model.git.gitFullName).To(BeEmpty())
		Expect(model.git.gitEmail).To(BeEmpty())
	})
	It("reports configuration failures in the summary and returns an error", func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("XDG_DATA_HOME", GinkgoT().TempDir())
		viper.Set("offline", true)
		viper.Set("setup.git.name", "Setup User")
		Expect(os.MkdirAll(filepath.Join(os.Getenv("XDG_DATA_HOME"), "devex", "plugins"), 0750)).To(Succeed())
		err := ConfigureAutomatedGit(context.Background(), mocks.NewMockRepository(), config.CrossPlatformSettings{})
		Expect(err).To(MatchError(ContainSubstring("setup completed with 1 errors:")))
		Expect(err).To(MatchError(ContainSubstring("Git configuration:")))
		Expect(err).To(MatchError(ContainSubstring("tool-git plugin is not installed")))
	})
	It("keeps an existing complete identity without requiring a plugin", func() {
		readGlobalGitIdentity = func(context.Context) (string, string) { return "Existing User", "existing@example.com" }
		viper.Set("setup.git.name", "Replacement User")
		viper.Set("setup.git.email", "replacement@example.com")
		Expect(ConfigureAutomatedGit(context.Background(), nil, config.CrossPlatformSettings{})).To(Succeed())
	})
})
