package apt_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("APT Installer", func() {
	var (
		mockRepo     *mocks.MockRepository
		mockExec     *mocks.MockCommandExecutor
		installer    *apt.APTInstaller
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockExec = mocks.NewMockCommandExecutor()

		// Store original executor and replace with mock
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec

		installer = apt.New()
	})

	AfterEach(func() {
		// Restore original executor
		utils.CommandExec = originalExec
	})

	Describe("New", func() {
		It("creates a new APT installer", func() {
			aptInstaller := apt.New()
			Expect(aptInstaller).NotTo(BeNil())
		})
	})

	Describe("Install", func() {
		Context("with valid package", func() {
			It("installs a package successfully", func() {
				err := installer.Install("test-package", mockRepo)

				// Verify the mock captured the expected commands
				Expect(mockExec.Commands).To(ContainElement("which apt-get"))
				Expect(mockExec.Commands).To(ContainElement("which dpkg"))
				Expect(mockExec.Commands).To(ContainElement("dpkg --version"))

				// Since we're using a simple mock, the install will succeed
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when apt system validation fails", func() {
			BeforeEach(func() {
				// Set the failing command to simulate apt-get not found
				mockExec.FailingCommand = "which apt-get"
			})

			It("returns validation error", func() {
				err := installer.Install("test-package", mockRepo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("apt system validation failed"))
			})
		})

		Context("when installation command fails", func() {
			BeforeEach(func() {
				// Set the failing command to simulate installation failure
				mockExec.FailingCommand = "sudo apt-get install -y failing-package"
			})

			It("returns installation error", func() {
				err := installer.Install("failing-package", mockRepo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via apt"))
			})
		})

		Context("with Docker package", func() {
			It("installs Docker and sets up service", func() {
				err := installer.Install("docker.io", mockRepo)

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

	Describe("RunAptUpdate", func() {
		Context("with force update", func() {
			It("runs apt update successfully", func() {
				err := apt.RunAptUpdate(true, mockRepo)

				// Verify update command was called
				Expect(mockExec.Commands).To(ContainElement("sudo apt-get update"))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when update fails", func() {
			BeforeEach(func() {
				mockExec.FailingCommand = "sudo apt-get update"
			})

			It("returns update error", func() {
				err := apt.RunAptUpdate(true, mockRepo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to execute APT update"))
			})
		})
	})
})
