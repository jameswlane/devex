package mise

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type MiseInstaller struct{}

func New() *MiseInstaller {
	return &MiseInstaller{}
}

func (m *MiseInstaller) Install(command string, repo types.Repository) error {
	log.Info("Mise Installer: Starting installation", "language", command)

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "mise",
		InstallCommand: command,
	}

	// Check if the language is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if language is installed", err, "language", command)
		return fmt.Errorf("failed to check if language is installed: %w", err)
	}

	if isInstalled {
		log.Info("Language is already installed, skipping installation", "language", command)
		return nil
	}

	// Run `mise use --global` command
	installCommand := fmt.Sprintf("mise use --global %s", command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install language via Mise", err, "language", command, "command", installCommand)
		return fmt.Errorf("failed to install language via Mise '%s': %w", command, err)
	}

	log.Info("Language installed successfully via Mise", "language", command)

	// Add the language to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add language to repository", err, "language", command)
		return fmt.Errorf("failed to add language '%s' to repository: %w", command, err)
	}

	log.Info("Language added to repository successfully", "language", command)
	return nil
}
