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
		It("executes the install command without errors", func() {
			cmd := commands.NewInstallCmd(repo, settings)
			cmd.SetArgs([]string{"test-app"})
			err := cmd.Execute()
			Expect(err).To(BeNil())
		})
	})

	Context("System Command", func() {
		BeforeEach(func() {
			cmd = commands.NewSystemCmd()
			cmd.SetOut(io.Discard) // Suppress output
			cmd.SetErr(io.Discard) // Suppress error output
		})

		It("requires the --user flag", func() {
			cmd.SetArgs([]string{})
			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("required flag(s) \"user\" not set"))
		})

		It("executes successfully with the --user flag", func() {
			cmd.SetArgs([]string{"--user", "testuser"})
			err := cmd.Execute()
			Expect(err).ToNot(HaveOccurred())
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
