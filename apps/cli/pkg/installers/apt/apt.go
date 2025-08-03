package apt

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type APTInstaller struct{}

var lastAptUpdateTime time.Time

// getCurrentUser attempts to determine the current user through multiple methods
func getCurrentUser() string {
	// Method 1: Try USER environment variable
	if username := os.Getenv("USER"); username != "" {
		return username
	}

	// Method 2: Try LOGNAME environment variable
	if username := os.Getenv("LOGNAME"); username != "" {
		return username
	}

	// Method 3: Use os/user package
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username
	}

	// Method 4: Try whoami command as fallback
	if output, err := utils.CommandExec.RunShellCommand("whoami"); err == nil {
		username := strings.TrimSpace(output)
		if username != "" {
			return username
		}
	}

	return ""
}

func New() *APTInstaller {
	return &APTInstaller{}
}

func (a *APTInstaller) Install(command string, repo types.Repository) error {
	log.Info("APT Installer: Starting installation", "command", command)

	// Validate apt availability
	if err := validateAptSystem(); err != nil {
		return fmt.Errorf("apt system validation failed: %w", err)
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

	// Ensure package lists are up to date if they're stale
	if err := ensurePackageListsUpdated(repo); err != nil {
		log.Warn("Failed to update package lists", "error", err)
		// Continue anyway, as the installation might still work
	}

	// Check if package is available in repositories
	if err := validatePackageAvailability(command); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Run apt-get install command
	installCommand := fmt.Sprintf("sudo apt-get install -y %s", command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package via apt", err, "command", command)
		return fmt.Errorf("failed to install package via apt: %w", err)
	}

	log.Info("APT package installed successfully", "command", command)

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

	log.Info("Package added to repository successfully", "command", command)
	return nil
}

func RunAptUpdate(forceUpdate bool, repo types.Repository) error {
	log.Info("Starting APT update", "forceUpdate", forceUpdate)

	// Check if update is required
	if !forceUpdate && time.Since(lastAptUpdateTime) < 24*time.Hour {
		log.Info("APT update skipped (cached)")
		return nil
	}

	// Execute apt-get update
	updateCommand := "sudo apt-get update"
	if _, err := utils.CommandExec.RunShellCommand(updateCommand); err != nil {
		log.Error("Failed to execute APT update", err, "command", updateCommand)
		return fmt.Errorf("failed to execute APT update: %w", err)
	}

	// Update the last update time cache
	lastAptUpdateTime = time.Now()
	if err := repo.Set("last_apt_update", lastAptUpdateTime.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to store last update time in repository", err)
	}

	log.Info("APT update completed successfully")
	return nil
}

// validateAptSystem checks if apt is available and functional
func validateAptSystem() error {
	// Check if apt-get is available
	if _, err := utils.CommandExec.RunShellCommand("which apt-get"); err != nil {
		return fmt.Errorf("apt-get not found: %w", err)
	}

	// Check if dpkg is available (needed for checking installation status)
	if _, err := utils.CommandExec.RunShellCommand("which dpkg"); err != nil {
		return fmt.Errorf("dpkg not found: %w", err)
	}

	// Check if we can access the dpkg database
	if _, err := utils.CommandExec.RunShellCommand("dpkg --version"); err != nil {
		return fmt.Errorf("dpkg not functional: %w", err)
	}

	return nil
}

// ensurePackageListsUpdated updates package lists if they're stale
func ensurePackageListsUpdated(repo types.Repository) error {
	// Check if we need to update (more than 6 hours old)
	if time.Since(lastAptUpdateTime) > 6*time.Hour {
		log.Info("Package lists are stale, updating")
		return RunAptUpdate(false, repo)
	}
	return nil
}

// validatePackageAvailability checks if a package is available in repositories
func validatePackageAvailability(packageName string) error {
	// Use apt-cache policy to check if package is available
	command := fmt.Sprintf("apt-cache policy %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		return fmt.Errorf("failed to check package availability: %w", err)
	}

	// Check if the output indicates the package is available
	if strings.Contains(output, "Unable to locate package") {
		return fmt.Errorf("package '%s' not found in any repository", packageName)
	}

	// Check if any installable version is available
	if !strings.Contains(output, "Candidate:") {
		return fmt.Errorf("no installable candidate found for package '%s'", packageName)
	}

	log.Info("Package availability validated", "package", packageName)
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
	log.Info("Configuring Docker service and permissions")

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
		log.Info("Note: You may need to log out and log back in for docker group changes to take effect")
	}

	// Wait a moment for service to fully start
	time.Sleep(2 * time.Second)

	// Verify Docker daemon is accessible
	if _, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Server.Version}}'"); err == nil {
		log.Info("Docker daemon is running and accessible")
	} else {
		log.Warn("Docker daemon may not be fully ready yet", "hint", "Try running 'sudo systemctl status docker' to check service status")
	}

	return nil
}
