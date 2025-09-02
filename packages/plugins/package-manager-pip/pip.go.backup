package pip

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

type PIPInstaller struct{}

func New() *PIPInstaller {
	return &PIPInstaller{}
}

func (p *PIPInstaller) Install(command string, repo types.Repository) error {
	log.Debug("PIP Installer: Starting installation", "package", command)

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
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

	log.Debug("Pip package installed successfully", "package", command)

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "package", command)
		return fmt.Errorf("failed to add pip package '%s' to repository: %w", command, err)
	}

	log.Debug("Pip package added to repository successfully", "package", command)
	return nil
}

// Uninstall removes packages using pip
func (p *PIPInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("PIP Installer: Starting uninstallation", "package", command)

	// Check if the package is installed
	isInstalled, err := p.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "package", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "package", command)
		return nil
	}

	// Run `pip uninstall` command
	uninstallCommand := fmt.Sprintf("pip uninstall -y %s", command)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall package via pip", err, "package", command, "command", uninstallCommand)
		return fmt.Errorf("failed to uninstall package via pip '%s': %w", command, err)
	}

	log.Debug("Pip package uninstalled successfully", "package", command)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "package", command)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Debug("Pip package removed from repository successfully", "package", command)
	return nil
}

// IsInstalled checks if a package is installed using pip
func (p *PIPInstaller) IsInstalled(command string) (bool, error) {
	// Use pip show command to check if package is installed
	checkCommand := fmt.Sprintf("pip show %s", command)
	_, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err != nil {
		// pip show returns non-zero exit code if package is not installed
		return false, nil
	}

	// If pip show succeeds, package is installed
	return true, nil
}
