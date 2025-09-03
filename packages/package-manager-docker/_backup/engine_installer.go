package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/metrics"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// DockerEngineInstaller handles Docker Engine installation with OS-specific logic
type DockerEngineInstaller struct {
	gpgDownloader *SecureGPGDownloader
	osDetector    platform.OSDetector
}

// NewEngineInstaller creates a new Docker Engine installer
func NewEngineInstaller() *DockerEngineInstaller {
	return &DockerEngineInstaller{
		gpgDownloader: NewSecureGPGDownloader(),
		osDetector:    platform.NewOSDetector(),
	}
}

// validateShellInput validates input for dangerous patterns
func validateShellInput(input string) error {
	dangerousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[;&|><$` + "`" + `]`),     // Shell metacharacters
		regexp.MustCompile(`\b(rm|dd|mkfs|format)\b`), // Dangerous commands
		regexp.MustCompile(`\.\.\/`),                  // Path traversal
		regexp.MustCompile(`\$\(`),                    // Command substitution
	}

	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(input) {
			return fmt.Errorf("input contains potentially dangerous pattern: %s", pattern.String())
		}
	}
	return nil
}

// secureWriteToFile safely writes content to a file using temp file and atomic move
func (d *DockerEngineInstaller) secureWriteToFile(ctx context.Context, content, filepath string) error {
	// Validate inputs
	if err := validateShellInput(content); err != nil {
		return fmt.Errorf("content validation failed: %w", err)
	}
	if err := validateShellInput(filepath); err != nil {
		return fmt.Errorf("filepath validation failed: %w", err)
	}

	// Create temporary file
	tempFile := filepath + ".tmp"

	// Write to temporary file first
	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer f.Close()
	defer os.Remove(tempFile) // Clean up on error

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to write content: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomically move to final location using sudo
	if err := d.runCommand(ctx, "sudo", "mv", tempFile, filepath); err != nil {
		return fmt.Errorf("failed to move file to final location: %w", err)
	}

	return nil
}

// InstallDockerEngine installs Docker Engine with OS-specific configuration
func (d *DockerEngineInstaller) InstallDockerEngine(ctx context.Context, repo types.Repository) error {
	log.Info("Starting Docker Engine installation with OS-specific configuration")

	timer := metrics.StartInstallation("docker-engine", "docker-ce")
	defer func() {
		if r := recover(); r != nil {
			timer.Failure(fmt.Errorf("panic during Docker Engine installation: %v", r))
			panic(r) // Re-panic after recording metrics
		}
	}()

	// Skip installation on non-Linux platforms
	if runtime.GOOS != "linux" {
		log.Info("Docker Engine installation only supported on Linux, skipping")
		timer.Success()
		return nil
	}

	// Detect OS distribution
	osInfo, err := d.osDetector.DetectOS()
	if err != nil {
		timer.Failure(err)
		return fmt.Errorf("failed to detect OS distribution: %w", err)
	}

	log.Info("Detected OS for Docker Engine installation", "os", osInfo.Distribution, "version", osInfo.Version)

	// Check if Docker is already installed
	if installed, err := d.isDockerEngineInstalled(ctx); err != nil {
		timer.Failure(err)
		return fmt.Errorf("failed to check Docker installation status: %w", err)
	} else if installed {
		log.Info("Docker Engine is already installed, skipping installation")
		timer.Success()
		return nil
	}

	// Install based on OS family
	var installErr error
	switch {
	case d.isDebianFamily(osInfo.Distribution):
		installErr = d.installDockerEngineDebian(ctx, osInfo)
	case d.isRedHatFamily(osInfo.Distribution):
		installErr = d.installDockerEngineRedHat(ctx, osInfo)
	case d.isArchFamily(osInfo.Distribution):
		installErr = d.installDockerEngineArch(ctx, osInfo)
	case d.isSUSEFamily(osInfo.Distribution):
		installErr = d.installDockerEngineSUSE(ctx, osInfo)
	default:
		installErr = fmt.Errorf("unsupported OS distribution: %s", osInfo.Distribution)
	}

	if installErr != nil {
		timer.Failure(installErr)
		return installErr
	}

	// Post-installation setup
	if err := d.postInstallSetup(ctx); err != nil {
		timer.Failure(err)
		return fmt.Errorf("post-installation setup failed: %w", err)
	}

	timer.Success()
	log.Info("Docker Engine installation completed successfully")
	return nil
}

// isDockerEngineInstalled checks if Docker Engine is already installed
func (d *DockerEngineInstaller) isDockerEngineInstalled(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "docker", "--version")
	err := cmd.Run()
	return err == nil, nil
}

// OS family detection methods
func (d *DockerEngineInstaller) isDebianFamily(distribution string) bool {
	debianDistros := []string{"ubuntu", "debian", "linuxmint", "elementary", "zorin"}
	distLower := strings.ToLower(distribution)
	for _, distro := range debianDistros {
		if strings.Contains(distLower, distro) {
			return true
		}
	}
	return false
}

func (d *DockerEngineInstaller) isRedHatFamily(distribution string) bool {
	redhatDistros := []string{"fedora", "centos", "rhel", "rocky", "almalinux", "oracle"}
	distLower := strings.ToLower(distribution)
	for _, distro := range redhatDistros {
		if strings.Contains(distLower, distro) {
			return true
		}
	}
	return false
}

func (d *DockerEngineInstaller) isArchFamily(distribution string) bool {
	archDistros := []string{"arch", "manjaro", "endeavouros", "arcolinux"}
	distLower := strings.ToLower(distribution)
	for _, distro := range archDistros {
		if strings.Contains(distLower, distro) {
			return true
		}
	}
	return false
}

func (d *DockerEngineInstaller) isSUSEFamily(distribution string) bool {
	suseDistros := []string{"opensuse", "suse", "sles"}
	distLower := strings.ToLower(distribution)
	for _, distro := range suseDistros {
		if strings.Contains(distLower, distro) {
			return true
		}
	}
	return false
}

// getDockerGPGURL returns OS-specific GPG URL
func (d *DockerEngineInstaller) getDockerGPGURL(distribution string) string {
	distLower := strings.ToLower(distribution)
	switch {
	case strings.Contains(distLower, "ubuntu") || strings.Contains(distLower, "debian"):
		return "https://download.docker.com/linux/ubuntu/gpg"
	case d.isRedHatFamily(distribution):
		return "https://download.docker.com/linux/centos/gpg"
	default:
		// Default to Ubuntu GPG key for other distributions
		return "https://download.docker.com/linux/ubuntu/gpg"
	}
}

// installDockerEngineDebian installs Docker Engine on Debian/Ubuntu systems
func (d *DockerEngineInstaller) installDockerEngineDebian(ctx context.Context, osInfo *platform.OSInfo) error {
	log.Info("Installing Docker Engine on Debian/Ubuntu system")

	// Update package index
	if err := d.runCommand(ctx, "sudo", "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update package index: %w", err)
	}

	// Install prerequisites
	prerequisites := []string{"ca-certificates", "curl", "gnupg", "lsb-release"}
	args := append([]string{"apt-get", "install", "-y"}, prerequisites...)
	if err := d.runCommand(ctx, "sudo", args...); err != nil {
		return fmt.Errorf("failed to install prerequisites: %w", err)
	}

	// Setup GPG key with certificate pinning
	gpgURL := d.getDockerGPGURL(osInfo.Distribution)
	log.Debug("Setting up Docker GPG key", "url", gpgURL)

	if err := d.gpgDownloader.DownloadAndVerifyGPGKey(ctx, gpgURL, DockerKeyringsPath); err != nil {
		return fmt.Errorf("failed to setup Docker GPG key: %w", err)
	}

	// Add Docker repository - SECURE: Use safe file writing instead of shell echo
	repoLine := fmt.Sprintf("deb [arch=%s signed-by=%s] https://download.docker.com/linux/%s %s stable",
		d.getArchitecture(ctx), DockerKeyringsPath, d.getDockerRepoOS(osInfo.Distribution), d.getVersionCodename(ctx, osInfo))

	if err := d.secureWriteToFile(ctx, repoLine+"\n", "/etc/apt/sources.list.d/docker.list"); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Update package index with new repository
	if err := d.runCommand(ctx, "sudo", "apt-get", "update"); err != nil {
		return fmt.Errorf("failed to update package index after adding Docker repository: %w", err)
	}

	// Install Docker Engine
	dockerPackages := []string{"apt-get", "install", "-y"}
	dockerPackages = append(dockerPackages, DockerPackagesAPT...)
	if err := d.runCommand(ctx, "sudo", dockerPackages...); err != nil {
		return fmt.Errorf("failed to install Docker Engine packages: %w", err)
	}

	return nil
}

// installDockerEngineRedHat installs Docker Engine on Red Hat family systems
func (d *DockerEngineInstaller) installDockerEngineRedHat(ctx context.Context, osInfo *platform.OSInfo) error {
	log.Info("Installing Docker Engine on Red Hat family system")

	// Install prerequisites
	if err := d.runCommand(ctx, "sudo", "dnf", "install", "-y", "dnf-utils"); err != nil {
		return fmt.Errorf("failed to install prerequisites: %w", err)
	}

	// Add Docker repository
	if err := d.runCommand(ctx, "sudo", "dnf", "config-manager", "--add-repo", DockerCentOSRepoURL); err != nil {
		return fmt.Errorf("failed to add Docker repository: %w", err)
	}

	// Install Docker Engine
	dockerPackages := []string{"dnf", "install", "-y"}
	dockerPackages = append(dockerPackages, DockerPackagesDNF...)
	if err := d.runCommand(ctx, "sudo", dockerPackages...); err != nil {
		return fmt.Errorf("failed to install Docker Engine packages: %w", err)
	}

	return nil
}

// installDockerEngineArch installs Docker Engine on Arch Linux systems
func (d *DockerEngineInstaller) installDockerEngineArch(ctx context.Context, osInfo *platform.OSInfo) error {
	log.Info("Installing Docker Engine on Arch Linux system")

	// Update package database
	if err := d.runCommand(ctx, "sudo", "pacman", "-Sy"); err != nil {
		return fmt.Errorf("failed to update package database: %w", err)
	}

	// Install Docker Engine
	dockerPackages := []string{"pacman", "-S", "--noconfirm"}
	dockerPackages = append(dockerPackages, DockerPackagesPacman...)
	if err := d.runCommand(ctx, "sudo", dockerPackages...); err != nil {
		return fmt.Errorf("failed to install Docker Engine packages: %w", err)
	}

	return nil
}

// installDockerEngineSUSE installs Docker Engine on SUSE systems
func (d *DockerEngineInstaller) installDockerEngineSUSE(ctx context.Context, osInfo *platform.OSInfo) error {
	log.Info("Installing Docker Engine on SUSE system")

	// Install Docker Engine
	dockerPackages := []string{"zypper", "install", "-y"}
	dockerPackages = append(dockerPackages, DockerPackagesZypper...)
	if err := d.runCommand(ctx, "sudo", dockerPackages...); err != nil {
		return fmt.Errorf("failed to install Docker Engine packages: %w", err)
	}

	return nil
}

// postInstallSetup performs post-installation configuration
func (d *DockerEngineInstaller) postInstallSetup(ctx context.Context) error {
	log.Info("Performing Docker Engine post-installation setup")

	// Create docker group if it doesn't exist
	if err := d.runCommand(ctx, "sudo", "groupadd", "docker"); err != nil {
		// Ignore error if group already exists
		log.Debug("Docker group creation failed (likely already exists)", "error", err)
	}

	// Add current user to docker group
	if currentUser, err := utils.UserProviderInstance.Current(); err == nil && currentUser.Username != "root" {
		log.Info("Adding user to docker group", "username", currentUser.Username)
		if err := d.runCommand(ctx, "sudo", "usermod", "-aG", "docker", currentUser.Username); err != nil {
			log.Warn("Failed to add user to docker group", "error", err, "username", currentUser.Username)
		}
	}

	// Create secure Docker daemon configuration
	if err := d.createDaemonConfig(ctx); err != nil {
		return fmt.Errorf("failed to create Docker daemon configuration: %w", err)
	}

	// Enable and start Docker service
	if err := d.runCommand(ctx, "sudo", "systemctl", "enable", "docker"); err != nil {
		log.Warn("Failed to enable Docker service", "error", err)
	}

	if err := d.runCommand(ctx, "sudo", "systemctl", "start", "docker"); err != nil {
		log.Warn("Failed to start Docker service", "error", err)
	}

	// Wait for Docker daemon to become ready
	return d.waitForDockerDaemon(ctx)
}

// createDaemonConfig creates a secure Docker daemon configuration
func (d *DockerEngineInstaller) createDaemonConfig(ctx context.Context) error {
	config := `{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "5"
  },
  "storage-driver": "overlay2",
  "live-restore": true,
  "userland-proxy": false,
  "no-new-privileges": true
}`

	// Create Docker config directory
	if err := d.runCommand(ctx, "sudo", "mkdir", "-p", DockerConfigDir); err != nil {
		return fmt.Errorf("failed to create Docker config directory: %w", err)
	}

	// Write daemon configuration - SECURE: Use safe file writing instead of shell echo
	if err := d.secureWriteToFile(ctx, config, DockerDaemonConfig); err != nil {
		return fmt.Errorf("failed to write Docker daemon config: %w", err)
	}

	// Set secure permissions
	if err := d.runCommand(ctx, "sudo", "chmod", "644", DockerDaemonConfig); err != nil {
		return fmt.Errorf("failed to set daemon config permissions: %w", err)
	}

	if err := d.runCommand(ctx, "sudo", "chown", "root:root", DockerDaemonConfig); err != nil {
		return fmt.Errorf("failed to set daemon config ownership: %w", err)
	}

	return nil
}

// waitForDockerDaemon waits for Docker daemon to become ready
func (d *DockerEngineInstaller) waitForDockerDaemon(ctx context.Context) error {
	log.Info("Waiting for Docker daemon to become ready")

	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for Docker daemon to become ready")
		case <-ticker.C:
			cmd := exec.CommandContext(ctx, "sudo", "docker", "version")
			if err := cmd.Run(); err == nil {
				log.Info("Docker daemon is ready")
				return nil
			}
			log.Debug("Docker daemon not ready yet, waiting...")
		}
	}
}

// Helper methods
func (d *DockerEngineInstaller) runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	log.Debug("Executing command", "cmd", cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Command failed", err, "cmd", cmd.String(), "output", string(output))
		return fmt.Errorf("command failed: %s: %w", cmd.String(), err)
	}

	log.Debug("Command succeeded", "cmd", cmd.String())
	return nil
}

func (d *DockerEngineInstaller) getArchitecture(ctx context.Context) string {
	output, err := exec.CommandContext(ctx, "dpkg", "--print-architecture").Output()
	if err != nil {
		return "amd64" // Default fallback
	}
	return strings.TrimSpace(string(output))
}

func (d *DockerEngineInstaller) getDockerRepoOS(distribution string) string {
	distLower := strings.ToLower(distribution)
	if strings.Contains(distLower, "ubuntu") {
		return "ubuntu"
	}
	if strings.Contains(distLower, "debian") {
		return "debian"
	}
	return "ubuntu" // Default fallback
}

func (d *DockerEngineInstaller) getVersionCodename(ctx context.Context, osInfo *platform.OSInfo) string {
	if osInfo.Codename != "" {
		return osInfo.Codename
	}

	// Fallback: try to detect from lsb_release
	output, err := exec.CommandContext(ctx, "lsb_release", "-cs").Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return "focal" // Safe fallback for Ubuntu
}
