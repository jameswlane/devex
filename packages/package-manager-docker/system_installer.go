package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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

const (
	dockerAptKeyPath    = "/etc/apt/keyrings/docker.asc"
	dockerAptSourcePath = "/etc/apt/sources.list.d/docker.list"
)

// installDockerEngineDebian installs Docker Engine on Debian/Ubuntu systems
func (d *DockerInstaller) installDockerEngineDebian(ctx context.Context) error {
	d.logger.Printf("Installing Docker Engine on Debian/Ubuntu...\n")

	osReleasePath := d.osReleasePath
	if osReleasePath == "" {
		osReleasePath = "/etc/os-release"
	}
	distro, codename, err := dockerAptRelease(osReleasePath)
	if err != nil {
		return err
	}

	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update apt package lists before installing Docker prerequisites: %w", err)
	}

	d.logger.Printf("Installing prerequisites...\n")
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", "install", "-y", "ca-certificates", "curl"); err != nil {
		return fmt.Errorf("failed to install Docker prerequisites ca-certificates and curl; check apt sources and network connectivity: %w", err)
	}

	d.logger.Printf("Adding Docker GPG key...\n")
	if err := sdk.ExecCommandWithContext(ctx, true, "install", "-m", "0755", "-d", "/etc/apt/keyrings"); err != nil {
		return fmt.Errorf("failed to create apt keyrings directory: %w", err)
	}

	keyURL := fmt.Sprintf("https://download.docker.com/linux/%s/gpg", distro)
	if err := sdk.ExecCommandWithContext(ctx, true, "curl", "-fsSL", keyURL, "-o", dockerAptKeyPath); err != nil {
		return fmt.Errorf("failed to download Docker apt key from %s: %w", keyURL, err)
	}
	if err := sdk.ExecCommandWithContext(ctx, true, "chmod", "a+r", dockerAptKeyPath); err != nil {
		return fmt.Errorf("failed to make Docker apt key readable: %w", err)
	}

	architecture, err := sdk.ExecCommandOutputWithContext(ctx, "dpkg", "--print-architecture")
	if err != nil {
		return fmt.Errorf("failed to determine dpkg architecture for Docker repository: %w", err)
	}
	architecture = strings.TrimSpace(architecture)
	if architecture == "" {
		return fmt.Errorf("failed to determine dpkg architecture for Docker repository: command returned no architecture")
	}

	d.logger.Printf("Adding Docker repository...\n")
	source := fmt.Sprintf("deb [arch=%s signed-by=%s] https://download.docker.com/linux/%s %s stable\n", architecture, dockerAptKeyPath, distro, codename)
	request := sdk.ExecutionRequest{
		Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem},
		Name:      "tee",
		Args:      []string{dockerAptSourcePath},
	}
	streams := sdk.OperationIO{Stdin: strings.NewReader(source), Stdout: io.Discard, Stderr: os.Stderr}
	if err := sdk.RunOperation(ctx, request, streams); err != nil {
		return fmt.Errorf("failed to write Docker apt source %s: %w", dockerAptSourcePath, err)
	}

	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update package lists after adding Docker repo: %w", err)
	}

	d.logger.Printf("Installing Docker Engine packages...\n")
	dockerPkgs := []string{"docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}
	dockerCmd := append([]string{"install", "-y"}, dockerPkgs...)
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", dockerCmd...); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	return nil
}

func dockerAptRelease(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) && path == "/etc/os-release" {
		// os-release(5): /usr/lib/os-release is the fallback when /etc has none.
		data, err = os.ReadFile("/usr/lib/os-release")
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to read %s to identify a Debian or Ubuntu system: %w", path, err)
	}
	values := parseOSRelease(data)

	candidates := append([]string{values["ID"]}, strings.Fields(values["ID_LIKE"])...)
	distro := ""
	for _, candidate := range candidates {
		if candidate == "debian" || candidate == "ubuntu" {
			distro = candidate
			break
		}
	}
	if distro == "" {
		return "", "", fmt.Errorf("docker apt installation requires Debian, Ubuntu, or a derivative with ID_LIKE in %s; select a supported package manager", path)
	}

	codename := values["UBUNTU_CODENAME"]
	if codename == "" {
		codename = values["VERSION_CODENAME"]
	}
	if codename == "" {
		return "", "", fmt.Errorf("docker apt installation requires UBUNTU_CODENAME or VERSION_CODENAME in %s", path)
	}
	return distro, codename, nil
}

func parseOSRelease(data []byte) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			// Values use shell quoting; fall back to stripping the quotes
			// when an escape such as \$ is not valid Go syntax.
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			} else {
				value = value[1 : len(value)-1]
			}
		} else if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	return values
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
