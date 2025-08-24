package apt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/metrics"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type APTInstaller struct{}

// APT version information
type APTVersion struct {
	Major int
	Minor int
	Patch int
}

// Cached APT version to avoid repeated detection
var cachedAPTVersion *APTVersion

// ResetVersionCache resets the cached APT version (useful for testing)
func ResetVersionCache() {
	cachedAPTVersion = nil
}

func getCurrentUser() string {
	return utilities.GetCurrentUser()
}

// getAPTVersion detects the APT version
func getAPTVersion() (*APTVersion, error) {
	if cachedAPTVersion != nil {
		return cachedAPTVersion, nil
	}

	// Try apt --version first (available in APT 1.0+)
	output, err := utils.CommandExec.RunShellCommand("apt --version")
	if err != nil {
		// Fallback to apt-get --version
		output, err = utils.CommandExec.RunShellCommand("apt-get --version")
		if err != nil {
			return nil, fmt.Errorf("failed to detect APT version: %w", err)
		}
	}

	// Parse version from output like "apt 3.0.0 (amd64)" or "apt 1.6.12ubuntu0.2 (amd64)"
	versionRegex := regexp.MustCompile(`apt\s+(\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) < 4 {
		// Try alternate format
		versionRegex = regexp.MustCompile(`apt\s+(\d+)\.(\d+)`)
		matches = versionRegex.FindStringSubmatch(output)
		if len(matches) < 3 {
			return nil, fmt.Errorf("failed to parse APT version from output: %s", output)
		}
		// Default patch to 0 if not specified
		matches = append(matches, "0")
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	cachedAPTVersion = &APTVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}

	log.Debug("Detected APT version", "version", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return cachedAPTVersion, nil
}

// isAPT3OrNewer checks if the system has APT 3.0 or newer
func isAPT3OrNewer() bool {
	version, err := getAPTVersion()
	if err != nil {
		log.Warn("Failed to detect APT version, assuming legacy", "error", err)
		return false
	}
	return version.Major >= 3
}

// getOptimalAPTCommand returns the best APT command for the current version
func getOptimalAPTCommand(operation string) string {
	switch operation {
	case "update":
		// apt-get update is still preferred for scripts per documentation
		return "sudo apt-get update"
	case "install":
		// apt-get install is still preferred for scripts per documentation
		return "sudo apt-get install -y"
	case "remove":
		// apt-get remove is still preferred for scripts per documentation
		return "sudo apt-get remove -y"
	case "search":
		// Use apt for search operations (better user experience)
		if isAPT3OrNewer() {
			return "apt search"
		}
		return "apt-cache search"
	case "show":
		// Use apt for show operations (better user experience)
		if isAPT3OrNewer() {
			return "apt show"
		}
		return "apt-cache show"
	case "policy":
		// Always use apt-cache for policy (stable interface)
		return "apt-cache policy"
	default:
		return "apt-get"
	}
}

func New() *APTInstaller {
	return &APTInstaller{}
}

func (a *APTInstaller) Install(command string, repo types.Repository) error {
	log.Debug("APT Installer: Starting installation", "command", command)

	// Start metrics tracking
	timer := metrics.StartInstallation("apt", command)

	// Detect APT version early to ensure optimal commands are used
	if _, err := getAPTVersion(); err != nil {
		log.Warn("Failed to detect APT version, using defaults", "error", err)
	}

	// Run background validation for better performance
	validator := utilities.NewBackgroundValidator(30 * time.Second)
	validator.AddSuite(utilities.CreateSystemValidationSuite("apt"))
	validator.AddSuite(utilities.CreateNetworkValidationSuite())

	ctx := context.Background()
	if err := validator.RunValidations(ctx); err != nil {
		return utilities.WrapError(err, utilities.ErrorTypeSystem, "install", command, "apt")
	}

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "apt",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed via apt: %w", err)
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "command", command)
		return nil
	}

	// Ensure package lists are up to date using intelligent update system
	if err := utilities.EnsurePackageManagerUpdated(ctx, "apt", repo, 6*time.Hour); err != nil {
		log.Warn("Failed to update package lists", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Check if package is available in repositories
	if err := validatePackageAvailability(command); err != nil {
		timer.Failure(err)
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Validate package name to prevent command injection
	if err := utils.ValidatePackageName(command); err != nil {
		log.Error("Invalid package name", err, "package", command)
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"installer": "apt",
			"package":   command,
			"reason":    "invalid_package_name",
		})
		timer.Failure(err)
		return fmt.Errorf("invalid package name: %w", err)
	}

	// Run apt install command securely using exec.CommandContext
	installCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Use apt or apt-get based on version
	aptCmd := "apt"
	if version, err := getAPTVersion(); err == nil && version.Major < 1 {
		aptCmd = "apt-get"
	}

	output, err := utils.CommandExec.RunCommand(installCtx, "sudo", aptCmd, "install", "-y", command)
	if err != nil {
		log.Error("Failed to install package via apt", err, "command", command, "output", output)

		// Check if it was a timeout
		if installCtx.Err() == context.DeadlineExceeded {
			metrics.RecordCount(metrics.MetricTimeoutOccurred, map[string]string{
				"installer": "apt",
				"package":   command,
				"operation": "install",
			})
		}

		timer.Failure(err)
		return fmt.Errorf("failed to install package via apt: %w", err)
	}

	log.Debug("APT package installed successfully", "command", command)

	// Verify installation succeeded
	if isInstalled, err := utilities.IsAppInstalled(appConfig); err != nil {
		log.Warn("Failed to verify installation", "error", err, "command", command)
		// For critical packages like Docker, provide additional guidance
		if command == "docker.io" {
			log.Info("Docker installation may require additional setup", "hint", "Try running 'sudo systemctl enable docker && sudo systemctl start docker' after installation")
		}
	} else if !isInstalled {
		// Provide more helpful error message with suggestions
		if command == "docker.io" {
			return fmt.Errorf("package installation verification failed for: %s (hint: docker.io may install as 'docker' package - check with 'dpkg -l | grep docker')", command)
		}
		return fmt.Errorf("package installation verification failed for: %s", command)
	}

	// Perform post-installation setup for specific packages
	if err := performPostInstallationSetup(ctx, command); err != nil {
		log.Warn("Post-installation setup failed", "package", command, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "command", command)
		timer.Failure(err)
		return fmt.Errorf("failed to add package to repository: %w", err)
	}

	log.Debug("Package added to repository successfully", "command", command)
	timer.Success()
	return nil
}

func RunAptUpdate(forceUpdate bool, repo types.Repository) error {
	log.Debug("Starting APT update", "forceUpdate", forceUpdate)

	ctx := context.Background()

	if forceUpdate {
		// Force update by using a very short max age
		return utilities.EnsurePackageManagerUpdated(ctx, "apt", repo, 1*time.Second)
	} else {
		// Use standard 24-hour cache
		return utilities.EnsurePackageManagerUpdated(ctx, "apt", repo, 24*time.Hour)
	}
}

// validatePackageAvailability checks if a package is available in repositories
func validatePackageAvailability(packageName string) error {
	// Validate package name first
	if err := utils.ValidatePackageName(packageName); err != nil {
		return fmt.Errorf("invalid package name: %w", err)
	}

	// Use secure command execution to check package availability
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use apt-cache policy to check package availability
	output, err := utils.CommandExec.RunCommand(ctx, "apt-cache", "policy", packageName)
	if err != nil {
		return fmt.Errorf("failed to check package availability: %w", err)
	}

	// Check if the output indicates the package is available
	// Handle both legacy and APT 3.0 error messages
	outputStr := output
	if strings.Contains(outputStr, "Unable to locate package") ||
		strings.Contains(outputStr, "No packages found") ||
		strings.Contains(outputStr, "Package not found") {
		return fmt.Errorf("package '%s' not found in any repository", packageName)
	}

	// Check if any installable version is available
	// APT 3.0 might have slightly different output format
	if !strings.Contains(outputStr, "Candidate:") && !strings.Contains(outputStr, "Version table:") {
		return fmt.Errorf("no installable candidate found for package '%s'", packageName)
	}

	log.Debug("Package availability validated", "package", packageName)
	return nil
}

// performPostInstallationSetup handles package-specific post-installation configuration
func performPostInstallationSetup(ctx context.Context, packageName string) error {
	switch packageName {
	case "docker.io":
		return setupDockerService(ctx)
	default:
		// No special setup required
		return nil
	}
}

// setupDockerService configures Docker service and user permissions
func setupDockerService(ctx context.Context) error {
	log.Debug("Configuring Docker service and permissions")

	// Enable Docker service to start on boot
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl enable docker"); err != nil {
		log.Warn("Failed to enable Docker service", "error", err)
		// Continue anyway
	} else {
		log.Info("Docker service enabled for automatic startup")
	}

	// Start Docker service
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl start docker"); err != nil {
		log.Warn("Failed to start Docker service", "error", err)
		// Continue anyway, user can start manually
	} else {
		log.Info("Docker service started successfully")
	}

	// Configure Docker daemon for log rotation (merge with existing config)
	if err := configureDockerDaemon(ctx); err != nil {
		log.Warn("Failed to configure Docker daemon log rotation", "error", err)
		// Not critical, continue
	}

	// Add current user to docker group
	currentUser := getCurrentUser()
	if currentUser == "" {
		log.Warn("Unable to determine current user, skipping docker group addition")
		return nil
	}

	addUserCmd := fmt.Sprintf("sudo usermod -aG docker %s", currentUser)
	if _, err := utils.CommandExec.RunShellCommand(addUserCmd); err != nil {
		log.Warn("Failed to add user to docker group", "user", currentUser, "error", err)
		log.Info("You may need to manually add your user to the docker group", "command", fmt.Sprintf("sudo usermod -aG docker %s", currentUser))
	} else {
		log.Info("User added to docker group", "user", currentUser)

		// Try to make the group change effective immediately using sg (switch group)
		// This is more reliable than newgrp in non-interactive contexts
		log.Debug("Attempting to refresh group membership")

		// The sg command can run a command with the new group, but we can't change the parent shell
		// However, we can test if Docker works with the new group
		testCmd := "sg docker -c 'docker version --format \"{{.Server.Version}}\"'"
		if output, err := utils.CommandExec.RunShellCommand(testCmd); err == nil {
			log.Info("Docker group membership verified and working", "docker_version", strings.TrimSpace(output))
			log.Info("Note: Current shell session still requires 'newgrp docker' or re-login for direct docker commands")
		} else {
			log.Info("Note: Group changes require session refresh - run 'newgrp docker' or log out and back in")
			log.Info("To test Docker access immediately: 'newgrp docker' then 'docker ps'")
		}
	}

	// Wait a moment for service to fully start
	time.Sleep(2 * time.Second)

	// Verify Docker daemon is accessible with current permissions
	// First try without sudo (in case group is already effective)
	if _, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}' 2>/dev/null"); err == nil {
		log.Info("Docker daemon is running and accessible without sudo")
	} else {
		log.Warn("Docker daemon may not be fully ready yet", "hint", "Try running 'sudo systemctl status docker' to check service status")
	}

	return nil
}

// configureDockerDaemon safely merges Docker daemon configuration
func configureDockerDaemon(ctx context.Context) error {
	const daemonConfigPath = "/etc/docker/daemon.json"

	// Our desired configuration
	desiredConfig := map[string]interface{}{
		"log-driver": "json-file",
		"log-opts": map[string]string{
			"max-size": "10m",
			"max-file": "5",
		},
	}

	// Read existing configuration if it exists
	existingConfig := make(map[string]interface{})
	if data, err := os.ReadFile(daemonConfigPath); err == nil {
		if err := json.Unmarshal(data, &existingConfig); err != nil {
			log.Warn("Existing daemon.json has invalid JSON, backing up and replacing", "error", err)
			// Backup the invalid file
			backupPath := daemonConfigPath + ".backup." + fmt.Sprintf("%d", time.Now().Unix())
			if _, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("sudo cp %s %s", daemonConfigPath, backupPath)); err != nil {
				log.Warn("Failed to backup invalid daemon.json", "error", err)
			} else {
				log.Info("Backed up invalid daemon.json", "backup_path", backupPath)
			}
			// Use empty config since existing is invalid
			existingConfig = make(map[string]interface{})
		} else {
			log.Debug("Found existing Docker daemon configuration, merging")
		}
	}

	// Merge configurations - desired config takes precedence for log settings only
	mergedConfig := existingConfig
	if mergedConfig == nil {
		mergedConfig = make(map[string]interface{})
	}

	// Only override log-related settings, preserve everything else
	mergedConfig["log-driver"] = desiredConfig["log-driver"]
	mergedConfig["log-opts"] = desiredConfig["log-opts"]

	// Marshal to JSON
	configJSON, err := json.MarshalIndent(mergedConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal daemon configuration: %w", err)
	}

	// Write configuration using a temporary file for atomic operation
	tempFile := daemonConfigPath + ".tmp"
	if err := writeConfigFileSecurely(ctx, configJSON, tempFile); err != nil {
		return fmt.Errorf("failed to write temporary daemon configuration: %w", err)
	}

	// Atomically move temp file to final location
	moveCmd := fmt.Sprintf("sudo mv %s %s", tempFile, daemonConfigPath)
	if _, err := utils.CommandExec.RunShellCommand(moveCmd); err != nil {
		// Cleanup temp file on failure
		if _, cleanupErr := utils.CommandExec.RunShellCommand(fmt.Sprintf("sudo rm -f %s", tempFile)); cleanupErr != nil {
			log.Warn("Failed to cleanup temporary daemon.json file", "file", tempFile, "error", cleanupErr)
		}
		return fmt.Errorf("failed to move daemon configuration to final location: %w", err)
	}

	log.Info("Docker daemon configured with log rotation (max 5 files of 10MB each)")

	// Restart Docker to apply daemon.json changes
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl restart docker"); err != nil {
		log.Warn("Failed to restart Docker after daemon configuration", "error", err)
		log.Info("You may need to manually restart Docker: sudo systemctl restart docker")
		// Don't return error - configuration was applied successfully
	} else {
		log.Info("Docker service restarted with new configuration")
	}

	return nil
}

// writeConfigFileSecurely writes content to a file using sudo without shell injection risks
func writeConfigFileSecurely(ctx context.Context, content []byte, filePath string) error {
	// Create a temporary file to write the content
	tempContentFile := filePath + ".content"
	if err := os.WriteFile(tempContentFile, content, 0600); err != nil {
		return fmt.Errorf("failed to write temporary content file: %w", err)
	}
	defer os.Remove(tempContentFile) // Clean up temp file

	// Use utils interface to run the command (supports mocking in tests)
	copyCommand := fmt.Sprintf("sudo cp %s %s", tempContentFile, filePath)
	if _, err := utils.CommandExec.RunShellCommand(copyCommand); err != nil {
		return fmt.Errorf("failed to copy file to %s: %w", filePath, err)
	}

	return nil
}

// Uninstall removes packages using apt
func (a *APTInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("APT Installer: Starting uninstallation", "command", command)

	// Validate package name to prevent command injection
	if err := utils.ValidatePackageName(command); err != nil {
		log.Error("Invalid package name", err, "package", command)
		return fmt.Errorf("invalid package name: %w", err)
	}

	// Check if the package is installed
	isInstalled, err := a.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "command", command)
		return nil
	}

	// Run apt remove command securely using exec.CommandContext
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Use apt or apt-get based on version
	aptCmd := "apt"
	if version, err := getAPTVersion(); err == nil && version.Major < 1 {
		aptCmd = "apt-get"
	}

	output, err := utils.CommandExec.RunCommand(ctx, "sudo", aptCmd, "remove", "-y", command)
	if err != nil {
		log.Error("Failed to uninstall package via apt", err, "command", command, "output", output)
		return fmt.Errorf("failed to uninstall package via apt: %w", err)
	}

	log.Debug("APT package uninstalled successfully", "command", command)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "command", command)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Debug("Package removed from repository successfully", "command", command)
	return nil
}

// IsInstalled checks if a package is installed using dpkg-query
func (a *APTInstaller) IsInstalled(command string) (bool, error) {
	// Validate package name to prevent command injection
	if err := utils.ValidatePackageName(command); err != nil {
		log.Warn("Invalid package name provided", "package", command, "error", err)
		return false, fmt.Errorf("invalid package name: %w", err)
	}

	// Use CheckPackageInstalled for secure package checking
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return utils.CheckPackageInstalled(ctx, command)
}

// PackageManager interface implementation methods

// Install installs multiple packages via APT (implements PackageManager interface)
func (a *APTInstaller) InstallPackages(ctx context.Context, packages []string, dryRun bool) error {
	if len(packages) == 0 {
		return nil
	}

	log.Info("Installing packages via APT", "packages", packages, "dryRun", dryRun)

	if dryRun {
		log.Info("DRY RUN: Would install packages", "packages", packages)
		return nil
	}

	// Update package lists first
	updateCmd := getOptimalAPTCommand("update")
	if _, err := utils.CommandExec.RunShellCommand(updateCmd); err != nil {
		return fmt.Errorf("failed to update APT package lists: %w", err)
	}

	// Install all packages in one command for efficiency
	packagesStr := strings.Join(packages, " ")
	baseInstallCmd := getOptimalAPTCommand("install")
	installCmd := fmt.Sprintf("%s %s", baseInstallCmd, packagesStr)

	if _, err := utils.CommandExec.RunShellCommand(installCmd); err != nil {
		return fmt.Errorf("failed to install packages %v: %w", packages, err)
	}

	log.Info("Successfully installed packages", "packages", packages)
	return nil
}

// IsAvailable checks if APT package manager is available
func (a *APTInstaller) IsAvailable(ctx context.Context) bool {
	_, err := utils.CommandExec.RunShellCommand("which apt-get")
	return err == nil
}

// GetName returns the package manager name
func (a *APTInstaller) GetName() string {
	return "apt"
}
