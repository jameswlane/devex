package utilities

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("Deb Package Installation Check", func() {
	var mockCommandExec *mocks.MockCommandExecutor

	BeforeEach(func() {
		mockCommandExec = mocks.NewMockCommandExecutor()
		utils.CommandExec = mockCommandExec
	})

	Context("isDebInstalled", func() {
		It("should find command in current PATH", func() {
			// By default, the mock executor will succeed for "which" commands
			result := isDebInstalled("fastfetch")

			Expect(result).To(BeTrue())
		})

		It("should find command in extended PATH when not in current PATH", func() {
			// Set specific which command to fail, but extended PATH should work
			mockCommandExec.FailingCommands["which fastfetch"] = true

			result := isDebInstalled("fastfetch")

			Expect(result).To(BeTrue())
		})

		It("should find executable by checking common paths directly", func() {
			// Fail both which commands but succeed on test -x
			mockCommandExec.FailingCommands["which fastfetch"] = true
			mockCommandExec.FailingCommands["export PATH=$PATH:/usr/bin:/usr/local/bin:/bin && which fastfetch"] = true

			result := isDebInstalled("fastfetch")

			Expect(result).To(BeTrue())
		})

		It("should find executable by version check with extended PATH", func() {
			// Fail all which and test commands but succeed on version check
			mockCommandExec.FailingCommands["which fastfetch"] = true
			mockCommandExec.FailingCommands["export PATH=$PATH:/usr/bin:/usr/local/bin:/bin && which fastfetch"] = true
			mockCommandExec.FailingCommands["test -x /usr/bin/fastfetch"] = true
			mockCommandExec.FailingCommands["test -x /usr/local/bin/fastfetch"] = true
			mockCommandExec.FailingCommands["test -x /bin/fastfetch"] = true

			result := isDebInstalled("fastfetch")

			Expect(result).To(BeTrue())
		})

		It("should return false when command is not found anywhere", func() {
			// Set all possible commands to fail
			failingCommands := []string{
				"which not-found-command",
				"export PATH=$PATH:/usr/bin:/usr/local/bin:/bin && which not-found-command",
				"test -x /usr/bin/not-found-command",
				"test -x /usr/local/bin/not-found-command",
				"test -x /bin/not-found-command",
				"export PATH=$PATH:/usr/bin:/usr/local/bin:/bin && (not-found-command --version 2>/dev/null || not-found-command --help 2>/dev/null || false)",
			}

			for _, cmd := range failingCommands {
				mockCommandExec.FailingCommands[cmd] = true
			}

			result := isDebInstalled("not-found-command")

			Expect(result).To(BeFalse())
		})

		It("should handle different command names properly", func() {
			// Default behavior should succeed for most commands
			result := isDebInstalled("custom-tool")

			Expect(result).To(BeTrue())
		})

		It("should verify actual command execution paths", func() {
			// Test the command and verify it was called
			mockCommandExec.Commands = []string{} // Reset commands

			result := isDebInstalled("test-app")

			Expect(result).To(BeTrue())
			// Should have at least attempted to run which command
			Expect(mockCommandExec.Commands).To(ContainElement("which test-app"))
		})

		It("should try multiple fallback methods when primary method fails", func() {
			// Make which command fail to force fallback methods
			mockCommandExec.FailingCommands["which fallback-test"] = true
			mockCommandExec.Commands = []string{} // Reset commands

			result := isDebInstalled("fallback-test")

			Expect(result).To(BeTrue())
			// Should have tried multiple methods
			Expect(len(mockCommandExec.Commands)).To(BeNumerically(">", 1))
		})
	})
})
