package mise

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

type MiseInstaller struct{}

func New() *MiseInstaller {
	return &MiseInstaller{}
}

func (m *MiseInstaller) Install(command string, repo repository.Repository) error {
	log.Info("Mise Installer: Starting installation", "language", command)

	// Wrap the command into a types.AppConfig object for the utilities function
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "mise",
		InstallCommand: command,
	}

	// Check if the language is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Mise Installer: Failed to check if language is installed", "language", command, "error", err)
		return fmt.Errorf("failed to check if language is installed: %v", err)
	}

	if isInstalled {
		log.Info("Mise Installer: Language already installed, skipping", "language", command)
		return nil
	}

	// Run mise use command
	err = utilities.RunCommand(fmt.Sprintf("mise use --global %s", command))
	if err != nil {
		log.Error("Mise Installer: Failed to install language", "language", command, "error", err)
		return fmt.Errorf("failed to install language via Mise: %v", err)
	}

	log.Info("Mise Installer: Installation successful", "language", command)

	// Add to repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Mise Installer: Failed to add language to repository", "language", command, "error", err)
		return fmt.Errorf("failed to add language to repository: %v", err)
	}

	log.Info("Mise Installer: Language added to repository", "language", command)
	return nil
}
