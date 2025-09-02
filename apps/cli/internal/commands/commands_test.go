package commands_test

import (
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Commands", func() {
	var cmd *cobra.Command
	var repo *mocks.MockRepository
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		log.InitTestLogger()                      // Initialize test logger to suppress all output
		repo = &mocks.MockRepository{}            // Use centralized mock repository
		settings = config.CrossPlatformSettings{} // Mock settings
	})

	_ = Describe("Install Command", func() {
		It("creates the install command without errors", func() {
			cmd := commands.NewInstallCmd(repo, settings)
			Expect(cmd).ToNot(BeNil())
			Expect(cmd.Use).To(Equal("install [apps...]"))
		})
	})

	Context("System Command", func() {
		BeforeEach(func() {
			cmd = commands.NewSystemCmd(settings)
			cmd.SetOut(io.Discard) // Suppress output
			cmd.SetErr(io.Discard) // Suppress error output
		})

		It("shows help when no subcommand is provided", func() {
			cmd.SetArgs([]string{})
			err := cmd.Execute()
			Expect(err).ToNot(HaveOccurred())
		})

		It("has no subcommands since functionality moved to plugins", func() {
			subCommands := cmd.Commands()
			Expect(subCommands).To(HaveLen(0))
		})
	})

	Context("Completion Command", func() {
		BeforeEach(func() {
			cmd = commands.NewCompletionCmd()
			cmd.SetOut(io.Discard) // Suppress output
			cmd.SetErr(io.Discard) // Suppress error output
		})

		It("supports valid shells", func() {
			shells := []string{"bash", "zsh", "fish", "powershell"}
			for _, shell := range shells {
				cmd.SetArgs([]string{shell})
				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("errors on invalid shell", func() {
			cmd.SetArgs([]string{"invalid-shell"})
			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid argument"))
		})
	})
})
