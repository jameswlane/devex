package apt

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/types"
)

type APTInstaller struct{}

var lastAptUpdateTime time.Time

func New() *APTInstaller {
	return &APTInstaller{}
}

func (a *APTInstaller) Install(command string, repo repository.Repository) error {
	log.Info("APT Installer: Starting installation", "command", command)

	// Wrap the command into a types.AppConfig object for the utilities function
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "apt",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("APT Installer: Failed to check if package is installed", "command", command, "error", err)
		return fmt.Errorf("failed to check if package is installed via apt: %v", err)
	}

	if isInstalled {
		log.Info("APT Installer: Package already installed, skipping", "command", command)
		return nil
	}

	// Run apt-get install
	err = utilities.RunCommand(fmt.Sprintf("sudo apt-get install -y %s", command))
	if err != nil {
		log.Error("APT Installer: Failed to install package", "command", command, "error", err)
		return fmt.Errorf("failed to install package via apt: %v", err)
	}

	log.Info("APT Installer: Installation successful", "command", command)

	// Add to repository
	if err := repo.AddApp(command); err != nil {
		log.Error("APT Installer: Failed to add package to repository", "command", command, "error", err)
		return fmt.Errorf("failed to add package to repository: %v", err)
	}

	log.Info("APT Installer: Package added to repository", "command", command)
	return nil
}

func RunAptUpdate(forceUpdate bool, repo repository.Repository) error {
	log.Info("Starting RunAptUpdate", "forceUpdate", forceUpdate)

	// Check if update is required
	if !forceUpdate && time.Since(lastAptUpdateTime) < 24*time.Hour {
		log.Info("Skipping 'apt update' (cached)")
		return nil
	}

	// Execute apt-get update
	if err := utilities.RunCommand("sudo apt-get update"); err != nil {
		log.Error("Failed to run 'sudo apt-get update'", "error", err)
		return err
	}

	// Update cache
	lastAptUpdateTime = time.Now()
	if err := repo.Set("last_apt_update", lastAptUpdateTime.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to store last update time", "error", err)
	}

	log.Info("APT update completed successfully")
	return nil
}
