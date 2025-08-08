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

// Uninstall removes packages using brew
func (b *BrewInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Brew Installer: Starting uninstallation", "packageName", command)

	// Check if the package is installed
	isInstalled, err := b.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "packageName", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "packageName", command)
		return nil
	}

	// Run `brew uninstall` command
	uninstallCommand := fmt.Sprintf("brew uninstall %s", command)
	_, err = utils.CommandExec.RunShellCommand(uninstallCommand)
	if err != nil {
		log.Error("Failed to uninstall package using Brew", err, "packageName", command, "command", uninstallCommand)
		return fmt.Errorf("failed to uninstall Brew package '%s': %w", command, err)
	}

	log.Info("Brew package uninstalled successfully", "packageName", command)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "packageName", command)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Info("Package removed from repository successfully", "packageName", command)
	return nil
}

// IsInstalled checks if a package is installed using brew
func (b *BrewInstaller) IsInstalled(command string) (bool, error) {
	// Use brew list to check if package is installed
	checkCommand := fmt.Sprintf("brew list %s", command)
	_, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// brew list returns non-zero exit code if package is not installed
		return false, nil
	}

	// If brew list succeeds, package is installed
	return true, nil
}
