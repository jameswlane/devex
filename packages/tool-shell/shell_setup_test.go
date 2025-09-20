package main_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	main "github.com/jameswlane/devex/packages/tool-shell"
)

var _ = Describe("Shell Setup", func() {
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

	Describe("getShellConfigFile", func() {
		Context("with valid shells", func() {
			It("should return correct path for bash", func() {
				path := plugin.GetShellConfigFile("bash", "/home/test")
				Expect(path).To(Equal("/home/test/.bashrc"))
			})

			It("should return correct path for zsh", func() {
				path := plugin.GetShellConfigFile("zsh", "/home/test")
				Expect(path).To(Equal("/home/test/.zshrc"))
			})

			It("should return correct path for fish", func() {
				path := plugin.GetShellConfigFile("fish", "/home/test")
				Expect(path).To(Equal("/home/test/.config/fish/config.fish"))
			})
		})

		Context("with invalid shells", func() {
			It("should return empty string for unsupported shell", func() {
				path := plugin.GetShellConfigFile("powershell", "/home/test")
				Expect(path).To(Equal(""))
			})
		})
	})

	Describe("getShellConfigs", func() {
		Context("for bash", func() {
			It("should return bash-specific configurations", func() {
				configs := plugin.GetShellConfigs("bash")

				Expect(len(configs)).To(BeNumerically(">", 5))

				// Check for essential bash configurations
				configString := strings.Join(configs, "\n")
				Expect(configString).To(ContainSubstring("HISTSIZE"))
				Expect(configString).To(ContainSubstring("HISTFILESIZE"))
				Expect(configString).To(ContainSubstring("CLICOLOR"))
				Expect(configString).To(ContainSubstring("alias ll="))
			})
		})

		Context("for zsh", func() {
			It("should return zsh-specific configurations", func() {
				configs := plugin.GetShellConfigs("zsh")

				Expect(len(configs)).To(BeNumerically(">", 5))

				configString := strings.Join(configs, "\n")
				Expect(configString).To(ContainSubstring("HISTSIZE"))
				Expect(configString).To(ContainSubstring("SAVEHIST"))
				Expect(configString).To(ContainSubstring("autoload -U colors"))
				Expect(configString).To(ContainSubstring("HIST_IGNORE_DUPS"))
			})
		})

		Context("for fish", func() {
			It("should return fish-specific configurations", func() {
				configs := plugin.GetShellConfigs("fish")

				Expect(len(configs)).To(BeNumerically(">", 3))

				configString := strings.Join(configs, "\n")
				Expect(configString).To(ContainSubstring("fish_color_command"))
				Expect(configString).To(ContainSubstring("fish_color_error"))
				Expect(configString).To(ContainSubstring("alias ll"))
			})
		})

		Context("for unsupported shells", func() {
			It("should return empty array for unknown shell", func() {
				configs := plugin.GetShellConfigs("powershell")
				Expect(configs).To(BeEmpty())
			})
		})
	})

	Describe("Shell Configuration Validation", func() {
		Context("configuration safety", func() {
			It("should not contain dangerous commands in bash configs", func() {
				configs := plugin.GetShellConfigs("bash")

				for _, config := range configs {
					Expect(config).ToNot(ContainSubstring("rm -rf"))
					Expect(config).ToNot(ContainSubstring("curl http"))
					Expect(config).ToNot(ContainSubstring("wget http"))
					Expect(config).ToNot(ContainSubstring("nc -l"))
				}
			})

			It("should not contain dangerous commands in zsh configs", func() {
				configs := plugin.GetShellConfigs("zsh")

				for _, config := range configs {
					Expect(config).ToNot(ContainSubstring("rm -rf"))
					Expect(config).ToNot(ContainSubstring("curl http"))
					Expect(config).ToNot(ContainSubstring("wget http"))
					Expect(config).ToNot(ContainSubstring("nc -l"))
				}
			})

			It("should not contain dangerous commands in fish configs", func() {
				configs := plugin.GetShellConfigs("fish")

				for _, config := range configs {
					Expect(config).ToNot(ContainSubstring("rm -rf"))
					Expect(config).ToNot(ContainSubstring("curl http"))
					Expect(config).ToNot(ContainSubstring("wget http"))
					Expect(config).ToNot(ContainSubstring("nc -l"))
				}
			})
		})

		Context("alias safety", func() {
			It("should only contain safe aliases", func() {
				allConfigs := [][]string{
					plugin.GetShellConfigs("bash"),
					plugin.GetShellConfigs("zsh"),
					plugin.GetShellConfigs("fish"),
				}

				for _, configs := range allConfigs {
					for _, config := range configs {
						if strings.HasPrefix(config, "alias ") {
							// Aliases should be reasonable navigation and listing commands
							// Allow both bash/zsh style and fish style alias syntax
							// bash/zsh: alias name='command', fish: alias name 'command'
							Expect(config).To(MatchRegexp(`alias [a-zA-Z0-9\.]+\s*=?'?(ls|cd)`))
						}
					}
				}
			})
		})
	})

	Describe("handleSetup Integration Tests", func() {
		Context("when shell detection succeeds", func() {
			It("should set up shell configuration", func() {
				Skip("Integration test - requires shell environment and file system")
			})
		})

		Context("when shell detection fails", func() {
			It("should return helpful error message", func() {
				Skip("Integration test - requires controlled shell environment")
			})
		})
	})

	Describe("File Operations", func() {
		Context("creating configuration files", func() {
			It("should handle file creation errors gracefully", func() {
				Skip("Integration test - requires file system manipulation")
			})
		})

		Context("updating existing configurations", func() {
			It("should preserve existing content while adding DevEx configs", func() {
				Skip("Integration test - requires file system operations")
			})
		})
	})
})
