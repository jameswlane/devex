package yay

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// YayInstaller implements the Installer interface for Yay (AUR helper)
type YayInstaller struct{}

// NewYayInstaller creates a new YayInstaller instance
func NewYayInstaller() *YayInstaller {
	return &YayInstaller{}
}

// Install installs packages using yay (implements BaseInstaller interface)
func (y *YayInstaller) Install(command string, repo types.Repository) error {
	log.Debug("YAY Installer: Starting AUR installation", "command", command)

	// Validate YAY system availability
	if err := validateYaySystem(); err != nil {
		return fmt.Errorf("yay system validation failed: %w", err)
	}

	// Check if the package is already installed
	isInstalled, err := y.isPackageInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed via yay: %w", err)
	}

	if isInstalled {
		log.Info("Package already installed, skipping installation", "command", command)
		return nil
	}

	// Check if package is available in AUR
	if err := validateAURPackageAvailability(command); err != nil {
		return fmt.Errorf("package validation failed: %w", err)
	}

	// Run yay install command
	installCommand := fmt.Sprintf("yay -S --noconfirm %s", command)
	if _, err := utils.CommandExec.RunShellCommand(installCommand); err != nil {
		log.Error("Failed to install package via yay", err, "command", command)
		return fmt.Errorf("failed to install package via yay: %w", err)
	}

	log.Debug("YAY package installed successfully", "command", command)

	// Verify installation succeeded
	if isInstalled, err := y.isPackageInstalled(command); err != nil {
		log.Warn("Failed to verify installation", "error", err, "command", command)
	} else if !isInstalled {
		return fmt.Errorf("package installation verification failed for: %s", command)
	}

	// Add the package to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add package to repository", err, "command", command)
		return fmt.Errorf("failed to add package to repository: %w", err)
	}

	log.Debug("Package added to repository successfully", "command", command)
	return nil
}

// Uninstall removes packages using yay
func (y *YayInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("YAY Installer: Starting uninstallation", "command", command)

	// Validate YAY system availability
	if err := validateYaySystem(); err != nil {
		return fmt.Errorf("yay system validation failed: %w", err)
	}

	// Check if the package is installed
	isInstalled, err := y.isPackageInstalled(command)
	if err != nil {
		log.Error("Failed to check if package is installed", err, "command", command)
		return fmt.Errorf("failed to check if package is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Package not installed, skipping uninstallation", "command", command)
		return nil
	}

	// Run yay remove command
	uninstallCommand := fmt.Sprintf("yay -Rs --noconfirm %s", command)
	if _, err := utils.CommandExec.RunShellCommand(uninstallCommand); err != nil {
		log.Error("Failed to uninstall package via yay", err, "command", command)
		return fmt.Errorf("failed to uninstall package via yay: %w", err)
	}

	log.Debug("YAY package uninstalled successfully", "command", command)

	// Remove the package from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove package from repository", err, "command", command)
		return fmt.Errorf("failed to remove package from repository: %w", err)
	}

	log.Debug("Package removed from repository successfully", "command", command)
	return nil
}

// IsInstalled checks if a package is installed using pacman
func (y *YayInstaller) IsInstalled(command string) (bool, error) {
	return y.isPackageInstalled(command)
}

// isPackageInstalled checks if a package is installed using pacman query
func (y *YayInstaller) isPackageInstalled(packageName string) (bool, error) {
	// Use pacman -Q to check if package is installed (yay uses pacman database)
	command := fmt.Sprintf("pacman -Q %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		// pacman -Q returns non-zero exit code if package is not installed
		if strings.Contains(output, "was not found") || strings.Contains(output, "not found") {
			return false, nil
		}
		// For other errors, return the error
		return false, fmt.Errorf("failed to check package installation status: %w", err)
	}

	// If pacman -Q succeeds, package is installed
	return true, nil
}

// validateYaySystem checks if YAY is available and functional
func validateYaySystem() error {
	// Check if yay is available
	if _, err := utils.CommandExec.RunShellCommand("which yay"); err != nil {
		return fmt.Errorf("yay not found: %w", err)
	}

	// Check if we can access yay
	if _, err := utils.CommandExec.RunShellCommand("yay --version"); err != nil {
		return fmt.Errorf("yay not functional: %w", err)
	}

	return nil
}

// validateAURPackageAvailability checks if a package is available in AUR
func validateAURPackageAvailability(packageName string) error {
	// Use yay -Si to check if package is available in AUR
	command := fmt.Sprintf("yay -Si %s", packageName)
	output, err := utils.CommandExec.RunShellCommand(command)
	if err != nil {
		return fmt.Errorf("failed to check AUR package availability: %w", err)
	}

	// Check if the output indicates the package is available
	if strings.Contains(output, "was not found") || strings.Contains(output, "not found") {
		return fmt.Errorf("package '%s' not found in AUR", packageName)
	}

	log.Debug("Package availability validated in AUR", "package", packageName)
	return nil
}
