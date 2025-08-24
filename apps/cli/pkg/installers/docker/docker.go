package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/metrics"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type DockerInstaller struct {
	// ServiceTimeout is the timeout for waiting for Docker daemon to become ready
	ServiceTimeout time.Duration
}

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
	return &DockerInstaller{
		ServiceTimeout: 30 * time.Second, // Default timeout
	}
}

// NewWithTimeout creates a new DockerInstaller with a custom timeout
func NewWithTimeout(timeout time.Duration) *DockerInstaller {
	return &DockerInstaller{
		ServiceTimeout: timeout,
	}
}

// handleDockerInContainer handles Docker daemon setup in container environments
func (d *DockerInstaller) handleDockerInContainer() error {
	// Check if Docker socket is mounted
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		log.Info("Docker socket is available, but daemon access failed")
		// The socket exists but we can't access it - likely a permission issue
		return fmt.Errorf("docker socket exists but not accessible - container may need to run as root or with proper socket permissions")
	}

	log.Warn("Docker socket not found at /var/run/docker.sock")
	log.Info("Attempting to start Docker daemon in container environment")

	// Attempt to start Docker daemon - this might work in privileged containers
	return d.attemptDockerDaemonStartup()
}

// attemptDockerDaemonStartup tries to start Docker daemon in privileged containers
func (d *DockerInstaller) attemptDockerDaemonStartup() error {
	ctx, cancel := context.WithTimeout(context.Background(), d.ServiceTimeout)
	defer cancel()

	// Try different methods to start Docker daemon
	if err := d.tryStartDockerService(ctx); err != nil {
		log.Warn("Failed to start Docker daemon in container", "error", err)
		return fmt.Errorf("unable to start Docker daemon in container")
	}

	log.Debug("Attempted to start Docker daemon in container")

	// Wait for Docker daemon to become ready
	if err := utils.WaitForDockerDaemon(ctx, d.ServiceTimeout); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			metrics.RecordCount(metrics.MetricTimeoutOccurred, map[string]string{
				"installer": "docker",
				"operation": "daemon_startup",
			})
		}
		return fmt.Errorf("docker daemon startup attempt failed - daemon not responsive: %w", err)
	}

	log.Info("Docker daemon started successfully in container")
	metrics.RecordCount(metrics.MetricDockerDaemonReady, map[string]string{})
	return nil
}

// tryStartDockerService attempts to start Docker using various methods
func (d *DockerInstaller) tryStartDockerService(ctx context.Context) error {
	// Try systemctl first
	if cmd := exec.CommandContext(ctx, "sudo", "systemctl", "start", "docker"); cmd.Run() == nil {
		return nil
	}

	// Try service command
	if cmd := exec.CommandContext(ctx, "sudo", "service", "docker", "start"); cmd.Run() == nil {
		return nil
	}

	// Try starting dockerd directly in background (for containers)
	cmd := exec.CommandContext(ctx, "sudo", "dockerd",
		"--host=unix:///var/run/docker.sock",
		"--host=tcp://0.0.0.0:2375")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("all Docker startup methods failed")
	}

	// Detach from the process
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

func (d *DockerInstaller) Install(command string, repo types.Repository) error {
	log.Debug("Docker Installer: Starting installation", "command", command)

	// Start metrics tracking
	timer := metrics.StartInstallation("docker", command)
	defer func() {
		if r := recover(); r != nil {
			timer.Failure(fmt.Errorf("panic: %v", r))
			panic(r)
		}
	}()

	// Check if Docker is available and running
	metrics.RecordCount(metrics.MetricDockerSetupStarted, map[string]string{"command": command})
	if err := d.validateDockerService(); err != nil {
		// In container environments without Docker, skip Docker-based installations
		if isRunningInContainer() {
			log.Warn("Docker daemon not available in container, skipping Docker-based installation", "app", command)
			log.Info("To enable Docker-in-Docker, run container with: --privileged -v /var/run/docker.sock:/var/run/docker.sock")
			timer.Success() // Consider this a success since we're skipping intentionally
			return nil      // Don't fail, just skip
		}
		metrics.RecordCount(metrics.MetricDockerSetupFailed, map[string]string{"command": command})
		timer.Failure(err)
		return fmt.Errorf("docker service validation failed: %w", err)
	}
	metrics.RecordCount(metrics.MetricDockerSetupSucceeded, map[string]string{"command": command})

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
		timer.Success()
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
	timer.Success()
	return nil
}

// Uninstall removes Docker containers
func (d *DockerInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Docker Installer: Starting uninstallation", "command", command)

	// Check if Docker is available and running
	if err := d.validateDockerService(); err != nil {
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

	// Validate container name to prevent command injection
	if err := utils.ValidatePackageName(containerName); err != nil {
		return fmt.Errorf("invalid container name: %w", err)
	}

	// Stop and remove the Docker container using secure execution
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to stop the container
	stopCmd := exec.CommandContext(ctx, "docker", "stop", containerName)
	if err := stopCmd.Run(); err != nil {
		// Try with sudo
		sudoStopCmd := exec.CommandContext(ctx, "sudo", "docker", "stop", containerName)
		if err := sudoStopCmd.Run(); err != nil {
			log.Warn("Failed to stop Docker container", "containerName", containerName, "error", err)
			// Continue with removal attempt even if stop failed
		}
	}

	// Remove the container
	rmCmd := exec.CommandContext(ctx, "docker", "rm", containerName)
	if err := rmCmd.Run(); err != nil {
		// Try with sudo
		sudoRmCmd := exec.CommandContext(ctx, "sudo", "docker", "rm", containerName)
		if err := sudoRmCmd.Run(); err != nil {
			log.Error("Failed to remove Docker container", err, "containerName", containerName)
			return fmt.Errorf("failed to remove Docker container: %w", err)
		}
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

	// Validate container name to prevent command injection
	if err := utils.ValidatePackageName(containerName); err != nil {
		return false, fmt.Errorf("invalid container name: %w", err)
	}

	// Check if the container is running using docker ps (secure execution)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "ps",
		"--filter", fmt.Sprintf("name=%s", containerName),
		"--filter", "status=running",
		"--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try with sudo if regular command failed
		sudoCmd := exec.CommandContext(ctx, "sudo", "docker", "ps",
			"--filter", fmt.Sprintf("name=%s", containerName),
			"--filter", "status=running",
			"--format", "{{.Names}}")
		output, err = sudoCmd.CombinedOutput()
		if err != nil {
			// If both fail, container is likely not running or Docker is not available
			return false, nil
		}
	}

	// Check if the container name appears in the output
	return strings.Contains(string(output), containerName), nil
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
func (d *DockerInstaller) validateDockerService() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if docker command is available
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker command not found: %w", err)
	}

	// Check if Docker daemon is accessible
	if err := d.checkDockerAccess(ctx); err == nil {
		return nil
	}

	// Check if we're in a container environment and handle accordingly
	if isRunningInContainer() {
		log.Warn("Running in container environment - Docker-in-Docker may require special setup")
		log.Info("Docker-in-Docker setup help", "hint", "Ensure your container runs with: --privileged -v /var/run/docker.sock:/var/run/docker.sock")
		return d.handleDockerInContainer()
	}

	return fmt.Errorf("docker daemon not accessible: For Docker-in-Docker, run container with --privileged -v /var/run/docker.sock:/var/run/docker.sock")
}

// checkDockerAccess verifies Docker daemon accessibility
func (d *DockerInstaller) checkDockerAccess(ctx context.Context) error {
	// Try regular docker access first (user in docker group)
	cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err == nil {
		log.Debug("Docker daemon is accessible via user permissions")
		return nil
	}

	// Try with sudo (service running but user not in group)
	sudoCmd := exec.CommandContext(ctx, "sudo", "docker", "version", "--format", "{{.Server.Version}}")
	if err := sudoCmd.Run(); err == nil {
		log.Info("Docker daemon is running but requires sudo access")
		log.Warn("User may not be in docker group or needs to refresh session", "hint", "Try logging out and back in, or run 'newgrp docker'")
		return nil
	}

	return fmt.Errorf("docker daemon not accessible")
}

// addUserToDockerGroup adds the current user to the docker group
func (d *DockerInstaller) addUserToDockerGroup() error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	if currentUser.Username == "" || currentUser.Username == "root" {
		return nil // Skip for root or empty username
	}

	// Validate username for security
	if err := utils.ValidateUsername(currentUser.Username); err != nil {
		return fmt.Errorf("invalid username: %w", err)
	}

	log.Info("Adding user to docker group", "user", currentUser.Username)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "usermod", "-aG", "docker", currentUser.Username)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add user to docker group: %w (output: %s)", err, string(output))
	}

	log.Info("User added to docker group. Session refresh may be required for permissions to take effect.", "user", currentUser.Username)
	metrics.RecordCount(metrics.MetricDockerGroupAdded, map[string]string{"user": currentUser.Username})
	return nil
}

// executeDockerCommand runs a Docker command, using sudo if necessary
func executeDockerCommand(command string) error {
	// Create a global instance for this function
	d := &DockerInstaller{ServiceTimeout: 30 * time.Second}

	// First try without sudo
	if _, err := utils.CommandExec.RunShellCommand(command); err == nil {
		log.Debug("Docker command executed with user permissions")
		return nil
	}

	// Add user to docker group if not already a member
	if err := d.addUserToDockerGroup(); err != nil {
		log.Warn("Failed to add user to docker group", "error", err)
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
