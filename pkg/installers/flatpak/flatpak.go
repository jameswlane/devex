package flatpak

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type FlatpakInstaller struct{}

func New() *FlatpakInstaller {
	return &FlatpakInstaller{}
}

func (f *FlatpakInstaller) Install(command string, repo types.Repository) error {
	log.Info("Flatpak Installer: Starting installation", "appID", command)

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		Name:           command,
		InstallMethod:  "flatpak",
		InstallCommand: command,
	}

	// Check if the app is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if app is installed", err, "appID", command)
		return fmt.Errorf("failed to check if Flatpak app is installed: %w", err)
	}

	if isInstalled {
		log.Info("App is already installed, skipping installation", "appID", command)
		return nil
	}

	// Run flatpak install command
	installCommand := fmt.Sprintf("flatpak install -y %s", command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install Flatpak app", err, "appID", command, "command", installCommand)
		return fmt.Errorf("failed to install Flatpak app '%s': %w", command, err)
	}

	log.Info("Flatpak app installed successfully", "appID", command)

	// Add the app to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add Flatpak app to repository", err, "appID", command)
		return fmt.Errorf("failed to add Flatpak app '%s' to repository: %w", command, err)
	}

	log.Info("Flatpak app added to repository successfully", "appID", command)
	return nil
}
