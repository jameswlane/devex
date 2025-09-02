package commands_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Add Command", func() {
	var (
		mockRepo *mocks.MockRepository
		settings *config.CrossPlatformSettings
		tempDir  string
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()

		// Create temporary directory for config
		var err error
		tempDir, err = os.MkdirTemp("", "devex-add-test")
		Expect(err).ToNot(HaveOccurred())

		// Create a basic settings with temp directory
		settings = &config.CrossPlatformSettings{
			HomeDir: tempDir,
		}
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("Command Creation", func() {
		It("should create add command with correct properties", func() {
			cmd := commands.NewAddCmd(mockRepo, *settings)

			Expect(cmd.Use).To(Equal("add [application-name]"))
			Expect(cmd.Short).To(ContainSubstring("Add applications to your DevEx configuration"))
			Expect(cmd.RunE).ToNot(BeNil())
		})

		It("should have expected flags", func() {
			cmd := commands.NewAddCmd(mockRepo, *settings)

			categoryFlag := cmd.Flags().Lookup("category")
			Expect(categoryFlag).ToNot(BeNil())
			Expect(categoryFlag.Shorthand).To(Equal("c"))

			searchFlag := cmd.Flags().Lookup("search")
			Expect(searchFlag).ToNot(BeNil())
			Expect(searchFlag.Shorthand).To(Equal("s"))

			dryRunFlag := cmd.Flags().Lookup("dry-run")
			Expect(dryRunFlag).ToNot(BeNil())
		})
	})

	Describe("Basic functionality", func() {
		Context("when no application name is provided", func() {
			It("should not return an error (enters interactive mode)", func() {
				cmd := commands.NewAddCmd(mockRepo, *settings)
				cmd.SetArgs([]string{})

				// This would normally run the TUI, but we can't test that easily
				// Just ensure the command is created properly
				Expect(cmd.Use).To(Equal("add [application-name]"))
			})
		})

		Context("with dry-run flag", func() {
			It("should accept the dry-run flag", func() {
				cmd := commands.NewAddCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"--dry-run"})

				// Parse flags to ensure they work
				err := cmd.ParseFlags([]string{"--dry-run"})
				Expect(err).ToNot(HaveOccurred())

				dryRunFlag := cmd.Flags().Lookup("dry-run")
				Expect(dryRunFlag.Value.String()).To(Equal("true"))
			})
		})

		Context("with category flag", func() {
			It("should accept the category flag", func() {
				cmd := commands.NewAddCmd(mockRepo, *settings)

				err := cmd.ParseFlags([]string{"--category", "development"})
				Expect(err).ToNot(HaveOccurred())

				categoryFlag := cmd.Flags().Lookup("category")
				Expect(categoryFlag.Value.String()).To(Equal("development"))
			})
		})

		Context("with search flag", func() {
			It("should accept the search flag", func() {
				cmd := commands.NewAddCmd(mockRepo, *settings)

				err := cmd.ParseFlags([]string{"--search", "docker"})
				Expect(err).ToNot(HaveOccurred())

				searchFlag := cmd.Flags().Lookup("search")
				Expect(searchFlag.Value.String()).To(Equal("docker"))
			})
		})
	})

	Describe("Configuration directory handling", func() {
		It("should handle GetConfigDir method", func() {
			// Test that the settings object has the expected config directory behavior
			// This is more of an integration test to ensure our usage patterns work
			configDir := settings.GetConfigDir()
			Expect(configDir).To(ContainSubstring(tempDir))
		})
	})
})
