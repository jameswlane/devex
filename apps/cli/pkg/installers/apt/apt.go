package apt

import (
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type APTInstaller struct{}

var lastAptUpdateTime time.Time

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
	} else if !isInstalled {
		return fmt.Errorf("package installation verification failed for: %s", command)
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
