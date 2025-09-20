package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
)

func TestAPTPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APT Plugin Suite")
}

var _ = Describe("APT Package Manager", func() {
	var plugin *main.APTInstaller

	BeforeEach(func() {
		plugin = main.NewAPTPlugin()
	})

	Describe("Execute", func() {
		Context("with valid commands", func() {
			It("should route install command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route remove command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route update command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route upgrade command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route search command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route list command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route info command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route is-installed command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route add-repository command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route remove-repository command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})

			It("should route validate-repository command correctly", func() {
				Skip("Integration test - requires actual APT system interaction")
			})
		})

		Context("with invalid commands", func() {
			It("should return error for unknown command", func() {
				err := plugin.Execute("invalid-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
			})
		})
	})

	Describe("Command Validation", func() {
		Context("install command", func() {
			It("should reject empty package list", func() {
				err := plugin.Execute("install", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})

			It("should reject packages with dangerous characters", func() {
				err := plugin.Execute("install", []string{"test;rm -rf /"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid characters"))
			})

			It("should reject empty package names", func() {
				err := plugin.Execute("install", []string{""})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package name cannot be empty"))
			})

			It("should reject excessively long package names", func() {
				longName := make([]byte, 200)
				for i := range longName {
					longName[i] = 'a'
				}
				err := plugin.Execute("install", []string{string(longName)})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package name too long"))
			})
		})

		Context("remove command", func() {
			It("should reject empty package list", func() {
				err := plugin.Execute("remove", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})

			It("should reject packages with dangerous characters", func() {
				err := plugin.Execute("remove", []string{"test;rm -rf /"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid characters"))
			})
		})

		Context("is-installed command", func() {
			It("should reject empty package list", func() {
				err := plugin.Execute("is-installed", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})

			It("should reject packages with dangerous characters", func() {
				err := plugin.Execute("is-installed", []string{"test;rm -rf /"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid characters"))
			})
		})

		Context("info command", func() {
			It("should reject empty package name", func() {
				err := plugin.Execute("info", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no package specified"))
			})

			It("should reject packages with dangerous characters", func() {
				err := plugin.Execute("info", []string{"test;rm -rf /"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid characters"))
			})
		})

		Context("add-repository command", func() {
			It("should reject insufficient arguments", func() {
				err := plugin.Execute("add-repository", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("add-repository requires"))
			})

			It("should reject incomplete arguments", func() {
				err := plugin.Execute("add-repository", []string{"key-url", "key-path"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("add-repository requires"))
			})

			It("should validate repository strings with malicious content", func() {
				err := plugin.Execute("add-repository", []string{
					"https://example.com/key.gpg",
					"/tmp/test.gpg",
					"deb https://example.com/repo; rm -rf / main",
					"/tmp/test.list",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious characters"))
			})
		})

		Context("remove-repository command", func() {
			It("should reject insufficient arguments", func() {
				err := plugin.Execute("remove-repository", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("remove-repository requires"))
			})

			It("should reject incomplete arguments", func() {
				err := plugin.Execute("remove-repository", []string{"source-file"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("remove-repository requires"))
			})
		})
	})

	Describe("Security Features", func() {
		Context("Package Name Validation", func() {
			dangerousPackageNames := []string{
				"git;rm -rf /",
				"git|evil_command",
				"git&malicious",
				"git$(whoami)",
				"git`whoami`",
				"git(test)",
				"git{test}",
				"git[test]",
				"git<file",
				"git>file",
				"git*",
				"git?",
				"git~test",
				"git test",
				"git\ttest",
				"git\ntest",
			}

			for _, dangerous := range dangerousPackageNames {
				func(pkg string) {
					It("should reject package name with dangerous characters: "+pkg, func() {
						err := plugin.Execute("install", []string{pkg})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("invalid characters"))
					})
				}(dangerous)
			}
		})

		Context("Repository String Validation", func() {
			maliciousRepoStrings := []string{
				"deb https://example.com/repo; rm -rf / main",
				"deb https://example.com/repo | evil_command main",
				"deb https://example.com/repo && malicious_command main",
				"deb https://example.com/repo $(whoami) main",
				"deb https://example.com/repo `whoami` main",
			}

			for _, malicious := range maliciousRepoStrings {
				func(repo string) {
					It("should reject repository string with injection attempt: "+repo, func() {
						err := plugin.Execute("add-repository", []string{
							"https://example.com/key.gpg",
							"/tmp/test.gpg",
							repo,
							"/tmp/test.list",
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("suspicious characters"))
					})
				}(malicious)
			}

			It("should reject repository strings without required keywords", func() {
				err := plugin.Execute("add-repository", []string{
					"https://example.com/key.gpg",
					"/tmp/test.gpg",
					"invalid-url main",
					"/tmp/test.list",
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("missing required keywords"))
			})
		})
	})
})
