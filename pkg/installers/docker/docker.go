package docker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/types"
)

type DockerInstaller struct{}

func New() *DockerInstaller {
	return &DockerInstaller{}
}

func (d *DockerInstaller) Install(command string, repo repository.Repository) error {
	log.Info("Docker Installer: Starting installation", "command", command)

	// Parse command to extract Docker container details (e.g., image, name)
	containerName := extractContainerName(command)
	if containerName == "" {
		log.Error("Docker Installer: Failed to extract container name from command", "command", command)
		return fmt.Errorf("failed to extract container name from command")
	}

	// Wrap the command into a types.AppConfig object for the utilities function
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "docker",
		InstallCommand: command,
	}

	// Check if the container is already running
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Docker Installer: Failed to check if container is running", "containerName", containerName, "error", err)
		return fmt.Errorf("failed to check if Docker container is running: %v", err)
	}

	if isInstalled {
		log.Info("Docker Installer: Container already running, skipping installation", "containerName", containerName)
		return nil
	}

	// Run Docker command
	err = utilities.RunCommand(command)
	if err != nil {
		log.Error("Docker Installer: Failed to execute Docker command", "command", command, "error", err)
		return fmt.Errorf("failed to execute Docker command: %v", err)
	}

	log.Info("Docker Installer: Command executed successfully", "command", command)

	// Add to repository
	if err := repo.AddApp(containerName); err != nil {
		log.Error("Docker Installer: Failed to add container to repository", "containerName", containerName, "error", err)
		return fmt.Errorf("failed to add Docker container to repository: %v", err)
	}

	log.Info("Docker Installer: Container added to repository", "containerName", containerName)
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
