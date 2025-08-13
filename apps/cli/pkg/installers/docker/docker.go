package docker

import (
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
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

	log.Debug("Attempted to start Docker daemon in container")

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
	log.Debug("Docker Installer: Starting installation", "command", command)

	// Check if Docker is available and running
	if err := validateDockerService(); err != nil {
		return fmt.Errorf("docker service validation failed: %w", err)
	}

	// Try to get app configuration to check for DockerOptions
	var finalCommand string
	var containerName string

	if appConfig, err := config.GetAppInfo(command); err == nil && appConfig.DockerOptions.ContainerName != "" {
		// Build complete docker run command from DockerOptions
		log.Debug("Building Docker command from DockerOptions", "app", appConfig.Name)

		dockerCmd, buildErr := buildDockerRunCommand(appConfig.InstallCommand, appConfig.DockerOptions)
		if buildErr != nil {
			log.Error("Failed to build Docker command from options", buildErr, "app", appConfig.Name)
			return fmt.Errorf("failed to build Docker command: %w", buildErr)
		}

		finalCommand = dockerCmd
		containerName = appConfig.DockerOptions.ContainerName
		log.Info("Built Docker command from configuration", "command", finalCommand, "container", containerName)
	} else {
		// Use command as-is (existing behavior for full docker run commands)
		finalCommand = command
		containerName = extractContainerName(command)
		if containerName == "" {
			log.Error("Failed to extract container name from command", fmt.Errorf("command: %s", command))
			return fmt.Errorf("failed to extract container name from command")
		}
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
	if err := executeDockerCommand(finalCommand); err != nil {
		log.Error("Failed to execute Docker command", err, "command", finalCommand)
		return fmt.Errorf("failed to execute Docker command: %w", err)
	}

	log.Debug("Docker command executed successfully", "command", finalCommand)

	// Add the container to the repository
	if err := repo.AddApp(containerName); err != nil {
		log.Error("Failed to add Docker container to repository", err, "containerName", containerName)
		return fmt.Errorf("failed to add Docker container to repository: %w", err)
	}

	log.Debug("Docker container added to repository successfully", "containerName", containerName)
	return nil
}

// Uninstall removes Docker containers
func (d *DockerInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Docker Installer: Starting uninstallation", "command", command)

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

	// Check if the container is running
	isInstalled, err := d.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if Docker container is running", err, "containerName", containerName)
		return fmt.Errorf("failed to check if Docker container is running: %w", err)
	}

	if !isInstalled {
		log.Info("Docker container not running, skipping uninstallation", "containerName", containerName)
		return nil
	}

	// Stop and remove the Docker container
	stopCommand := fmt.Sprintf("docker stop %s", containerName)
	if err := executeDockerCommand(stopCommand); err != nil {
		log.Warn("Failed to stop Docker container", "containerName", containerName, "error", err)
		// Continue with removal attempt even if stop failed
	}

	removeCommand := fmt.Sprintf("docker rm %s", containerName)
	if err := executeDockerCommand(removeCommand); err != nil {
		log.Error("Failed to remove Docker container", err, "containerName", containerName)
		return fmt.Errorf("failed to remove Docker container: %w", err)
	}

	log.Debug("Docker container removed successfully", "containerName", containerName)

	// Remove the container from the repository
	if err := repo.DeleteApp(containerName); err != nil {
		log.Error("Failed to remove Docker container from repository", err, "containerName", containerName)
		return fmt.Errorf("failed to remove Docker container from repository: %w", err)
	}

	log.Debug("Docker container removed from repository successfully", "containerName", containerName)
	return nil
}

// IsInstalled checks if a Docker container is running
func (d *DockerInstaller) IsInstalled(command string) (bool, error) {
	// Extract container name from the command
	containerName := extractContainerName(command)
	if containerName == "" {
		return false, fmt.Errorf("failed to extract container name from command: %s", command)
	}

	// Check if the container is running using docker ps
	checkCommand := fmt.Sprintf("docker ps --filter \"name=%s\" --filter \"status=running\" --format \"{{.Names}}\"", containerName)
	output, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// Try with sudo if regular command failed
		sudoCheckCommand := "sudo " + checkCommand
		output, err = utils.CommandExec.RunShellCommand(sudoCheckCommand)
		if err != nil {
			// If both fail, container is likely not running or Docker is not available
			return false, nil
		}
	}

	// Check if the container name appears in the output
	return strings.Contains(output, containerName), nil
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
		log.Debug("Docker daemon is accessible via user permissions")
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
		log.Debug("Docker command executed with user permissions")
		return nil
	}

	// Add user to docker group if not already a member
	currentUser, err := user.Current()
	if err == nil && currentUser.Username != "" {
		username := currentUser.Username

		// Validate username for security (prevent command injection)
		if strings.ContainsAny(username, ";&|`$()[]{}*?") {
			log.Warn("Invalid characters in username, skipping group add", "user", username)
		} else {
			log.Info("Adding user to docker group", "user", username)
			if _, groupErr := utils.CommandExec.RunShellCommand(fmt.Sprintf("sudo usermod -aG docker %s", username)); groupErr != nil {
				log.Warn("Failed to add user to docker group", "error", groupErr, "user", username)
			} else {
				log.Info("User added to docker group. Session refresh may be required for permissions to take effect.", "user", username)
			}
		}
	}

	// If that fails, try with sudo
	sudoCommand := "sudo " + command
	if _, err := utils.CommandExec.RunShellCommand(sudoCommand); err != nil {
		log.Error("Docker command failed with both user and sudo access", err, "command", command)
		return fmt.Errorf("docker command failed even with sudo - check if Docker daemon is running and accessible: %w", err)
	}

	log.Info("Docker command executed with sudo (user may need to refresh docker group membership)", "hint", "Run 'newgrp docker' or log out and back in to refresh group membership")
	return nil
}

// buildDockerRunCommand constructs a complete docker run command from DockerOptions
func buildDockerRunCommand(imageName string, options types.DockerOptions) (string, error) {
	if imageName == "" {
		return "", fmt.Errorf("image name is required")
	}

	if err := options.Validate(); err != nil {
		return "", fmt.Errorf("invalid docker options: %w", err)
	}

	// Build command parts securely
	var cmdParts []string
	cmdParts = append(cmdParts, "docker", "run", "-d")
	cmdParts = append(cmdParts, "--name", options.ContainerName)

	// Add restart policy if specified
	if options.RestartPolicy != "" {
		cmdParts = append(cmdParts, "--restart", options.RestartPolicy)
	}

	// Add port mappings
	for _, port := range options.Ports {
		if port != "" {
			cmdParts = append(cmdParts, "-p", port)
		}
	}

	// Add environment variables
	for _, env := range options.Environment {
		if env != "" {
			cmdParts = append(cmdParts, "-e", env)
		}
	}

	// Add the image name
	cmdParts = append(cmdParts, imageName)

	// Join with spaces - this is safe since all parts are validated
	return strings.Join(cmdParts, " "), nil
}
