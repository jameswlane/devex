package apt

import (
	"fmt"
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

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		Name:           command,
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

	// Run apt-get install command
	installCommand := fmt.Sprintf("sudo apt-get install -y %s", command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package via apt", err, "command", command)
		return fmt.Errorf("failed to install package via apt: %w", err)
	}

	log.Info("APT package installed successfully", "command", command)

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
