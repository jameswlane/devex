package apt

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
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
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Run apt install command using optimal command for the APT version
	baseCommand := getOptimalAPTCommand("install")
	installCommand := fmt.Sprintf("%s %s", baseCommand, command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package via apt", err, "command", command)
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
	if err := performPostInstallationSetup(command); err != nil {
		log.Warn("Post-installation setup failed", "package", command, "error", err)
		// Don't fail the installation, just warn
	}

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "command", command)
		return fmt.Errorf("failed to add package to repository: %w", err)
	}

	log.Debug("Package added to repository successfully", "command", command)
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
	// Use optimal command to check if package is available
	baseCommand := getOptimalAPTCommand("policy")
	command := fmt.Sprintf("%s %s", baseCommand, packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		return fmt.Errorf("failed to check package availability: %w", err)
	}

	// Check if the output indicates the package is available
	// Handle both legacy and APT 3.0 error messages
	if strings.Contains(output, "Unable to locate package") ||
		strings.Contains(output, "No packages found") ||
		strings.Contains(output, "Package not found") {
		return fmt.Errorf("package '%s' not found in any repository", packageName)
	}

	// Check if any installable version is available
	// APT 3.0 might have slightly different output format
	if !strings.Contains(output, "Candidate:") && !strings.Contains(output, "Version table:") {
		return fmt.Errorf("no installable candidate found for package '%s'", packageName)
	}

	log.Debug("Package availability validated", "package", packageName)
	return nil
}

// performPostInstallationSetup handles package-specific post-installation configuration
func performPostInstallationSetup(packageName string) error {
	switch packageName {
	case "docker.io":
		return setupDockerService()
	default:
		// No special setup required
		return nil
	}
}

// setupDockerService configures Docker service and user permissions
func setupDockerService() error {
	log.Debug("Configuring Docker service and permissions")

	// Enable Docker service to start on boot
	if _, err := utils.CommandExec.RunShellCommandWithTimeout("sudo systemctl enable docker", 30*time.Second); err != nil {
		log.Warn("Failed to enable Docker service", "error", err)
		// Continue anyway
	} else {
		log.Info("Docker service enabled for automatic startup")
	}

	// Start Docker service
	if _, err := utils.CommandExec.RunShellCommandWithTimeout("sudo systemctl start docker", 30*time.Second); err != nil {
		log.Warn("Failed to start Docker service", "error", err)
		// Continue anyway, user can start manually
	} else {
		log.Info("Docker service started successfully")
	}

	// Configure Docker daemon for log rotation
	log.Debug("Configuring Docker daemon log rotation")
	daemonConfig := `{"log-driver":"json-file","log-opts":{"max-size":"10m","max-file":"5"}}`
	daemonConfigCmd := fmt.Sprintf("echo '%s' | sudo tee /etc/docker/daemon.json", daemonConfig)
	if _, err := utils.CommandExec.RunShellCommandWithTimeout(daemonConfigCmd, 30*time.Second); err != nil {
		log.Warn("Failed to configure Docker daemon log rotation", "error", err)
		// Not critical, continue
	} else {
		log.Info("Docker daemon configured with log rotation (max 5 files of 10MB each)")
		// Restart Docker to apply daemon.json changes
		if _, err := utils.CommandExec.RunShellCommandWithTimeout("sudo systemctl restart docker", 30*time.Second); err != nil {
			log.Warn("Failed to restart Docker after daemon configuration", "error", err)
		} else {
			log.Info("Docker service restarted with new configuration")
		}
	}

	// Add current user to docker group
	currentUser := getCurrentUser()
	if currentUser == "" {
		log.Warn("Unable to determine current user, skipping docker group addition")
		return nil
	}

	addUserCmd := fmt.Sprintf("sudo usermod -aG docker %s", currentUser)
	if _, err := utils.CommandExec.RunShellCommandWithTimeout(addUserCmd, 30*time.Second); err != nil {
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
		if output, err := utils.CommandExec.RunShellCommandWithTimeout(testCmd, 30*time.Second); err == nil {
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

// Uninstall removes packages using apt
func (a *APTInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("APT Installer: Starting uninstallation", "command", command)

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

	// Run apt remove command using optimal command for the APT version
	baseCommand := getOptimalAPTCommand("remove")
	uninstallCommand := fmt.Sprintf("%s %s", baseCommand, command)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall package via apt", err, "command", command)
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
	// Use dpkg-query to check if package is installed
	checkCommand := fmt.Sprintf("dpkg-query -W -f='${Status}' %s 2>/dev/null", command)
	output, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// dpkg-query returns non-zero exit code if package is not installed
		return false, nil
	}

	// Check if the package is installed and configured properly
	return strings.Contains(output, "install ok installed"), nil
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
