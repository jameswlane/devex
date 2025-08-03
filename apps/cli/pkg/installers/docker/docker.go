package docker

import (
	"fmt"
	"os"
	"strings"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type DockerInstaller struct{}

// isRunningInContainer detects if we're running inside a Docker container
func isRunningInContainer() bool {
	// Method 1: Check for .dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Method 2: Check cgroup info
	if output, err := utils.CommandExec.RunShellCommand("cat /proc/1/cgroup 2>/dev/null | grep -q docker || echo 'false'"); err == nil {
		if !strings.Contains(output, "false") {
			return true
		}
	}

	// Method 3: Check if container ID environment variable is set
	if os.Getenv("HOSTNAME") != "" {
		if hostname, err := utils.CommandExec.RunShellCommand("hostname"); err == nil {
			// Docker containers often have 12-character hostnames
			if len(strings.TrimSpace(hostname)) == 12 {
				return true
			}
		}
	}

	return false
}

func New() *DockerInstaller {
	return &DockerInstaller{}
}

// handleDockerInContainer handles Docker daemon setup in container environments
func handleDockerInContainer() error {
	// Check if Docker socket is mounted
	if _, err := utils.CommandExec.RunShellCommand("test -S /var/run/docker.sock"); err == nil {
		log.Info("Docker socket is available, but daemon access failed")
		// The socket exists but we can't access it - likely a permission issue
		return fmt.Errorf("docker socket exists but not accessible - container may need to run as root or with proper socket permissions")
	}

	log.Warn("Docker socket not found at /var/run/docker.sock")
	return attemptDockerDaemonStartup()
}

// attemptDockerDaemonStartup tries to start Docker daemon in privileged containers
func attemptDockerDaemonStartup() error {
	startCmd := "sudo service docker start 2>/dev/null || sudo systemctl start docker 2>/dev/null || sudo dockerd --host=unix:///var/run/docker.sock --host=tcp://0.0.0.0:2375 &"

	if _, err := utils.CommandExec.RunShellCommand(startCmd); err != nil {
		log.Warn("Failed to start Docker daemon in container", "error", err)
		return fmt.Errorf("unable to start Docker daemon in container")
	}

	log.Info("Attempted to start Docker daemon in container")

	// Give Docker time to start and verify it's accessible
	if _, err := utils.CommandExec.RunShellCommand("sleep 5"); err == nil {
		if _, err := utils.CommandExec.RunShellCommand("sudo docker version --format '{{.Server.Version}}'"); err == nil {
			log.Info("Docker daemon started successfully in container")
			return nil
		}
	}

	return fmt.Errorf("docker daemon startup attempt failed - daemon not responsive")
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
		InstallCommand: containerName, // For Docker, just use the container name for installation checking
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

	// Run the Docker command (try with and without sudo as needed)
	if err := executeDockerCommand(command); err != nil {
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

	// Try regular docker access first (user in docker group)
	if _, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}'"); err == nil {
		log.Info("Docker daemon is accessible via user permissions")
		return nil
	}

	// Try with sudo (service running but user not in group)
	if _, err := utils.CommandExec.RunShellCommand("sudo docker version --format '{{.Server.Version}}'"); err == nil {
		log.Info("Docker daemon is running but requires sudo access")
		log.Warn("User may not be in docker group or needs to refresh session", "hint", "Try logging out and back in, or run 'newgrp docker'")
		return nil
	}

	// Check if we're in a container environment and handle accordingly
	if isRunningInContainer() {
		log.Warn("Running in container environment - Docker-in-Docker may require special setup")
		log.Info("Docker-in-Docker setup help", "hint", "Ensure your container runs with: --privileged -v /var/run/docker.sock:/var/run/docker.sock")
		return handleDockerInContainer()
	}

	return fmt.Errorf("docker daemon not accessible: For Docker-in-Docker, run container with --privileged -v /var/run/docker.sock:/var/run/docker.sock")
}

// executeDockerCommand runs a Docker command, using sudo if necessary
func executeDockerCommand(command string) error {
	// First try without sudo
	if _, err := utils.CommandExec.RunShellCommand(command); err == nil {
		log.Info("Docker command executed with user permissions")
		return nil
	}

	// If that fails, try with sudo
	sudoCommand := "sudo " + command
	if _, err := utils.CommandExec.RunShellCommand(sudoCommand); err != nil {
		return fmt.Errorf("docker command failed even with sudo: %w", err)
	}

	log.Info("Docker command executed with sudo (user may need to refresh docker group membership)")
	return nil
}
