package pip

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type PIPInstaller struct{}

func New() *PIPInstaller {
	return &PIPInstaller{}
}

func (p *PIPInstaller) Install(command string, repo types.Repository) error {
	log.Info("PIP Installer: Starting installation", "package", command)

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "pip",
		InstallCommand: command,
	}

	// Check if the package is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "package", command)
		return fmt.Errorf("failed to check if pip package is installed: %w", err)
	}

	if isInstalled {
		log.Info("Package is already installed, skipping installation", "package", command)
		return nil
	}

	// Run `pip install` command
	installCommand := fmt.Sprintf("pip install %s", command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package via pip", err, "package", command, "command", installCommand)
		return fmt.Errorf("failed to install package via pip '%s': %w", command, err)
	}

	log.Info("Pip package installed successfully", "package", command)

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "package", command)
		return fmt.Errorf("failed to add pip package '%s' to repository: %w", command, err)
	}

	log.Info("Pip package added to repository successfully", "package", command)
	return nil
}
