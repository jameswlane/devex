package docker

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/types"
)

// Install installs a Docker app based on the app configuration
func Install(app types.AppConfig, dryRun bool, repo repository.Repository) error {
	// Check if the container is already running using Docker ps
	isInstalledOnSystem, err := check_install.IsAppInstalled(app.DockerOptions.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to check if Docker container is running: %v", err)
	}

	if isInstalledOnSystem {
		log.Info(fmt.Sprintf("Docker container %s is already running, skipping installation", app.DockerOptions.ContainerName))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would run Docker command for container: %s", app.DockerOptions.ContainerName))
		log.Info("Dry run: Simulating installation delay (5 seconds)")
		time.Sleep(5 * time.Second)
		log.Info("Dry run: Completed simulation delay")
		return nil
	}

	// Build the Docker run command
	cmdArgs := []string{"run", "-d"}

	// Add restart policy
	if app.DockerOptions.RestartPolicy != "" {
		cmdArgs = append(cmdArgs, "--restart", app.DockerOptions.RestartPolicy)
	}

	// Add ports
	for _, port := range app.DockerOptions.Ports {
		cmdArgs = append(cmdArgs, "-p", port)
	}

	// Add environment variables
	for _, env := range app.DockerOptions.Environment {
		cmdArgs = append(cmdArgs, "-e", env)
	}

	// Add container name and image
	cmdArgs = append(cmdArgs, "--name", app.DockerOptions.ContainerName, app.InstallCommand)

	// Execute the Docker run command
	cmd := exec.Command("sudo", append([]string{"docker"}, cmdArgs...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Info(fmt.Sprintf("Failed to install Docker container: %s - %s", app.DockerOptions.ContainerName, string(output)), err)
		return err
	}

	// Add the installed container to the repository
	err = repo.AddApp(app.DockerOptions.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to add Docker container %s to repository: %v", app.DockerOptions.ContainerName, err)
	}

	log.Info(fmt.Sprintf("Docker container %s installed successfully", app.DockerOptions.ContainerName))
	return nil
}
