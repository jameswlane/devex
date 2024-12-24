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
	log.Info("Starting Install", "app", app, "dryRun", dryRun)

	// Check if the container is already running using Docker ps
	log.Info("Checking if Docker container is running", "containerName", app.DockerOptions.ContainerName)
	isInstalledOnSystem, err := check_install.IsAppInstalled(app.DockerOptions.ContainerName)
	if err != nil {
		log.Error("Failed to check if Docker container is running", "containerName", app.DockerOptions.ContainerName, "error", err)
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
	log.Info("Building Docker run command", "containerName", app.DockerOptions.ContainerName)
	cmdArgs := []string{"run", "-d"}

	// Add restart policy
	if app.DockerOptions.RestartPolicy != "" {
		log.Info("Adding restart policy", "restartPolicy", app.DockerOptions.RestartPolicy)
		cmdArgs = append(cmdArgs, "--restart", app.DockerOptions.RestartPolicy)
	}

	// Add ports
	for _, port := range app.DockerOptions.Ports {
		log.Info("Adding port", "port", port)
		cmdArgs = append(cmdArgs, "-p", port)
	}

	// Add environment variables
	for _, env := range app.DockerOptions.Environment {
		log.Info("Adding environment variable", "env", env)
		cmdArgs = append(cmdArgs, "-e", env)
	}

	// Add container name and image
	log.Info("Adding container name and image", "containerName", app.DockerOptions.ContainerName, "image", app.InstallCommand)
	cmdArgs = append(cmdArgs, "--name", app.DockerOptions.ContainerName, app.InstallCommand)

	// Execute the Docker run command
	log.Info("Executing Docker run command", "command", fmt.Sprintf("docker %v", cmdArgs))
	cmd := exec.Command("sudo", append([]string{"docker"}, cmdArgs...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to install Docker container", "containerName", app.DockerOptions.ContainerName, "error", err, "output", string(output))
		return fmt.Errorf("failed to install Docker container: %v - %s", err, string(output))
	}
	log.Info("Docker run command executed successfully", "output", string(output))

	// Add the installed container to the repository
	log.Info("Adding Docker container to repository", "containerName", app.DockerOptions.ContainerName)
	err = repo.AddApp(app.DockerOptions.ContainerName)
	if err != nil {
		log.Error("Failed to add Docker container to repository", "containerName", app.DockerOptions.ContainerName, "error", err)
		return fmt.Errorf("failed to add Docker container %s to repository: %v", app.DockerOptions.ContainerName, err)
	}

	log.Info(fmt.Sprintf("Docker container %s installed successfully", app.DockerOptions.ContainerName))
	return nil
}
