package brew

import (
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/types"
)

type BrewInstaller struct{}

func New() *BrewInstaller {
	return &BrewInstaller{}
}

func (b *BrewInstaller) Install(command string, repo repository.Repository) error {
	log.Info("Brew Installer: Starting installation", "packageName", command)

	// Wrap the command into a types.AppConfig object for the utilities function
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "brew",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Brew Installer: Failed to check if package is installed", "packageName", command, "error", err)
		return fmt.Errorf("failed to check if Brew package is installed: %v", err)
	}

	if isInstalled {
		log.Info("Brew Installer: Package already installed, skipping", "packageName", command)
		return nil
	}

	// Run brew install command
	err = utilities.RunCommand(fmt.Sprintf("brew install %s", command))
	if err != nil {
		log.Error("Brew Installer: Failed to install package", "packageName", command, "error", err)
		return fmt.Errorf("failed to install Brew package: %v", err)
	}

	log.Info("Brew Installer: Installation successful", "packageName", command)

	// Add to repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Brew Installer: Failed to add package to repository", "packageName", command, "error", err)
		return fmt.Errorf("failed to add Brew package to repository: %v", err)
	}

	log.Info("Brew Installer: Package added to repository", "packageName", command)
	return nil
}
