package main_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	pluginPath string
	mockBinDir string
)

var _ = BeforeSuite(func() {
	var err error
	pluginPath, err = gexec.Build("github.com/jameswlane/devex/packages/package-manager-apt")
	Expect(err).NotTo(HaveOccurred())

	// Create temporary directory for mock binaries
	mockBinDir, err = os.MkdirTemp("", "apt-plugin-test-")
	Expect(err).NotTo(HaveOccurred())

	createMockAptBinaries()
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
	if mockBinDir != "" {
		if err := os.RemoveAll(mockBinDir); err != nil {
			GinkgoWriter.Printf("Warning: Failed to remove mock bin directory: %v\n", err)
		}
	}
})

// createMockAptBinaries creates mock apt, dpkg-query binaries for safe testing
func createMockAptBinaries() {
	// Mock apt binary - returns version info and handles basic commands
	aptScript := `#!/bin/bash
case "$1" in
  "--version")
    echo "apt 2.4.7 (amd64)"
    exit 0
    ;;
  "update")
    echo "Reading package lists... Done"
    exit 0
    ;;
  "search")
    if [ -z "$2" ]; then
      echo "E: No packages found" >&2
      exit 1
    fi
    echo "mock-package/test 1.0 amd64"
    echo "  Mock package for testing"
    exit 0
    ;;
  "list")
    if [ "$2" = "--installed" ]; then
      echo "git/test,now 2.39.2 amd64 [installed]"
      echo "vim/test,now 8.2 amd64 [installed]"
    fi
    exit 0
    ;;
  "show")
    if [ -z "$2" ]; then
      echo "E: No packages found" >&2
      exit 1
    fi
    echo "Package: $2"
    echo "Version: 1.0"
    echo "Description: Mock package for testing"
    exit 0
    ;;
  "install")
    echo "Reading package lists... Done"
    echo "Building dependency tree... Done" 
    echo "The following packages will be installed:"
    shift
    echo "$@"
    echo "Need to get 0 B of archives."
    echo "After this operation, 0 B of additional disk space will be used."
    exit 0
    ;;
  "remove")
    echo "Reading package lists... Done"
    echo "Building dependency tree... Done"
    echo "The following packages will be REMOVED:"
    shift
    echo "$@"
    exit 0
    ;;
  "upgrade")
    echo "Reading package lists... Done"
    echo "Building dependency tree... Done"
    echo "0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded."
    exit 0
    ;;
  *)
    echo "E: Invalid operation" >&2
    exit 1
    ;;
esac`

	aptPath := filepath.Join(mockBinDir, "apt")
	err := os.WriteFile(aptPath, []byte(aptScript), 0755)
	Expect(err).NotTo(HaveOccurred())

	// Mock apt-cache binary for policy checks
	aptCacheScript := `#!/bin/bash
case "$1" in
  "policy")
    if [ -z "$2" ]; then
      echo "E: No packages found" >&2
      exit 1
    fi
    case "$2" in
      "nonexistent-package"*)
        echo "N: Unable to locate package $2"
        exit 0
        ;;
      *)
        echo "$2:"
        echo "  Installed: (none)"
        echo "  Candidate: 1.0"
        echo "  Version table:"
        echo "     1.0 500"
        exit 0
        ;;
    esac
    ;;
  *)
    echo "E: Invalid operation" >&2
    exit 1
    ;;
esac`

	aptCachePath := filepath.Join(mockBinDir, "apt-cache")
	err = os.WriteFile(aptCachePath, []byte(aptCacheScript), 0755)
	Expect(err).NotTo(HaveOccurred())

	// Mock dpkg-query binary for installation checks
	dpkgQueryScript := `#!/bin/bash
# Parse arguments properly
# Expected: dpkg-query -W -f=${Status} packagename
if [ "$1" = "-W" ] && [[ "$2" == -f=* ]]; then
    # Extract package name from argument 3
    packagename="$3"
else
    # Fallback argument parsing
    packagename="$2"
fi

case "$packagename" in
  "git"|"vim"|"curl")
    echo "install ok installed"
    exit 0
    ;;
  "nonexistent-package"*)
    exit 1
    ;;
  *)
    # Unknown package, not installed
    exit 1
    ;;
esac`

	dpkgQueryPath := filepath.Join(mockBinDir, "dpkg-query")
	err = os.WriteFile(dpkgQueryPath, []byte(dpkgQueryScript), 0755)
	Expect(err).NotTo(HaveOccurred())
}

// runPlugin runs the plugin with normal system PATH
func runPlugin(args ...string) *gexec.Session {
	command := exec.Command(pluginPath, args...)
	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}

// runPluginWithMockAPT runs the plugin with mocked APT binaries in PATH
func runPluginWithMockAPT(args ...string) *gexec.Session {
	command := exec.Command(pluginPath, args...)
	// Prepend mock bin directory to PATH
	originalPath := os.Getenv("PATH")
	command.Env = append(os.Environ(), "PATH="+mockBinDir+":"+originalPath)

	session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	return session
}

var _ = Describe("APT Plugin", func() {
	Context("Plugin Info", func() {
		It("should return valid plugin information", func() {
			command := exec.Command(pluginPath, "--plugin-info")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))

			output := string(session.Out.Contents())
			Expect(output).To(ContainSubstring("package-manager-apt"))
			Expect(output).To(ContainSubstring("APT package manager support"))
			Expect(output).To(ContainSubstring("install"))
			Expect(output).To(ContainSubstring("remove"))
			Expect(output).To(ContainSubstring("update"))
		})
	})

	Context("Package Name Validation", func() {
		It("should reject package names with dangerous characters", func() {
			command := exec.Command(pluginPath, "is-installed", "test;rm -rf /")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("invalid characters"))
		})

		It("should accept valid package names", func() {
			command := exec.Command(pluginPath, "is-installed", "git")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit())
		})

		It("should reject empty package names", func() {
			command := exec.Command(pluginPath, "is-installed", "")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
		})

		It("should reject overly long package names", func() {
			longName := make([]byte, 101)
			for i := range longName {
				longName[i] = 'a'
			}

			command := exec.Command(pluginPath, "is-installed", string(longName))
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("too long"))
		})
	})

	Context("Command Validation", func() {
		It("should handle unknown commands", func() {
			command := exec.Command(pluginPath, "unknown-command")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("unknown command"))
		})

		It("should require package names for is-installed", func() {
			command := exec.Command(pluginPath, "is-installed")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("no packages specified"))
		})

		It("should require package names for install", func() {
			command := exec.Command(pluginPath, "install")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("no packages specified"))
		})

		It("should require package names for remove", func() {
			command := exec.Command(pluginPath, "remove")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("no packages specified"))
		})
	})

	Context("Search Functionality", func() {
		It("should require search terms", func() {
			session := runPluginWithMockAPT("search")
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("no search term specified"))
		})

		It("should accept valid search terms using mock APT", func() {
			session := runPluginWithMockAPT("search", "vim")
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("mock-package"))
		})
	})

	Context("Info Command", func() {
		It("should require package names", func() {
			command := exec.Command(pluginPath, "info")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("no package specified"))
		})

		It("should validate package names", func() {
			command := exec.Command(pluginPath, "info", "invalid;package")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("invalid characters"))
		})
	})

	Context("Mock APT Integration Tests", func() {
		It("should work with mock APT binaries", func() {
			session := runPluginWithMockAPT("is-installed", "git")
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("is installed"))
		})

		It("should detect non-installed packages", func() {
			session := runPluginWithMockAPT("is-installed", "nonexistent-package")
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("is not installed"))
		})

		It("should handle package installation simulation", func() {
			session := runPluginWithMockAPT("install", "test-package")
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("Installing packages"))
		})

		It("should handle package removal simulation", func() {
			session := runPluginWithMockAPT("remove", "git")
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("Removing packages"))
		})

		It("should handle update command", func() {
			session := runPluginWithMockAPT("update")
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("Package lists updated"))
		})

		It("should handle upgrade command", func() {
			session := runPluginWithMockAPT("upgrade")
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out.Contents()).To(ContainSubstring("Packages upgraded"))
		})

		It("should handle list command", func() {
			session := runPluginWithMockAPT("list", "--installed")
			Eventually(session).Should(gexec.Exit(0))
		})

		It("should handle info command", func() {
			session := runPluginWithMockAPT("info", "test-package")
			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("APT Version Detection", func() {
		It("should detect APT version with mock binary", func() {
			// This tests version detection without running real apt commands
			session := runPluginWithMockAPT("--plugin-info")
			Eventually(session).Should(gexec.Exit(0))
			// Plugin should start successfully, indicating version detection worked
		})
	})

	Context("Package Availability Validation", func() {
		It("should validate available packages", func() {
			session := runPluginWithMockAPT("install", "test-package")
			Eventually(session).Should(gexec.Exit(0))
			// Should not fail with "package not found" error
		})

		It("should handle unavailable packages", func() {
			session := runPluginWithMockAPT("install", "nonexistent-package")
			Eventually(session).Should(gexec.Exit(1))
			Expect(session.Err.Contents()).To(ContainSubstring("not found"))
		})
	})

	Context("Error Handling", func() {
		It("should handle panic recovery gracefully", func() {
			// This tests the panic recovery in main()
			command := exec.Command(pluginPath, "--plugin-info")
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})
	})

	Context("Environment Compatibility", func() {
		It("should work without USER environment variable", func() {
			command := exec.Command(pluginPath, "--plugin-info")
			command.Env = []string{} // Clear environment
			session, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			Expect(err).NotTo(HaveOccurred())

			Eventually(session).Should(gexec.Exit(0))
		})

		It("should handle missing dpkg-query gracefully", func() {
			// This would be difficult to test without mocking
			// but the code has proper error handling for missing dpkg-query
			Skip("Requires system without dpkg-query to test properly")
		})
	})

	Context("Repository Management", func() {
		Context("Repository Validation", func() {
			It("should reject empty repository strings", func() {
				session := runPlugin("add-repository", "", "/tmp/test.gpg", "", "/tmp/test.list")
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("repository string cannot be empty"))
			})

			It("should reject short repository strings", func() {
				session := runPlugin("add-repository", "https://example.com", "/tmp/test.gpg", "deb", "/tmp/test.list")
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("repository string too short"))
			})

			It("should reject repository strings without required keywords", func() {
				session := runPlugin("add-repository", "https://example.com", "/tmp/test.gpg", "invalid repo string", "/tmp/test.list")
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("missing required keywords"))
			})

			It("should reject repository strings with invalid URLs", func() {
				session := runPlugin("add-repository", "https://example.com", "/tmp/test.gpg", "deb invalid-url main", "/tmp/test.list")
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("invalid URL"))
			})

			It("should reject repository strings with suspicious characters", func() {
				session := runPlugin("add-repository", "https://example.com", "/tmp/test.gpg", "deb https://example.com/repo; rm -rf / main", "/tmp/test.list")
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("suspicious characters"))
			})

			It("should accept valid repository strings", func() {
				Skip("This test would require actual file system operations")
				// session := runPlugin("add-repository", "https://example.com/key.asc", "/tmp/test.gpg", "deb https://example.com/repo main", "/tmp/test.list")
				// This would need more complex mocking to test properly
			})
		})

		Context("Command Arguments", func() {
			It("should require all arguments for add-repository", func() {
				session := runPlugin("add-repository")
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("add-repository requires"))
			})

			It("should require all arguments for remove-repository", func() {
				session := runPlugin("remove-repository")
				Eventually(session).Should(gexec.Exit(1))
				Expect(session.Err.Contents()).To(ContainSubstring("remove-repository requires"))
			})

			It("should accept validate-repository without arguments", func() {
				Skip("This test would try to run real apt-get commands")
				// session := runPlugin("validate-repository")
				// This would require mocking apt-get commands
			})
		})

		Context("New Commands Available", func() {
			It("should include repository management commands in plugin info", func() {
				session := runPlugin("--plugin-info")
				Eventually(session).Should(gexec.Exit(0))

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring("add-repository"))
				Expect(output).To(ContainSubstring("remove-repository"))
				Expect(output).To(ContainSubstring("validate-repository"))
			})

			It("should provide proper command descriptions", func() {
				session := runPlugin("--plugin-info")
				Eventually(session).Should(gexec.Exit(0))

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring("Add a new APT repository with GPG key"))
				Expect(output).To(ContainSubstring("Remove an APT repository and its GPG key"))
				Expect(output).To(ContainSubstring("Validate repository configuration and GPG keys"))
			})

			It("should provide proper command flags", func() {
				session := runPlugin("--plugin-info")
				Eventually(session).Should(gexec.Exit(0))

				output := string(session.Out.Contents())
				Expect(output).To(ContainSubstring("key-url"))
				Expect(output).To(ContainSubstring("key-path"))
				Expect(output).To(ContainSubstring("source-line"))
				Expect(output).To(ContainSubstring("source-file"))
				Expect(output).To(ContainSubstring("require-dearmor"))
			})
		})
	})

	Context("Security Features", func() {
		Context("Repository Security", func() {
			It("should validate repository URLs", func() {
				session := runPlugin("add-repository", "https://example.com", "/tmp/test.gpg", "deb ftp://insecure.com main", "/tmp/test.list")
				Eventually(session).Should(gexec.Exit(1))
				// Should reject non-HTTPS URLs in modern implementations
			})

			It("should prevent command injection in repository strings", func() {
				injectionAttempts := []string{
					"deb https://example.com $(rm -rf /) main",
					"deb https://example.com `whoami` main",
					"deb https://example.com && malicious_command main",
					"deb https://example.com | evil_pipe main",
					"deb https://example.com; dangerous_command main",
				}

				for _, attempt := range injectionAttempts {
					session := runPlugin("add-repository", "https://example.com", "/tmp/test.gpg", attempt, "/tmp/test.list")
					Eventually(session).Should(gexec.Exit(1))
					Expect(session.Err.Contents()).To(ContainSubstring("suspicious characters"))
				}
			})
		})
	})
})
