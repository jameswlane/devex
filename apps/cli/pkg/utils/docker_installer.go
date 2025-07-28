package utils

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/platform"
)

// InstallDocker installs Docker Engine using the official Docker installation method
func InstallDocker(ctx context.Context) error {
	log.Info("Starting Docker installation")

	// Check if Docker is already installed
	if IsDockerInstalled() {
		log.Info("Docker is already installed")
		return nil
	}

	switch runtime.GOOS {
	case "linux":
		return installDockerLinux(ctx)
	case "darwin":
		return installDockerMacOS(ctx)
	case "windows":
		return installDockerWindows(ctx)
	default:
		return fmt.Errorf("docker installation not supported on %s", runtime.GOOS)
	}
}

// IsDockerInstalled checks if Docker is already installed and working
func IsDockerInstalled() bool {
	// Check if docker command exists and is working
	output, err := CommandExec.RunShellCommand("docker --version")
	if err != nil {
		return false
	}
	return strings.Contains(output, "Docker version")
}

// ValidateDockerInstallation validates that Docker is properly installed and running
func ValidateDockerInstallation() error {
	log.Info("Validating Docker installation")

	// Check if docker command exists
	if !IsDockerInstalled() {
		return fmt.Errorf("docker command not found")
	}

	// Check if Docker daemon is running
	output, err := CommandExec.RunShellCommand("docker ps")
	if err != nil {
		// Check if it's a permission issue
		if strings.Contains(output, "permission denied") || strings.Contains(output, "docker.sock") {
			log.Warn("Docker permission issue detected - user needs to be in docker group")
			log.Info("Docker is installed but requires group membership refresh")
			log.Info("Please log out and back in, or run: newgrp docker")
			log.Info("Then retry the setup or database installation")
			// Don't fail completely for permission issues - Docker is properly installed
			return nil
		}

		// Try to start Docker if it's not running
		log.Info("Docker daemon not running, attempting to start")
		if _, startErr := CommandExec.RunShellCommand("sudo systemctl start docker"); startErr != nil {
			return fmt.Errorf("docker daemon not running and failed to start: %w (original error: %w)", startErr, err)
		}

		// Wait a moment for Docker to fully start
		log.Info("Waiting for Docker daemon to start...")
		time.Sleep(3 * time.Second)

		// Try again after starting
		retryOutput, retryErr := CommandExec.RunShellCommand("docker ps")
		if retryErr != nil {
			if strings.Contains(retryOutput, "permission denied") || strings.Contains(retryOutput, "docker.sock") {
				log.Warn("Docker permission issue detected after restart")
				log.Info("Docker daemon is running but requires group membership refresh")
				log.Info("Please log out and back in, or run: newgrp docker")
				return nil
			}
			return fmt.Errorf("docker daemon still not responding: %w", retryErr)
		}
	}

	log.Info("Docker validation successful")
	return nil
}

func installDockerLinux(ctx context.Context) error {
	log.Info("Installing Docker on Linux")

	// Detect distribution
	plat := platform.DetectPlatform()
	distro := plat.Distribution

	switch distro {
	case "ubuntu", "debian":
		return installDockerDebian(ctx)
	case "fedora", "rhel", "centos":
		return installDockerRedHat(ctx)
	case "arch", "manjaro":
		return installDockerArch(ctx)
	default:
		return fmt.Errorf("docker installation not supported on %s", distro)
	}
}

func installDockerDebian(ctx context.Context) error {
	log.Info("Installing Docker using APT (Debian/Ubuntu)")

	// Step 1: Remove conflicting packages
	conflictingPackages := []string{
		"docker.io", "docker-doc", "docker-compose",
		"podman-docker", "containerd", "runc",
	}

	for _, pkg := range conflictingPackages {
		log.Info("Removing conflicting package", "package", pkg)
		// Use apt-get remove with -y flag, ignore errors if package not installed
		_, _ = CommandExec.RunShellCommand(fmt.Sprintf("sudo apt-get remove -y %s 2>/dev/null || true", pkg))
	}

	// Step 2: Update package index and install dependencies
	log.Info("Updating package index and installing dependencies")
	if _, err := CommandExec.RunShellCommand("sudo apt-get update"); err != nil {
		return fmt.Errorf("failed to update package index: %w", err)
	}

	dependencies := []string{"ca-certificates", "curl", "gnupg", "lsb-release"}
	dependencyCmd := fmt.Sprintf("sudo apt-get install -y %s", strings.Join(dependencies, " "))
	if _, err := CommandExec.RunShellCommand(dependencyCmd); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	// Step 3: Add Docker's official GPG key
	log.Info("Adding Docker's GPG key")

	// Create keyrings directory
	if _, err := CommandExec.RunShellCommand("sudo mkdir -p /etc/apt/keyrings"); err != nil {
		return fmt.Errorf("failed to create keyrings directory: %w", err)
	}

	// Get platform information
	plat := platform.DetectPlatform()
	distro := plat.Distribution

	// Download and add GPG key
	keyURL := fmt.Sprintf("https://download.docker.com/linux/%s/gpg", distro)
	keyCmd := fmt.Sprintf("curl -fsSL %s | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg", keyURL)
	if _, err := CommandExec.RunShellCommand(keyCmd); err != nil {
		return fmt.Errorf("failed to add Docker GPG key: %w", err)
	}

	// Set proper permissions
	if _, err := CommandExec.RunShellCommand("sudo chmod a+r /etc/apt/keyrings/docker.gpg"); err != nil {
		return fmt.Errorf("failed to set GPG key permissions: %w", err)
	}

	// Step 4: Add Docker repository
	log.Info("Adding Docker repository")

	// Get architecture (convert Go arch to Debian arch if needed)
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	}

	// Get codename from lsb_release or os-release
	codename, err := getDebianCodename()
	if err != nil {
		return fmt.Errorf("failed to get codename: %w", err)
	}

	repoLine := fmt.Sprintf("deb [arch=%s signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/%s %s stable", arch, distro, codename)
	repoCmd := fmt.Sprintf("echo \"%s\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null", repoLine)
	if _, err := CommandExec.RunShellCommand(repoCmd); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Step 5: Update package index again
	log.Info("Updating package index with Docker repository")
	if _, err := CommandExec.RunShellCommand("sudo apt-get update"); err != nil {
		return fmt.Errorf("failed to update package index with Docker repo: %w", err)
	}

	// Step 6: Install Docker Engine
	log.Info("Installing Docker Engine")
	dockerPackages := []string{
		"docker-ce", "docker-ce-cli", "containerd.io",
		"docker-buildx-plugin", "docker-compose-plugin",
	}
	installCmd := fmt.Sprintf("sudo apt-get install -y %s", strings.Join(dockerPackages, " "))
	if _, err := CommandExec.RunShellCommand(installCmd); err != nil {
		return fmt.Errorf("failed to install Docker packages: %w", err)
	}

	// Step 7: Start and enable Docker service
	log.Info("Starting and enabling Docker service")
	if _, err := CommandExec.RunShellCommand("sudo systemctl start docker"); err != nil {
		return fmt.Errorf("failed to start Docker service: %w", err)
	}
	if _, err := CommandExec.RunShellCommand("sudo systemctl enable docker"); err != nil {
		return fmt.Errorf("failed to enable Docker service: %w", err)
	}

	// Step 8: Add current user to docker group
	log.Info("Adding user to docker group")
	currentUser := os.Getenv("USER")
	if currentUser != "" {
		usermodCmd := fmt.Sprintf("sudo usermod -aG docker %s", currentUser)
		if _, err := CommandExec.RunShellCommand(usermodCmd); err != nil {
			log.Warn("Failed to add user to docker group", "error", err, "user", currentUser)
			log.Info("You may need to manually add yourself to the docker group with: sudo usermod -aG docker $USER")
		} else {
			log.Info("User added to docker group", "user", currentUser)
			log.Info("You may need to log out and back in for group changes to take effect")
		}
	}

	log.Info("Docker installation completed successfully")
	return nil
}

func installDockerRedHat(ctx context.Context) error {
	log.Info("Installing Docker using DNF/YUM (Red Hat/Fedora)")

	// Remove conflicting packages
	conflictingPackages := []string{"docker", "docker-client", "docker-client-latest", "docker-common", "docker-latest", "docker-latest-logrotate", "docker-logrotate", "docker-engine", "podman", "runc"}
	for _, pkg := range conflictingPackages {
		_, _ = CommandExec.RunShellCommand(fmt.Sprintf("sudo dnf remove -y %s 2>/dev/null || sudo yum remove -y %s 2>/dev/null || true", pkg, pkg))
	}

	// Install dependencies
	if _, err := CommandExec.RunShellCommand("sudo dnf install -y dnf-plugins-core || sudo yum install -y yum-utils"); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	// Add Docker repository
	repoCmd := "sudo dnf config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo || sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo"
	if _, err := CommandExec.RunShellCommand(repoCmd); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Install Docker
	installCmd := "sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin || sudo yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin"
	if _, err := CommandExec.RunShellCommand(installCmd); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	// Start and enable Docker
	if _, err := CommandExec.RunShellCommand("sudo systemctl start docker"); err != nil {
		return fmt.Errorf("failed to start Docker: %w", err)
	}
	if _, err := CommandExec.RunShellCommand("sudo systemctl enable docker"); err != nil {
		return fmt.Errorf("failed to enable Docker: %w", err)
	}

	// Add user to docker group
	if currentUser := os.Getenv("USER"); currentUser != "" {
		if _, err := CommandExec.RunShellCommand(fmt.Sprintf("sudo usermod -aG docker %s", currentUser)); err != nil {
			log.Warn("Failed to add user to docker group", "error", err)
		}
	}

	return nil
}

func installDockerArch(ctx context.Context) error {
	log.Info("Installing Docker using pacman (Arch Linux)")

	// Install Docker
	if _, err := CommandExec.RunShellCommand("sudo pacman -S --noconfirm docker docker-compose"); err != nil {
		return fmt.Errorf("failed to install Docker: %w", err)
	}

	// Start and enable Docker
	if _, err := CommandExec.RunShellCommand("sudo systemctl start docker"); err != nil {
		return fmt.Errorf("failed to start Docker: %w", err)
	}
	if _, err := CommandExec.RunShellCommand("sudo systemctl enable docker"); err != nil {
		return fmt.Errorf("failed to enable Docker: %w", err)
	}

	// Add user to docker group
	if currentUser := os.Getenv("USER"); currentUser != "" {
		if _, err := CommandExec.RunShellCommand(fmt.Sprintf("sudo usermod -aG docker %s", currentUser)); err != nil {
			log.Warn("Failed to add user to docker group", "error", err)
		}
	}

	return nil
}

func installDockerMacOS(ctx context.Context) error {
	log.Info("Installing Docker on macOS")

	// Check if Homebrew is available
	if _, err := CommandExec.RunShellCommand("which brew"); err != nil {
		return fmt.Errorf("homebrew is required to install Docker on macOS, please install Homebrew first")
	}

	// Install Docker Desktop via Homebrew
	if _, err := CommandExec.RunShellCommand("brew install --cask docker"); err != nil {
		return fmt.Errorf("failed to install Docker Desktop: %w", err)
	}

	log.Info("Docker Desktop installed. Please start Docker Desktop from your Applications folder")
	return nil
}

func installDockerWindows(ctx context.Context) error {
	return fmt.Errorf("docker installation on Windows is not yet implemented, please install Docker Desktop manually from https://www.docker.com/products/docker-desktop")
}

// getDebianCodename gets the Debian/Ubuntu codename for repository configuration
func getDebianCodename() (string, error) {
	// Try lsb_release first
	if output, err := CommandExec.RunShellCommand("lsb_release -cs 2>/dev/null"); err == nil {
		codename := strings.TrimSpace(output)
		if codename != "" {
			return codename, nil
		}
	}

	// Fallback to reading /etc/os-release
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "VERSION_CODENAME=") {
				codename := strings.Trim(strings.TrimPrefix(line, "VERSION_CODENAME="), "\"")
				if codename != "" {
					return codename, nil
				}
			}
		}
	}

	// If all else fails, try some common defaults
	plat := platform.DetectPlatform()
	switch plat.Distribution {
	case "ubuntu":
		return "jammy", nil // Default to Ubuntu 22.04 LTS
	case "debian":
		return "bookworm", nil // Default to Debian 12
	default:
		return "", fmt.Errorf("unable to determine codename for distribution: %s", plat.Distribution)
	}
}
