package main

import (
	"context"
	"fmt"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleEnsureInstalled installs Docker Engine if not present
func (d *DockerInstaller) handleEnsureInstalled(ctx context.Context, args []string) error {
	d.logger.Printf("Checking if Docker Engine is installed...\n")

	// Check if docker command exists and daemon is accessible
	if d.isDockerAvailable() && d.isDockerDaemonRunning() {
		d.logger.Success("Docker Engine is already installed and running")
		return nil
	}

	if d.isDockerAvailable() && !d.isDockerDaemonRunning() {
		d.logger.Printf("Docker is installed but daemon is not running. Starting Docker service...\n")
		if err := d.startDockerService(ctx); err != nil {
			d.logger.Warning("Failed to start Docker service: %v", err)
		}

		// Wait for daemon to start with retry mechanism
		if d.waitForDockerDaemon(ctx) {
			d.logger.Success("Docker service started successfully")
			return nil
		}
	}

	d.logger.Printf("Installing Docker Engine...\n")

	// Install Docker Engine based on system
	if err := d.installDockerEngine(ctx); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	// Start Docker service
	if err := d.startDockerService(ctx); err != nil {
		d.logger.Warning("Failed to start Docker service: %v", err)
	}

	// Add user to docker group if requested
	addUser := false
	for _, arg := range args {
		if arg == "--add-user" {
			addUser = true
			break
		}
	}

	if addUser {
		if err := d.addUserToDockerGroup(ctx); err != nil {
			d.logger.Warning("Failed to add user to docker group: %v", err)
		}
	}

	// Verify installation
	if !d.isDockerAvailable() {
		return fmt.Errorf("docker engine installation verification failed")
	}

	d.logger.Success("Docker Engine installed successfully")
	if addUser {
		d.logger.Printf("Note: You may need to log out and back in for docker group changes to take effect\n")
	}
	return nil
}

// installDockerEngine installs Docker Engine based on the detected system
func (d *DockerInstaller) installDockerEngine(ctx context.Context) error {
	// Try apt-get first (Debian/Ubuntu)
	if sdk.CommandExists("apt-get") {
		return d.installDockerEngineDebian(ctx)
	}

	// Try dnf (Fedora/RHEL)
	if sdk.CommandExists("dnf") {
		return d.installDockerEngineFedora(ctx)
	}

	// Try pacman (Arch Linux)
	if sdk.CommandExists("pacman") {
		return d.installDockerEngineArch(ctx)
	}

	// Try zypper (openSUSE)
	if sdk.CommandExists("zypper") {
		return d.installDockerEngineSUSE(ctx)
	}

	return fmt.Errorf("unsupported system: no supported package manager found")
}

// installDockerEngineDebian installs Docker Engine on Debian/Ubuntu systems
func (d *DockerInstaller) installDockerEngineDebian(ctx context.Context) error {
	d.logger.Printf("Installing Docker Engine on Debian/Ubuntu...\n")

	// Update package lists
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", "update"); err != nil {
		d.logger.Warning("Failed to update package lists: %v", err)
	}

	// Install required packages
	d.logger.Printf("Installing prerequisites...\n")
	prereqs := []string{"apt-transport-https", "ca-certificates", "curl", "gnupg", "lsb-release"}
	prereqCmd := append([]string{"install", "-y"}, prereqs...)
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", prereqCmd...); err != nil {
		d.logger.Warning("Failed to install prerequisites: %v", err)
	}

	// Add Docker's official GPG key
	d.logger.Printf("Adding Docker GPG key...\n")
	if err := sdk.ExecCommandWithContext(ctx, true, "mkdir", "-p", "/etc/apt/keyrings"); err != nil {
		d.logger.Warning("Failed to create keyrings directory: %v", err)
	}

	gpgCmd := "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg"
	if err := sdk.ExecCommandWithContext(ctx, true, "bash", "-c", gpgCmd); err != nil {
		return fmt.Errorf("failed to add Docker GPG key: %w", err)
	}

	// Add Docker repository
	d.logger.Printf("Adding Docker repository...\n")
	repoCmd := `echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null`
	if err := sdk.ExecCommandWithContext(ctx, true, "bash", "-c", repoCmd); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Update package lists again
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update package lists after adding Docker repo: %w", err)
	}

	// Install Docker Engine
	d.logger.Printf("Installing Docker Engine packages...\n")
	dockerPkgs := []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}
	dockerCmd := append([]string{"install", "-y"}, dockerPkgs...)
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", dockerCmd...); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// installDockerEngineFedora installs Docker Engine on Fedora/RHEL systems
func (d *DockerInstaller) installDockerEngineFedora(ctx context.Context) error {
	d.logger.Printf("Installing Docker Engine on Fedora/RHEL...\n")

	// Add Docker repository
	repoURL := "https://download.docker.com/linux/fedora/docker-ce.repo"
	if err := sdk.ExecCommandWithContext(ctx, true, "dnf", "config-manager", "--add-repo", repoURL); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Install Docker Engine
	dockerPkgs := []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}
	installCmd := append([]string{"install", "-y"}, dockerPkgs...)
	if err := sdk.ExecCommandWithContext(ctx, true, "dnf", installCmd...); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// installDockerEngineArch installs Docker Engine on Arch Linux
func (d *DockerInstaller) installDockerEngineArch(ctx context.Context) error {
	d.logger.Printf("Installing Docker Engine on Arch Linux...\n")

	if err := sdk.ExecCommandWithContext(ctx, true, "pacman", "-S", "--noconfirm", "docker", "docker-compose"); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// installDockerEngineSUSE installs Docker Engine on openSUSE
func (d *DockerInstaller) installDockerEngineSUSE(ctx context.Context) error {
	d.logger.Printf("Installing Docker Engine on openSUSE...\n")

	if err := sdk.ExecCommandWithContext(ctx, true, "zypper", "install", "-y", "docker", "docker-compose"); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// startDockerService starts the Docker service
func (d *DockerInstaller) startDockerService(ctx context.Context) error {
	d.logger.Printf("Starting Docker service...\n")

	// Try systemctl first
	if sdk.CommandExists("systemctl") {
		if err := sdk.ExecCommandWithContext(ctx, true, "systemctl", "enable", "docker"); err != nil {
			d.logger.Warning("Failed to enable Docker service: %v", err)
		}
		return sdk.ExecCommandWithContext(ctx, true, "systemctl", "start", "docker")
	}

	// Try service command as fallback
	if sdk.CommandExists("service") {
		return sdk.ExecCommandWithContext(ctx, true, "service", "docker", "start")
	}

	return fmt.Errorf("no service management system found")
}

// waitForDockerDaemon waits for Docker daemon to become available with retry mechanism
func (d *DockerInstaller) waitForDockerDaemon(ctx context.Context) bool {
	maxRetries := 10
	retryDelay := 2 * time.Second

	d.logger.Printf("Waiting for Docker daemon to start...")

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			d.logger.Warning("Context cancelled while waiting for Docker daemon")
			return false
		case <-time.After(retryDelay):
			if d.isDockerDaemonRunning() {
				return true
			}
			d.logger.Printf("Attempt %d/%d: Docker daemon not ready yet, retrying...", i+1, maxRetries)
		}
	}

	d.logger.Warning("Docker daemon failed to start within timeout period")
	return false
}

// addUserToDockerGroup adds the current user to the docker group
func (d *DockerInstaller) addUserToDockerGroup(ctx context.Context) error {
	// Get current user with validation
	user, err := sdk.SafeGetEnv("USER")
	if err != nil {
		d.logger.Warning("USER environment variable validation failed: %v", err)
		// Try LOGNAME as fallback
		user, err = sdk.SafeGetEnv("LOGNAME")
		if err != nil {
			d.logger.Warning("LOGNAME environment variable validation failed: %v", err)
			return fmt.Errorf("unable to determine current user: both USER and LOGNAME are invalid")
		}
	}
	if user == "" {
		// Try LOGNAME as fallback
		user, err = sdk.SafeGetEnv("LOGNAME")
		if err != nil {
			d.logger.Warning("LOGNAME environment variable validation failed: %v", err)
			return fmt.Errorf("unable to determine current user: both USER and LOGNAME are empty or invalid")
		}
		if user == "" {
			return fmt.Errorf("unable to determine current user: both USER and LOGNAME are empty")
		}
	}

	d.logger.Printf("Adding user %s to docker group...\n", user)

	// Add user to docker group
	if err := sdk.ExecCommandWithContext(ctx, true, "usermod", "-aG", "docker", user); err != nil {
		return fmt.Errorf("failed to add user to docker group: %w", err)
	}

	return nil
}
