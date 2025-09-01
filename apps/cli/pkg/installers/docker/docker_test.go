package docker

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Docker Installer", func() {
	var (
		installer *DockerInstaller
		mockExec  *mocks.MockCommandExecutor
		mockRepo  *MockRepository
	)

	BeforeEach(func() {
		installer = New()
		mockExec = mocks.NewMockCommandExecutor()
		mockRepo = &MockRepository{}

		// Store original and replace with mock
		utils.CommandExec = mockExec
	})

	AfterEach(func() {
		// Clean up environment variables
		os.Unsetenv("HOSTNAME")
		// Reset mock state
		mockExec.Commands = []string{}
		mockExec.FailingCommand = ""
		mockExec.FailingCommands = make(map[string]bool)
		mockExec.InstallationState = make(map[string]bool)
	})

	Describe("New", func() {
		It("creates a new Docker installer instance", func() {
			installer := New()
			Expect(installer).ToNot(BeNil())
			Expect(installer).To(BeAssignableToTypeOf(&DockerInstaller{}))
		})
	})

	Describe("Install", func() {
		Context("with valid Docker command", func() {
			BeforeEach(func() {
				// Mock successful docker service validation
				mockExec.Commands = []string{} // Reset commands
			})

			It("installs Docker container successfully", func() {
				command := "docker run --name test-container -d nginx:latest"

				err := installer.Install(command, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement("test-container"))

				// Verify Docker service validation was called
				Expect(mockExec.Commands).To(ContainElement("which docker"))
			})

			It("skips installation if container already running", func() {
				command := "docker run --name existing-container -d nginx:latest"

				// Mark the container as already running in the mock executor
				// This will be returned by "docker ps -a --format {{.Names}}"
				mockExec.InstallationState["existing-container"] = true

				err := installer.Install(command, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				// Repository should not be called for already installed containers
				Expect(mockRepo.AddedApps).To(BeEmpty())

				// Verify that Docker service validation was called
				Expect(mockExec.Commands).To(ContainElement("which docker"))
				// Verify that Docker version check was successful (for service availability)
				Expect(mockExec.Commands).To(ContainElement("docker version --format '{{.Server.Version}}'"))
				// Verify that container status was checked
				Expect(mockExec.Commands).To(ContainElement("docker ps -a --format {{.Names}}"))
				// The docker run command should NOT be executed since container already exists
				Expect(mockExec.Commands).ToNot(ContainElement(command))
			})
		})

		Context("when Docker service validation fails", func() {
			BeforeEach(func() {
				// Mock Docker not being available - use FailingCommands map for reliable failure
				mockExec.FailingCommands["which docker"] = true
			})

			It("returns Docker service validation error", func() {
				command := "docker run --name test-container -d nginx:latest"

				err := installer.Install(command, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("docker service validation failed"))
			})
		})

		Context("with malformed Docker command", func() {
			It("returns error for command without container name", func() {
				command := "docker run -d nginx:latest" // No --name flag

				err := installer.Install(command, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to extract container name"))
			})
		})

		Context("when Docker command execution fails", func() {
			BeforeEach(func() {
				// Docker service validation succeeds but command execution fails
				command := "docker run --name failing-container -d nginx:latest"
				sudoCommand := "sudo " + command
				// Both user and sudo versions should fail
				mockExec.FailingCommands[command] = true
				mockExec.FailingCommands[sudoCommand] = true
			})

			It("returns Docker command execution error", func() {
				command := "docker run --name failing-container -d nginx:latest"

				err := installer.Install(command, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to execute Docker command"))
			})
		})

		Context("when repository operations fail", func() {
			BeforeEach(func() {
				mockRepo.ShouldFailAddApp = true
			})

			It("returns repository error", func() {
				command := "docker run --name test-container -d nginx:latest"

				err := installer.Install(command, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to add Docker container to repository"))
			})
		})
	})

	Describe("extractContainerName", func() {
		Context("with valid commands", func() {
			It("extracts container name from --name flag", func() {
				command := "docker run --name my-container -d nginx:latest"
				name := extractContainerName(command)
				Expect(name).To(Equal("my-container"))
			})

			It("extracts container name from middle of command", func() {
				command := "docker run -d --name test-app -p 8080:80 nginx:latest"
				name := extractContainerName(command)
				Expect(name).To(Equal("test-app"))
			})

			It("handles multiple flags correctly", func() {
				command := "docker run --restart unless-stopped --name production-db -d postgres:13"
				name := extractContainerName(command)
				Expect(name).To(Equal("production-db"))
			})
		})

		Context("with invalid commands", func() {
			It("returns empty string when no --name flag", func() {
				command := "docker run -d nginx:latest"
				name := extractContainerName(command)
				Expect(name).To(Equal(""))
			})

			It("returns empty string when --name flag has no value", func() {
				command := "docker run --name"
				name := extractContainerName(command)
				Expect(name).To(Equal(""))
			})

			It("handles empty command", func() {
				command := ""
				name := extractContainerName(command)
				Expect(name).To(Equal(""))
			})
		})
	})

	Describe("Docker Service Validation", func() {
		Context("when Docker is properly configured", func() {
			It("succeeds when docker is available and daemon accessible", func() {
				// Test actual installation which includes validation - use valid docker run command
				err := installer.Install("docker run --name test-container -d nginx:latest", mockRepo)
				Expect(err).NotTo(HaveOccurred())

				// Verify the validation commands were called
				Expect(len(mockExec.Commands)).To(BeNumerically(">", 0))
				Expect(mockExec.Commands).To(ContainElement("docker version --format '{{.Server.Version}}'"))
			})

			It("succeeds when docker requires sudo access", func() {
				// Mock user docker access failing but sudo succeeding
				mockExec.FailingCommands["docker version --format '{{.Server.Version}}'"] = true
				// Note: sudo version is NOT in FailingCommands, so it will succeed

				err := installer.Install("docker run --name test-container -d nginx:latest", mockRepo)
				Expect(err).NotTo(HaveOccurred())

				// Verify both user and sudo access were attempted
				Expect(mockExec.Commands).To(ContainElement("docker version --format '{{.Server.Version}}'"))
				Expect(mockExec.Commands).To(ContainElement("sudo docker version --format '{{.Server.Version}}'"))
			})
		})

		Context("when Docker is not available", func() {
			It("fails when docker command not found", func() {
				mockExec.FailingCommands["which docker"] = true

				err := installer.Install("docker run --name test-container -d nginx:latest", mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("docker command not found"))
			})

			It("handles container environment appropriately", func() {
				// Mock running in container
				os.Setenv("HOSTNAME", "1234567890ab") // 12-char hostname typical of containers
				// Mock docker daemon not accessible - both user and sudo should fail
				dockerVersionCmd := "docker version --format '{{.Server.Version}}'"
				sudoDockerVersionCmd := "sudo docker version --format '{{.Server.Version}}'"
				mockExec.FailingCommands[dockerVersionCmd] = true
				mockExec.FailingCommands[sudoDockerVersionCmd] = true

				err := installer.Install("docker run --name test-container -d nginx:latest", mockRepo)

				// The installer should now skip installation in container environments without Docker
				// This is considered a successful no-op operation rather than an error
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("executeDockerCommand", func() {
		Context("with successful execution", func() {
			It("executes command with user permissions", func() {
				command := "docker run --name test -d nginx"

				err := executeDockerCommand(command)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement(command))
			})

			It("falls back to sudo when user permissions fail", func() {
				command := "docker run --name test -d nginx"
				mockExec.FailingCommands[command] = true // Make user command fail
				// Note: sudo version is NOT in FailingCommands, so it will succeed

				err := executeDockerCommand(command)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo " + command))
			})
		})

		Context("with failed execution", func() {
			It("returns error when both user and sudo fail", func() {
				command := "docker run --name test -d nginx"
				sudoCommand := "sudo " + command
				// Make both user and sudo commands fail
				mockExec.FailingCommands[command] = true
				mockExec.FailingCommands[sudoCommand] = true

				err := executeDockerCommand(command)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("docker command failed even with sudo"))
			})
		})
	})

	Describe("isRunningInContainer", func() {
		Context("container detection methods", func() {
			It("detects container via hostname length", func() {
				os.Setenv("HOSTNAME", "1234567890ab") // 12-character hostname

				result := isRunningInContainer()

				Expect(result).To(BeTrue())
			})

			It("returns false for non-container environment", func() {
				os.Unsetenv("HOSTNAME")

				result := isRunningInContainer()

				// This should be false since we're not mocking .dockerenv or cgroup
				Expect(result).To(BeFalse())
			})
		})
	})

	Describe("handleDockerInContainer", func() {
		Context("when Docker socket is available", func() {
			It("detects socket permission issues", func() {
				// Mock socket test succeeding but Docker version check fails
				// This simulates socket existing but not accessible
				mockExec.FailingCommands["docker version --format '{{.Server.Version}}'"] = true
				mockExec.FailingCommands["sudo docker version --format '{{.Server.Version}}'"] = true

				err := installer.handleDockerInContainer()

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("socket exists but not accessible"))
			})
		})

		Context("when Docker socket is not available", func() {
			BeforeEach(func() {
				// Mock socket not existing
				mockExec.FailingCommands["test -S /var/run/docker.sock"] = true
				// Mock individual daemon startup commands failing
				mockExec.FailingCommands["sudo service docker start"] = true
				mockExec.FailingCommands["sudo systemctl start docker"] = true
				mockExec.FailingCommands["sudo dockerd --host=unix:///var/run/docker.sock"] = true
			})

			It("attempts daemon startup", func() {
				err := installer.handleDockerInContainer()

				Expect(err).To(HaveOccurred()) // Will fail in test environment
				// Verify daemon startup was attempted - check for individual commands
				Expect(mockExec.Commands).To(ContainElement("sudo service docker start"))
			})
		})
	})

	Describe("attemptDockerDaemonStartup", func() {
		It("attempts to start Docker daemon", func() {
			// Configure the mock to fail individual daemon startup commands
			mockExec.FailingCommands["sudo service docker start"] = true
			mockExec.FailingCommands["sudo systemctl start docker"] = true
			mockExec.FailingCommands["sudo dockerd --host=unix:///var/run/docker.sock"] = true

			err := installer.attemptDockerDaemonStartup()

			Expect(err).To(HaveOccurred()) // Will fail in test environment

			// Verify the startup command was called
			Expect(mockExec.Commands).To(ContainElement("sudo service docker start"))
		})

		It("handles startup command failure", func() {
			// Configure individual commands to fail
			mockExec.FailingCommands["sudo service docker start"] = true
			mockExec.FailingCommands["sudo systemctl start docker"] = true
			mockExec.FailingCommands["sudo dockerd --host=unix:///var/run/docker.sock"] = true

			err := installer.attemptDockerDaemonStartup()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unable to start Docker daemon"))
		})
	})
})

// MockRepository for testing
type MockRepository struct {
	AddedApps        []string
	ShouldFailAddApp bool
	Apps             []types.AppConfig
}

func (m *MockRepository) AddApp(name string) error {
	if m.ShouldFailAddApp {
		return errors.New("mock repository error")
	}
	m.AddedApps = append(m.AddedApps, name)
	return nil
}

func (m *MockRepository) DeleteApp(name string) error {
	return nil
}

func (m *MockRepository) GetApp(name string) (*types.AppConfig, error) {
	for _, app := range m.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, errors.New("app not found")
}

func (m *MockRepository) ListApps() ([]types.AppConfig, error) {
	return m.Apps, nil
}

func (m *MockRepository) SaveApp(app types.AppConfig) error {
	return nil
}

func (m *MockRepository) Set(key string, value string) error {
	return nil
}

func (m *MockRepository) Get(key string) (string, error) {
	return "", nil
}
