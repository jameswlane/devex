package commands_test

import (
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/commands"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/mocks"
)

var _ = Describe("Commands", func() {
	var cmd *cobra.Command
	var repo *mocks.MockRepository
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		log.InitDefaultLogger(io.Discard)         // Suppress logs during tests
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

		It("has shell subcommand", func() {
			shellCmd := cmd.Commands()
			Expect(shellCmd).To(HaveLen(1))
			Expect(shellCmd[0].Name()).To(Equal("shell"))

			// Check that shell command has its own subcommands
			shellSubCmds := shellCmd[0].Commands()
			Expect(shellSubCmds).To(HaveLen(6))

			subCmdNames := make([]string, len(shellSubCmds))
			for i, subcmd := range shellSubCmds {
				subCmdNames[i] = subcmd.Name()
			}

			Expect(subCmdNames).To(ContainElement("copy"))
			Expect(subCmdNames).To(ContainElement("append"))
			Expect(subCmdNames).To(ContainElement("status"))
			Expect(subCmdNames).To(ContainElement("list"))
			Expect(subCmdNames).To(ContainElement("debug"))
			Expect(subCmdNames).To(ContainElement("test-copy"))
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
