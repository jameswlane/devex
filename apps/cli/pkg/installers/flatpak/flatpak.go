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
	log.Debug("Flatpak Installer: Starting installation", "appID", command)

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
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

	log.Debug("Flatpak app installed successfully", "appID", command)

	// Add the app to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add Flatpak app to repository", err, "appID", command)
		return fmt.Errorf("failed to add Flatpak app '%s' to repository: %w", command, err)
	}

	log.Debug("Flatpak app added to repository successfully", "appID", command)
	return nil
}

// Uninstall removes apps using flatpak
func (f *FlatpakInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Flatpak Installer: Starting uninstallation", "appID", command)

	// Check if the app is installed
	isInstalled, err := f.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if app is installed", err, "appID", command)
		return fmt.Errorf("failed to check if app is installed: %w", err)
	}

	if !isInstalled {
		log.Info("App not installed, skipping uninstallation", "appID", command)
		return nil
	}

	// Run flatpak uninstall command
	uninstallCommand := fmt.Sprintf("flatpak uninstall -y %s", command)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall Flatpak app", err, "appID", command, "command", uninstallCommand)
		return fmt.Errorf("failed to uninstall Flatpak app '%s': %w", command, err)
	}

	log.Debug("Flatpak app uninstalled successfully", "appID", command)

	// Remove the app from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove Flatpak app from repository", err, "appID", command)
		return fmt.Errorf("failed to remove Flatpak app from repository: %w", err)
	}

	log.Debug("Flatpak app removed from repository successfully", "appID", command)
	return nil
}

// IsInstalled checks if an app is installed using flatpak
func (f *FlatpakInstaller) IsInstalled(command string) (bool, error) {
	// Use flatpak list command to check if app is installed
	checkCommand := fmt.Sprintf("flatpak list --columns=application | grep -q '^%s$'", command)
	_, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// grep returns non-zero exit code if pattern is not found
		return false, nil
	}

	// If grep succeeds, app is installed
	return true, nil
}
