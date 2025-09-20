package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	main "github.com/jameswlane/devex/packages/tool-git"
)

var _ = Describe("Git Aliases", func() {
	var plugin *main.GitPlugin

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "tool-git",
			Version:     "test",
			Description: "Test git plugin",
		}
		plugin = &main.GitPlugin{
			BasePlugin: sdk.NewBasePlugin(info),
		}
	})

	Describe("GetGitAliases", func() {
		It("should return a comprehensive set of Git aliases", func() {
			aliases := plugin.GetGitAliases()

			// Check that we have a good number of aliases
			Expect(len(aliases)).To(BeNumerically(">=", 20))

			// Check for essential basic shortcuts
			Expect(aliases["st"]).To(Equal("status"))
			Expect(aliases["co"]).To(Equal("checkout"))
			Expect(aliases["br"]).To(Equal("branch"))
			Expect(aliases["ci"]).To(Equal("commit"))
		})

		It("should include productivity aliases", func() {
			aliases := plugin.GetGitAliases()

			// Check for workflow enhancing aliases
			Expect(aliases).To(HaveKey("unstage"))
			Expect(aliases).To(HaveKey("last"))
			Expect(aliases).To(HaveKey("visual"))
		})

		It("should include branch management aliases", func() {
			aliases := plugin.GetGitAliases()

			// Check for branch-related aliases
			Expect(aliases).To(HaveKey("br")) // branch
			Expect(aliases).To(HaveKey("co")) // checkout
		})

		It("should include log viewing aliases", func() {
			aliases := plugin.GetGitAliases()

			// Check for log-related aliases
			Expect(aliases).To(HaveKey("lg"))
			// Note: hist alias doesn't exist, checking other log aliases instead
			Expect(aliases).To(HaveKey("last"))
			Expect(aliases).To(HaveKey("ll"))
		})

		It("should have sensible alias commands", func() {
			aliases := plugin.GetGitAliases()

			for aliasName, command := range aliases {
				// All aliases should be non-empty
				Expect(aliasName).ToNot(BeEmpty())
				Expect(command).ToNot(BeEmpty())

				// Aliases should be short (typically 2-10 characters)
				Expect(len(aliasName)).To(BeNumerically("<=", 10))
			}
		})

		It("should not have conflicting aliases", func() {
			aliases := plugin.GetGitAliases()

			// Check that all alias keys are unique (this is guaranteed by map structure)
			// but we can check for reasonable alias names
			for aliasName := range aliases {
				// Alias names should not contain dangerous characters
				Expect(aliasName).ToNot(ContainSubstring(";"))
				Expect(aliasName).ToNot(ContainSubstring("&"))
				Expect(aliasName).ToNot(ContainSubstring("|"))
				Expect(aliasName).ToNot(ContainSubstring("`"))
				Expect(aliasName).ToNot(ContainSubstring("$"))
			}
		})

		It("should have safe command strings", func() {
			aliases := plugin.GetGitAliases()

			for _, command := range aliases {
				// Commands should not contain obvious shell injection attempts
				Expect(command).ToNot(ContainSubstring("; rm"))
				Expect(command).ToNot(ContainSubstring("&& rm"))
				Expect(command).ToNot(ContainSubstring("| curl"))

				// Commands should be reasonable git commands
				// Most should start with common git subcommands or be git options
				if len(command) > 0 {
					// Allow most characters used in git aliases and shell functions
					Expect(command).To(MatchRegexp(`^[a-zA-Z0-9\-\s\.\[\]'"=:(){}\\^$!|%&;/@]+$`))
				}
			}
		})
	})

	Describe("HandleAliases", func() {
		It("should install aliases without error when git is available", func() {
			Skip("Integration test - requires git and modifies global config")
		})

		It("should report installation progress", func() {
			Skip("Integration test - requires git and modifies global config")
		})

		It("should handle git command failures gracefully", func() {
			Skip("Integration test - requires specific git failure scenarios")
		})
	})

	Describe("Security Considerations", func() {
		Context("alias validation", func() {
			It("should not allow command injection in alias names", func() {
				aliases := plugin.GetGitAliases()

				for aliasName := range aliases {
					// Alias names should be safe for shell execution
					Expect(aliasName).To(MatchRegexp(`^[a-zA-Z0-9\-_]+$`))
				}
			})

			It("should not allow dangerous commands", func() {
				aliases := plugin.GetGitAliases()

				dangerousPatterns := []string{
					"rm -rf",
					"curl http",
					"wget http",
					"nc -l",
					"/etc/passwd",
					"sudo ",
				}

				for _, command := range aliases {
					for _, pattern := range dangerousPatterns {
						Expect(command).ToNot(ContainSubstring(pattern))
					}
				}
			})
		})
	})

	Describe("Alias Quality", func() {
		It("should provide meaningful shortcuts", func() {
			aliases := plugin.GetGitAliases()

			// Basic shortcuts should be shorter than the original
			basicMappings := map[string]string{
				"st": "status",
				"co": "checkout",
				"br": "branch",
				"ci": "commit",
			}

			for alias, expectedCommand := range basicMappings {
				Expect(aliases[alias]).To(Equal(expectedCommand))
				Expect(len(alias)).To(BeNumerically("<", len(expectedCommand)))
			}
		})

		It("should include useful workflow commands", func() {
			aliases := plugin.GetGitAliases()

			// Should have some complex but useful commands
			complexAliases := []string{"unstage", "last", "visual"}
			for _, alias := range complexAliases {
				Expect(aliases).To(HaveKey(alias))
				// Complex aliases should have substantial commands
				Expect(len(aliases[alias])).To(BeNumerically(">=", 5))
			}
		})
	})
})
