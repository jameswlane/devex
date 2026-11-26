package main_test

import (
	"context"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Tool Manager", func() {
	var plugin *main.MisePlugin

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "package-manager-mise",
			Version:     "test",
			Description: "Test mise plugin",
		}
		plugin = &main.MisePlugin{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "mise"),
		}
		// Initialize logger directly to prevent nil pointer dereference
		plugin.InitLogger(sdk.NewDefaultLogger(false))
	})

	Describe("HandleInstall", func() {
		Context("with valid tools", func() {
			It("should validate tool specification before installation", func() {
				// This would typically mock the actual installation
				// For unit tests, we focus on the validation logic
				Skip("Integration test - requires mise to be installed")
			})

			It("should handle multiple tool installations", func() {
				Skip("Integration test - requires mise to be installed")
			})
		})

		Context("with invalid tools", func() {
			It("should return error for empty tool list", func() {
				err := plugin.HandleInstall(context.Background(), []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no tools specified"))
			})

			It("should return error for invalid tool specification", func() {
				err := plugin.HandleInstall(context.Background(), []string{"tool;echo hacked"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tool specification"))
			})
		})

		Context("with environment variables", func() {
			It("should respect MISE_LOCAL environment variable", func() {
				err := os.Setenv("MISE_LOCAL", "1")
				Expect(err).ToNot(HaveOccurred())
				defer func() {
					_ = os.Unsetenv("MISE_LOCAL")
				}()

				// The function should use --local flag when MISE_LOCAL=1
				// This would be tested in integration tests
				Skip("Integration test - requires mise to be installed")
			})
		})
	})

	Describe("HandleRemove", func() {
		Context("with valid tools", func() {
			It("should validate tool before removal", func() {
				Skip("Integration test - requires mise to be installed")
			})
		})

		Context("with invalid tools", func() {
			It("should return error for empty tool list", func() {
				err := plugin.HandleRemove(context.Background(), []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no tools specified"))
			})

			It("should return error for invalid tool specification", func() {
				err := plugin.HandleRemove(context.Background(), []string{"tool;rm -rf /"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tool specification"))
			})
		})
	})

	Describe("HandleUpdate", func() {
		It("should update all tools when no specific tools provided", func() {
			Skip("Integration test - requires mise to be installed")
		})

		It("should return error for invalid tool specification", func() {
			err := plugin.HandleUpdate(context.Background(), []string{"node;echo hacked"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid tool specification"))
		})
	})

	Describe("HandleSearch", func() {
		Context("with valid search terms", func() {
			It("should search for tools", func() {
				Skip("Integration test - requires mise to be installed")
			})
		})

		Context("with invalid search terms", func() {
			It("should return error for empty search term", func() {
				err := plugin.HandleSearch(context.Background(), []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no search term specified"))
			})

			It("should validate search term for dangerous characters", func() {
				err := plugin.HandleSearch(context.Background(), []string{"node;echo"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid search term"))
			})
		})
	})

	Describe("HandleList", func() {
		It("should list installed tools", func() {
			Skip("Integration test - requires mise to be installed")
		})

		It("should handle --installed flag", func() {
			Skip("Integration test - requires mise to be installed")
		})

		It("should handle --available flag", func() {
			Skip("Integration test - requires mise to be installed")
		})
	})

	Describe("HandleIsInstalled", func() {
		Context("with valid tool", func() {
			It("returns nil when mise where reports an install path", func() {
				recorder := sdktest.NewRecorder(GinkgoT(), "mise")
				recorder.Script("mise", []string{"where", "node@24"}, sdktest.Response{
					ExitCode: 0,
					Stdout:   "/home/devex-test-user/.local/share/mise/installs/node/24.1.0\n",
				})

				Expect(plugin.HandleIsInstalled(context.Background(), []string{"node@24"})).To(Succeed())
				Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
					{Args: []string{"where", "node@24"}},
				}))
			})

			It("passes backend tool specs through install, is-installed, and remove", func() {
				recorder := sdktest.NewRecorder(GinkgoT(), "mise")
				GinkgoT().Setenv("MISE_LOCAL", "0")
				recorder.Script("mise", []string{"where", "aqua:aaif-goose/goose"}, sdktest.Response{
					ExitCode: 0,
					Stdout:   "/home/devex-test-user/.local/share/mise/installs/aqua-aaif-goose-goose/1.61.0\n",
				})

				Expect(plugin.HandleInstall(context.Background(), []string{"aqua:aaif-goose/goose@latest"})).To(Succeed())
				Expect(plugin.HandleIsInstalled(context.Background(), []string{"aqua:aaif-goose/goose"})).To(Succeed())
				Expect(plugin.HandleRemove(context.Background(), []string{"aqua:aaif-goose/goose"})).To(Succeed())
				Expect(plugin.HandleInstall(context.Background(), []string{"pipx:aider-chat[python=3.12]@latest"})).To(Succeed())
				Expect(plugin.HandleRemove(context.Background(), []string{"pipx:aider-chat"})).To(Succeed())

				Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
					{Args: []string{"use", "--global", "aqua:aaif-goose/goose@latest"}},
					{Args: []string{"where", "aqua:aaif-goose/goose"}},
					{Args: []string{"unuse", "--global", "aqua:aaif-goose/goose"}},
					{Args: []string{"use", "--global", "pipx:aider-chat[python=3.12]@latest"}},
					{Args: []string{"unuse", "--global", "pipx:aider-chat"}},
				}))
			})

			It("passes a version alias such as node@lts through to mise where", func() {
				recorder := sdktest.NewRecorder(GinkgoT(), "mise")
				recorder.Script("mise", []string{"where", "node@lts"}, sdktest.Response{
					ExitCode: 0,
					Stdout:   "/home/devex-test-user/.local/share/mise/installs/node/24.1.0\n",
				})

				Expect(plugin.HandleIsInstalled(context.Background(), []string{"node@lts"})).To(Succeed())
				Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
					{Args: []string{"where", "node@lts"}},
				}))
			})

			It("returns NotInstalledError when mise where exits non-zero for a missing tool", func() {
				recorder := sdktest.NewRecorder(GinkgoT(), "mise")
				recorder.Script("mise", []string{"where", "node@24"}, sdktest.Response{
					ExitCode: 1,
					Stderr:   "mise ERROR node@24 not installed\n",
				})

				err := plugin.HandleIsInstalled(context.Background(), []string{"node@24"})
				Expect(err).To(HaveOccurred())
				var notInstalled *sdk.NotInstalledError
				Expect(errors.As(err, &notInstalled)).To(BeTrue())
				Expect(notInstalled.Packages).To(Equal([]string{"node@24"}))
				Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
					{Args: []string{"where", "node@24"}},
				}))
			})

			It("returns a wrapped error when mise where fails for another reason", func() {
				recorder := sdktest.NewRecorder(GinkgoT(), "mise")
				recorder.Script("mise", []string{"where", "node@24"}, sdktest.Response{
					ExitCode: 1,
					Stderr:   "mise ERROR Invalid TOML in config file\n",
				})

				err := plugin.HandleIsInstalled(context.Background(), []string{"node@24"})
				Expect(err).To(HaveOccurred())
				var notInstalled *sdk.NotInstalledError
				Expect(errors.As(err, &notInstalled)).To(BeFalse())
				Expect(err.Error()).To(ContainSubstring("failed to check whether tool 'node@24' is installed"))
			})

			It("returns NotInstalledError when mise where prints empty output", func() {
				recorder := sdktest.NewRecorder(GinkgoT(), "mise")
				recorder.Script("mise", []string{"where", "node@24"}, sdktest.Response{
					ExitCode: 0,
					Stdout:   "   \n",
				})

				err := plugin.HandleIsInstalled(context.Background(), []string{"node@24"})
				Expect(err).To(HaveOccurred())
				var notInstalled *sdk.NotInstalledError
				Expect(errors.As(err, &notInstalled)).To(BeTrue())
				Expect(notInstalled.Packages).To(Equal([]string{"node@24"}))
			})

			It("returns a wrapped error when mise cannot be executed", func() {
				sdktest.NewRecorder(GinkgoT())

				err := plugin.HandleIsInstalled(context.Background(), []string{"node@24"})
				Expect(err).To(HaveOccurred())
				var notInstalled *sdk.NotInstalledError
				Expect(errors.As(err, &notInstalled)).To(BeFalse())
				Expect(err.Error()).To(ContainSubstring("failed to check whether tool 'node@24' is installed"))
			})
		})

		Context("with invalid tool", func() {
			It("should return error for empty tool", func() {
				err := plugin.HandleIsInstalled(context.Background(), []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no tool specified"))
			})

			It("should validate tool specification", func() {
				err := plugin.HandleIsInstalled(context.Background(), []string{"tool;echo"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tool specification"))
			})
		})
	})

	Describe("Error Handling", func() {
		It("should provide actionable error messages", func() {
			err := plugin.HandleInstall(context.Background(), []string{"tool;echo hacked"})
			Expect(err).To(HaveOccurred())
			// Error message should be clear and actionable
			Expect(err.Error()).To(ContainSubstring("invalid"))
		})
	})
})
