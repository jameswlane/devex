package brew

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type BrewInstaller struct{}

func New() *BrewInstaller {
	return &BrewInstaller{}
}

func (b *BrewInstaller) Install(command string, repo types.Repository) error {
	log.Info("Brew Installer: Starting installation", "packageName", command)

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "brew",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "packageName", command)
		return fmt.Errorf("failed to check if Brew package is installed: %w", err)
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "packageName", command)
		return nil
	}

	// Run `brew install` command
	installCommand := fmt.Sprintf("brew install %s", command)
	_, err = utils.CommandExec.RunShellCommand(installCommand)
	if err != nil {
		log.Error("Failed to install package using Brew", err, "packageName", command, "command", installCommand)
		return fmt.Errorf("failed to install Brew package '%s': %w", command, err)
	}

	log.Info("Brew package installed successfully", "packageName", command)

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "packageName", command)
		return fmt.Errorf("failed to add Brew package to repository: %w", err)
	}

	log.Info("Package added to repository successfully", "packageName", command)
	return nil
}
