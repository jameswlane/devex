package pip

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

type PIPInstaller struct{}

func New() *PIPInstaller {
	return &PIPInstaller{}
}

func (p *PIPInstaller) Install(command string, repo repository.Repository) error {
	log.Info("PIP Installer: Starting installation", "package", command)

	// Wrap the command into a types.AppConfig object for the utilities function
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "pip",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("PIP Installer: Failed to check if package is installed", "package", command, "error", err)
		return fmt.Errorf("failed to check if pip package is installed: %v", err)
	}

	if isInstalled {
		log.Info("PIP Installer: Package already installed, skipping", "package", command)
		return nil
	}

	// Run pip install
	err = utilities.RunCommand(fmt.Sprintf("pip install %s", command))
	if err != nil {
		log.Error("PIP Installer: Failed to install package", "package", command, "error", err)
		return fmt.Errorf("failed to install package via pip: %v", err)
	}

	log.Info("PIP Installer: Installation successful", "package", command)

	// Add to repository
	if err := repo.AddApp(command); err != nil {
		log.Error("PIP Installer: Failed to add package to repository", "package", command, "error", err)
		return fmt.Errorf("failed to add package to repository: %v", err)
	}

	log.Info("PIP Installer: Package added to repository", "package", command)
	return nil
}
