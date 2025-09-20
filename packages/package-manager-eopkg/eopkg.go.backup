package eopkg

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// EopkgInstaller implements the Installer interface for Eopkg (Solus)
type EopkgInstaller struct{}

// NewEopkgInstaller creates a new EopkgInstaller instance
func NewEopkgInstaller() *EopkgInstaller {
	return &EopkgInstaller{}
}

// Install installs packages using eopkg
func (e *EopkgInstaller) Install(command string, repo types.Repository) error {
	if err := validateEopkgSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which eopkg"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"eopkg", command,
				[]string{
					"Install Solus Linux or run on Solus-based system",
					"Manual installation: sudo eopkg install " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"eopkg", command, err)
	}

	log.Warn("Eopkg installer is not fully implemented yet")
	log.Info("To manually install this package, run: sudo eopkg install %s", command)
	log.Info("You may need to update your package database first: sudo eopkg update-repo")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"eopkg", command,
		[]string{
			"Manual installation: sudo eopkg install " + command,
			"Update repositories: sudo eopkg update-repo",
			"Search package: eopkg search " + command,
		})
}

// Uninstall removes packages using eopkg
func (e *EopkgInstaller) Uninstall(command string, repo types.Repository) error {
	if err := validateEopkgSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which eopkg"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"eopkg", command,
				[]string{
					"Install Solus Linux or run on Solus-based system",
					"Manual removal: sudo eopkg remove " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"eopkg", command, err)
	}

	log.Warn("Eopkg uninstaller is not fully implemented yet")
	log.Info("To manually remove this package, run: sudo eopkg remove %s", command)
	log.Info("Remove unused dependencies: sudo eopkg remove-orphans")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"eopkg", command,
		[]string{
			"Manual removal: sudo eopkg remove " + command,
			"Remove with dependencies: sudo eopkg autoremove " + command,
			"Clean orphaned packages: sudo eopkg remove-orphans",
		})
}

// IsInstalled checks if a package is installed using eopkg
func (e *EopkgInstaller) IsInstalled(command string) (bool, error) {
	if err := validateEopkgSystem(); err != nil {
		return false, common.NewInstallerError(
			common.ErrorTypeSystemNotFound,
			"eopkg", command, err)
	}

	log.Warn("Eopkg IsInstalled is not fully implemented yet")
	log.Info("To manually check if this package is installed, run: eopkg list-installed | grep %s", command)

	return false, common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"eopkg", command,
		[]string{
			"Manual check: eopkg list-installed | grep " + command,
			"Package info: eopkg info " + command,
			"Search packages: eopkg search " + command,
		})
}

// validateEopkgSystem validates that the eopkg system is available and functional
func validateEopkgSystem() error {
	// Check if eopkg command exists
	if _, err := utils.CommandExec.RunShellCommand("which eopkg"); err != nil {
		return fmt.Errorf("eopkg command not found")
	}

	// Check if we're on a Solus-based system
	if output, err := utils.CommandExec.RunShellCommand("cat /etc/os-release 2>/dev/null | grep -i solus"); err != nil || output == "" {
		// Not necessarily an error, but worth noting
		log.Debug("System may not be Solus-based")
	}

	// Check if eopkg database exists
	if _, err := utils.CommandExec.RunShellCommand("test -d /var/lib/eopkg"); err != nil {
		return fmt.Errorf("eopkg database not found")
	}

	return nil
}
