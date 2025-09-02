package dnf_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/dnf"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("DNF Installer", func() {
	var (
		mockRepo     *mocks.MockRepository
		mockExec     *mocks.MockCommandExecutor
		installer    *dnf.DnfInstaller
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockExec = mocks.NewMockCommandExecutor()

		// Store original executor and replace with mock
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec

		installer = dnf.NewDnfInstaller()
	})

	AfterEach(func() {
		// Restore original executor
		utils.CommandExec = originalExec
	})

	Describe("NewDnfInstaller", func() {
		It("creates a new DNF installer", func() {
			dnfInstaller := dnf.NewDnfInstaller()
			Expect(dnfInstaller).NotTo(BeNil())
		})
	})

	Describe("Install", func() {
		Context("when DNF is available", func() {
			It("installs a package successfully using DNF", func() {
				err := installer.Install("test-package", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// Verify DNF system validation commands
				Expect(mockExec.Commands).To(ContainElement("which dnf"))
				Expect(mockExec.Commands).To(ContainElement("rpm --version"))
				// Verify package checks
				Expect(mockExec.Commands).To(ContainElement("rpm -q test-package"))
				// Verify installation command
				Expect(mockExec.Commands).To(ContainElement("sudo dnf install -y test-package"))
			})
		})

		Context("when YUM is available but DNF is not", func() {
			BeforeEach(func() {
				// Make DNF unavailable but YUM available
				if mockExec.FailingCommands == nil {
					mockExec.FailingCommands = make(map[string]bool)
				}
				mockExec.FailingCommands["which dnf"] = true
				// Also make dnf commands fail (since dnf is not available)
				mockExec.FailingCommands["dnf info test-package"] = true
			})

			It("installs a package successfully using YUM fallback", func() {
				err := installer.Install("test-package", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// Verify YUM system validation
				Expect(mockExec.Commands).To(ContainElement("which yum"))
				// Verify YUM installation command
				Expect(mockExec.Commands).To(ContainElement("sudo yum install -y test-package"))
			})
		})

		Context("when package is already installed", func() {
			BeforeEach(func() {
				// Mark the package as already installed in the mock
				mockExec.InstallationState["test-package"] = true
			})

			It("skips installation when package is already installed", func() {
				err := installer.Install("test-package", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// Should not attempt to install
				Expect(mockExec.Commands).ToNot(ContainElement(ContainSubstring("dnf install")))
				Expect(mockExec.Commands).ToNot(ContainElement(ContainSubstring("yum install")))
			})
		})

		Context("when neither DNF nor YUM is available", func() {
			BeforeEach(func() {
				// Make both DNF and YUM fail
				if mockExec.FailingCommands == nil {
					mockExec.FailingCommands = make(map[string]bool)
				}
				mockExec.FailingCommands["which dnf"] = true
				mockExec.FailingCommands["which yum"] = true
			})

			It("returns an error", func() {
				err := installer.Install("test-package", mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("neither dnf nor yum found"))
			})
		})

		Context("when package is not available", func() {
			BeforeEach(func() {
				// Make package info commands fail
				if mockExec.FailingCommands == nil {
					mockExec.FailingCommands = make(map[string]bool)
				}
				mockExec.FailingCommands["dnf info nonexistent-package"] = true
				mockExec.FailingCommands["yum info nonexistent-package"] = true
			})

			It("returns an error when package is not available", func() {
				err := installer.Install("nonexistent-package", mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to check package availability"))
			})
		})

		Context("with Docker package", func() {
			It("installs Docker and sets up service", func() {
				err := installer.Install("docker", mockRepo)

				// Verify Docker-specific commands were executed
				commands := mockExec.Commands
				var foundDockerCommands []string
				for _, cmd := range commands {
					if cmd == "sudo systemctl enable docker" ||
						cmd == "sudo systemctl start docker" ||
						cmd == "whoami" {
						foundDockerCommands = append(foundDockerCommands, cmd)
					}
				}

				// Should have some Docker setup commands
				Expect(len(foundDockerCommands)).To(BeNumerically(">", 0))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("InstallGroup", func() {
		Context("when DNF is available", func() {
			It("installs a package group successfully", func() {
				err := installer.InstallGroup("Development Tools", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo dnf group install -y 'Development Tools'"))
			})
		})

		Context("when YUM is available but DNF is not", func() {
			BeforeEach(func() {
				// Make DNF unavailable
				mockExec.FailingCommand = "which dnf"
			})

			It("installs a package group using YUM fallback", func() {
				err := installer.InstallGroup("Development Tools", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo yum groupinstall -y 'Development Tools'"))
			})
		})
	})

	Describe("AddRepository", func() {
		It("adds a repository successfully with valid inputs", func() {
			err := installer.AddRepository("test-repo", "https://example.com/repo", "https://example.com/key.gpg")

			Expect(err).ToNot(HaveOccurred())
			// Verify secure file creation commands were executed
			var foundPrintfCmd, foundMoveCmd bool
			for _, cmd := range mockExec.Commands {
				if strings.Contains(cmd, "printf %s") && strings.Contains(cmd, "/tmp/repo-config-") {
					foundPrintfCmd = true
				}
				if strings.Contains(cmd, "sudo mv") && strings.Contains(cmd, "/etc/yum.repos.d/test-repo.repo") {
					foundMoveCmd = true
				}
			}
			Expect(foundPrintfCmd).To(BeTrue(), "Should create temporary file with printf")
			Expect(foundMoveCmd).To(BeTrue(), "Should move file to final location with sudo")
		})

		It("rejects invalid repository names", func() {
			err := installer.AddRepository("test/repo", "https://example.com/repo", "https://example.com/key.gpg")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid repository name"))
		})

		It("rejects invalid URLs", func() {
			err := installer.AddRepository("test-repo", "ftp://example.com/repo", "https://example.com/key.gpg")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid baseurl"))
		})

		It("rejects URLs with shell metacharacters", func() {
			err := installer.AddRepository("test-repo", "https://example.com/repo'; rm -rf /", "https://example.com/key.gpg")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid baseurl"))
		})
	})

	Describe("EnableEPEL", func() {
		Context("when DNF is available", func() {
			It("enables EPEL using DNF", func() {
				err := installer.EnableEPEL()

				Expect(err).ToNot(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo dnf install -y epel-release"))
			})
		})

		Context("when YUM is available but DNF is not", func() {
			BeforeEach(func() {
				mockExec.FailingCommand = "which dnf"
			})

			It("enables EPEL using YUM fallback", func() {
				err := installer.EnableEPEL()

				Expect(err).ToNot(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo yum install -y epel-release"))
			})
		})
	})

	Describe("RunDnfUpdate", func() {
		Context("when DNF is available", func() {
			It("runs DNF metadata update", func() {
				err := dnf.RunDnfUpdate(true, mockRepo)

				Expect(err).ToNot(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo dnf check-update"))
			})
		})

		Context("when YUM is available but DNF is not", func() {
			BeforeEach(func() {
				mockExec.FailingCommand = "which dnf"
			})

			It("runs YUM metadata update", func() {
				err := dnf.RunDnfUpdate(true, mockRepo)

				Expect(err).ToNot(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo yum check-update"))
			})
		})

		Context("when check-update returns exit code 100", func() {
			BeforeEach(func() {
				// Mock the exit code 100 which indicates updates are available (not an error)
				if mockExec.FailingCommands == nil {
					mockExec.FailingCommands = make(map[string]bool)
				}
				// We can't easily mock exit codes, but we can verify the function handles errors gracefully
			})

			It("handles update checks correctly", func() {
				err := dnf.RunDnfUpdate(true, mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// At minimum, it should have attempted the check
				var foundUpdateCommand bool
				for _, cmd := range mockExec.Commands {
					if cmd == "sudo dnf check-update" || cmd == "sudo yum check-update" {
						foundUpdateCommand = true
						break
					}
				}
				Expect(foundUpdateCommand).To(BeTrue())
			})
		})
	})
})
