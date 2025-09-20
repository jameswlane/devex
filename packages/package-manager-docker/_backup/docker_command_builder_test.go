package docker

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Docker Command Building", func() {
	var (
		mockExec     *mocks.MockCommandExecutor
		mockRepo     *mocks.MockRepository
		installer    *DockerInstaller
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockExec = &mocks.MockCommandExecutor{
			FailingCommands:   make(map[string]bool),
			Commands:          []string{},
			InstallationState: make(map[string]bool),
		}
		mockRepo = mocks.NewMockRepository()
		installer = New()

		originalExec = utils.CommandExec
		utils.CommandExec = mockExec
	})

	AfterEach(func() {
		utils.CommandExec = originalExec
	})

	Context("buildDockerRunCommand", func() {
		It("should build a basic docker run command", func() {
			options := types.DockerOptions{
				ContainerName: "test-container",
			}

			result, err := buildDockerRunCommand("nginx", options)

			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal("docker run -d --name test-container nginx"))
		})

		It("should build a complete docker run command with all options", func() {
			options := types.DockerOptions{
				ContainerName: "postgres16",
				Ports:         []string{"127.0.0.1:5432:5432"},
				Environment:   []string{"POSTGRES_HOST_AUTH_METHOD=trust"},
				RestartPolicy: "unless-stopped",
			}

			result, err := buildDockerRunCommand("postgres:16", options)

			Expect(err).ToNot(HaveOccurred())
			expected := "docker run -d --name postgres16 --restart unless-stopped -p 127.0.0.1:5432:5432 -e POSTGRES_HOST_AUTH_METHOD=trust postgres:16"
			Expect(result).To(Equal(expected))
		})

		It("should handle multiple ports and environment variables", func() {
			options := types.DockerOptions{
				ContainerName: "multi-service",
				Ports:         []string{"127.0.0.1:8080:80", "127.0.0.1:8443:443"},
				Environment:   []string{"ENV_VAR1=value1", "ENV_VAR2=value2"},
				RestartPolicy: "always",
			}

			result, err := buildDockerRunCommand("multi:latest", options)

			Expect(err).ToNot(HaveOccurred())
			expected := "docker run -d --name multi-service --restart always -p 127.0.0.1:8080:80 -p 127.0.0.1:8443:443 -e ENV_VAR1=value1 -e ENV_VAR2=value2 multi:latest"
			Expect(result).To(Equal(expected))
		})

		It("should filter out empty ports and environment variables", func() {
			options := types.DockerOptions{
				ContainerName: "test-container",
				Ports:         []string{"", "127.0.0.1:3000:3000", ""},
				Environment:   []string{"VALID_ENV=value", "", "ANOTHER_ENV=test"},
				RestartPolicy: "on-failure",
			}

			result, err := buildDockerRunCommand("node:16", options)

			Expect(err).ToNot(HaveOccurred())
			expected := "docker run -d --name test-container --restart on-failure -p 127.0.0.1:3000:3000 -e VALID_ENV=value -e ANOTHER_ENV=test node:16"
			Expect(result).To(Equal(expected))
		})

		It("should return error for empty image name", func() {
			options := types.DockerOptions{
				ContainerName: "test-container",
			}

			_, err := buildDockerRunCommand("", options)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("image name is required"))
		})

		It("should return error for invalid DockerOptions", func() {
			options := types.DockerOptions{
				ContainerName: "", // Invalid: empty container name
			}

			_, err := buildDockerRunCommand("nginx", options)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid docker options"))
		})
	})

	Context("Install behavior", func() {
		It("should fall back to command as-is if no DockerOptions found", func() {
			// Test with a full docker run command instead of app name
			fullDockerCommand := "docker run --name test-container -d nginx:latest"

			err := installer.Install(fullDockerCommand, mockRepo)

			Expect(err).ToNot(HaveOccurred())

			// Should have attempted to run the command as-is
			Expect(mockExec.Commands).To(ContainElement(fullDockerCommand))

			// Should have added the container to repository with extracted name
			apps, err := mockRepo.ListApps()
			Expect(err).ToNot(HaveOccurred())
			var appNames []string
			for _, app := range apps {
				appNames = append(appNames, app.Name)
			}
			Expect(appNames).To(ContainElement("test-container"))
		})

		It("should handle missing app configuration gracefully", func() {
			err := installer.Install("nonexistent-app", mockRepo)

			// Should fail because "nonexistent-app" is not a valid Docker command
			// Security validation should reject commands that don't start with "docker"
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("command must start with 'docker'"))
		})
	})
})
