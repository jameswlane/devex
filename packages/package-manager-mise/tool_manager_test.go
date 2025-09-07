package main_test

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
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

		It("should update specific tools when provided", func() {
			Skip("Integration test - requires mise to be installed")
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
			It("should check if tool is installed", func() {
				Skip("Integration test - requires mise to be installed")
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
