package main

import (
	"fmt"
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleEnsureInstalled installs Docker Engine if not present
func (d *DockerInstaller) handleEnsureInstalled(args []string) error {
	d.logger.Printf("Checking if Docker Engine is installed...\n")

	// Check if docker command exists and daemon is accessible
	if d.isDockerAvailable() && d.isDockerDaemonRunning() {
		d.logger.Success("Docker Engine is already installed and running")
		return nil
	}

	if d.isDockerAvailable() && !d.isDockerDaemonRunning() {
		d.logger.Printf("Docker is installed but daemon is not running. Starting Docker service...\n")
		if err := d.startDockerService(); err != nil {
			d.logger.Warning("Failed to start Docker service: %v", err)
		}
		if d.isDockerDaemonRunning() {
			d.logger.Success("Docker service started successfully")
			return nil
		}
	}

	d.logger.Printf("Installing Docker Engine...\n")

	// Install Docker Engine based on system
	if err := d.installDockerEngine(); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	// Start Docker service
	if err := d.startDockerService(); err != nil {
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
		if err := d.addUserToDockerGroup(); err != nil {
			d.logger.Warning("Failed to add user to docker group: %v", err)
		}
	}

	// Verify installation
	if !d.isDockerAvailable() {
		return fmt.Errorf("Docker Engine installation verification failed")
	}

	d.logger.Success("Docker Engine installed successfully")
	if addUser {
		d.logger.Printf("Note: You may need to log out and back in for docker group changes to take effect\n")
	}
	return nil
}

// installDockerEngine installs Docker Engine based on the detected system
func (d *DockerInstaller) installDockerEngine() error {
	// Try apt-get first (Debian/Ubuntu)
	if sdk.CommandExists("apt-get") {
		return d.installDockerEngineDebian()
	}

	// Try dnf (Fedora/RHEL)
	if sdk.CommandExists("dnf") {
		return d.installDockerEngineFedora()
	}

	// Try pacman (Arch Linux)
	if sdk.CommandExists("pacman") {
		return d.installDockerEngineArch()
	}

	// Try zypper (openSUSE)
	if sdk.CommandExists("zypper") {
		return d.installDockerEngineSUSE()
	}

	return fmt.Errorf("unsupported system: no supported package manager found")
}

// installDockerEngineDebian installs Docker Engine on Debian/Ubuntu systems
func (d *DockerInstaller) installDockerEngineDebian() error {
	d.logger.Printf("Installing Docker Engine on Debian/Ubuntu...\n")

	// Update package lists
	if err := sdk.ExecCommand(true, "apt-get", "update"); err != nil {
		d.logger.Warning("Failed to update package lists: %v", err)
	}

	// Install required packages
	d.logger.Printf("Installing prerequisites...\n")
	prereqs := []string{"apt-transport-https", "ca-certificates", "curl", "gnupg", "lsb-release"}
	prereqCmd := append([]string{"install", "-y"}, prereqs...)
	if err := sdk.ExecCommand(true, "apt-get", prereqCmd...); err != nil {
		d.logger.Warning("Failed to install prerequisites: %v", err)
	}

	// Add Docker's official GPG key
	d.logger.Printf("Adding Docker GPG key...\n")
	if err := sdk.ExecCommand(true, "mkdir", "-p", "/etc/apt/keyrings"); err != nil {
		d.logger.Warning("Failed to create keyrings directory: %v", err)
	}
	
	gpgCmd := "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg"
	if err := sdk.ExecCommand(true, "bash", "-c", gpgCmd); err != nil {
		return fmt.Errorf("failed to add Docker GPG key: %w", err)
	}

	// Add Docker repository
	d.logger.Printf("Adding Docker repository...\n")
	repoCmd := `echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null`
	if err := sdk.ExecCommand(true, "bash", "-c", repoCmd); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Update package lists again
	if err := sdk.ExecCommand(true, "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update package lists after adding Docker repo: %w", err)
	}

	// Install Docker Engine
	d.logger.Printf("Installing Docker Engine packages...\n")
	dockerPkgs := []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}
	dockerCmd := append([]string{"install", "-y"}, dockerPkgs...)
	if err := sdk.ExecCommand(true, "apt-get", dockerCmd...); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// installDockerEngineFedora installs Docker Engine on Fedora/RHEL systems
func (d *DockerInstaller) installDockerEngineFedora() error {
	d.logger.Printf("Installing Docker Engine on Fedora/RHEL...\n")

	// Add Docker repository
	repoURL := "https://download.docker.com/linux/fedora/docker-ce.repo"
	if err := sdk.ExecCommand(true, "dnf", "config-manager", "--add-repo", repoURL); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Install Docker Engine
	dockerPkgs := []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}
	installCmd := append([]string{"install", "-y"}, dockerPkgs...)
	if err := sdk.ExecCommand(true, "dnf", installCmd...); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// installDockerEngineArch installs Docker Engine on Arch Linux
func (d *DockerInstaller) installDockerEngineArch() error {
	d.logger.Printf("Installing Docker Engine on Arch Linux...\n")

	if err := sdk.ExecCommand(true, "pacman", "-S", "--noconfirm", "docker", "docker-compose"); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// installDockerEngineSUSE installs Docker Engine on openSUSE
func (d *DockerInstaller) installDockerEngineSUSE() error {
	d.logger.Printf("Installing Docker Engine on openSUSE...\n")

	if err := sdk.ExecCommand(true, "zypper", "install", "-y", "docker", "docker-compose"); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

// startDockerService starts the Docker service
func (d *DockerInstaller) startDockerService() error {
	d.logger.Printf("Starting Docker service...\n")

	// Try systemctl first
	if sdk.CommandExists("systemctl") {
		if err := sdk.ExecCommand(true, "systemctl", "enable", "docker"); err != nil {
			d.logger.Warning("Failed to enable Docker service: %v", err)
		}
		return sdk.ExecCommand(true, "systemctl", "start", "docker")
	}

	// Try service command as fallback
	if sdk.CommandExists("service") {
		return sdk.ExecCommand(true, "service", "docker", "start")
	}

	return fmt.Errorf("no service management system found")
}

// addUserToDockerGroup adds the current user to the docker group
func (d *DockerInstaller) addUserToDockerGroup() error {
	// Get current user
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("LOGNAME")
	}
	if user == "" {
		return fmt.Errorf("unable to determine current user")
	}

	d.logger.Printf("Adding user %s to docker group...\n", user)

	// Add user to docker group
	if err := sdk.ExecCommand(true, "usermod", "-aG", "docker", user); err != nil {
		return fmt.Errorf("failed to add user to docker group: %w", err)
	}

	return nil
}