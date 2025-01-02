package flatpak

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

type FlatpakInstaller struct{}

func New() *FlatpakInstaller {
	return &FlatpakInstaller{}
}

func (f *FlatpakInstaller) Install(command string, repo repository.Repository) error {
	log.Info("Flatpak Installer: Starting installation", "appID", command)

	// Wrap the command into a types.AppConfig object for the utilities function
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "flatpak",
		InstallCommand: command,
	}

	// Check if the app is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Flatpak Installer: Failed to check if app is installed", "appID", command, "error", err)
		return fmt.Errorf("failed to check if Flatpak app is installed: %v", err)
	}

	if isInstalled {
		log.Info("Flatpak Installer: App already installed, skipping", "appID", command)
		return nil
	}

	// Run flatpak install
	err = utilities.RunCommand(fmt.Sprintf("flatpak install -y %s", command))
	if err != nil {
		log.Error("Flatpak Installer: Failed to install app", "appID", command, "error", err)
		return fmt.Errorf("failed to install Flatpak app: %v", err)
	}

	log.Info("Flatpak Installer: Installation successful", "appID", command)

	// Add to repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Flatpak Installer: Failed to add app to repository", "appID", command, "error", err)
		return fmt.Errorf("failed to add Flatpak app to repository: %v", err)
	}

	log.Info("Flatpak Installer: App added to repository", "appID", command)
	return nil
}
