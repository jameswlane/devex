package docker

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type DockerInstaller struct{}

func New() *DockerInstaller {
	return &DockerInstaller{}
}

func (d *DockerInstaller) Install(command string, repo types.Repository) error {
	log.Info("Docker Installer: Starting installation", "command", command)

	// Check if Docker is available and running
	if err := validateDockerService(); err != nil {
		return fmt.Errorf("docker service validation failed: %w", err)
	}

	// Extract container name from the command
	containerName := extractContainerName(command)
	if containerName == "" {
		log.Error("Failed to extract container name from command", fmt.Errorf("command: %s", command))
		return fmt.Errorf("failed to extract container name from command")
	}

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: containerName,
		},
		InstallMethod:  "docker",
		InstallCommand: command,
	}

	// Check if the container is already running
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if Docker container is running", err, "containerName", containerName)
		return fmt.Errorf("failed to check if Docker container is running: %w", err)
	}

	if isInstalled {
		log.Info("Docker container is already running, skipping installation", "containerName", containerName)
		return nil
	}

	// Run the Docker command
	if _, err := utils.CommandExec.RunShellCommand(command); err != nil {
		log.Error("Failed to execute Docker command", err, "command", command)
		return fmt.Errorf("failed to execute Docker command: %w", err)
	}

	log.Info("Docker command executed successfully", "command", command)

	// Add the container to the repository
	if err := repo.AddApp(containerName); err != nil {
		log.Error("Failed to add Docker container to repository", err, "containerName", containerName)
		return fmt.Errorf("failed to add Docker container to repository: %w", err)
	}

	log.Info("Docker container added to repository successfully", "containerName", containerName)
	return nil
}

func extractContainerName(command string) string {
	parts := strings.Fields(command)
	for i, part := range parts {
		if part == "--name" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// validateDockerService checks if Docker is installed and the daemon is running
func validateDockerService() error {
	// Check if docker command is available
	if _, err := utils.CommandExec.RunShellCommand("which docker"); err != nil {
		return fmt.Errorf("docker command not found: %w", err)
	}

	// Check if Docker daemon is running by attempting to get version
	if _, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}'"); err != nil {
		return fmt.Errorf("docker daemon not running or not accessible: %w (hint: try 'sudo systemctl start docker' or add user to docker group)", err)
	}

	return nil
}
