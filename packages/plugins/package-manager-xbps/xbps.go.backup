package xbps

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// XbpsInstaller implements the Installer interface for XBPS (Void Linux)
type XbpsInstaller struct{}

// NewXbpsInstaller creates a new XbpsInstaller instance
func NewXbpsInstaller() *XbpsInstaller {
	return &XbpsInstaller{}
}

// Install installs packages using xbps-install
func (x *XbpsInstaller) Install(command string, repo types.Repository) error {
	if err := validateXbpsSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which xbps-install"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"xbps", command,
				[]string{
					"Install Void Linux or run on Void-based system",
					"Manual installation: sudo xbps-install -S " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"xbps", command, err)
	}

	log.Warn("XBPS installer is not fully implemented yet")
	log.Info("To manually install this package, run: sudo xbps-install -S %s", command)
	log.Info("Update repos first: sudo xbps-install -S")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"xbps", command,
		[]string{
			"Manual installation: sudo xbps-install -S " + command,
			"Update repositories: sudo xbps-install -S",
			"Search packages: xbps-query -Rs " + command,
			"Force install: sudo xbps-install -Sf " + command,
		})
}

// Uninstall removes packages using xbps-remove
func (x *XbpsInstaller) Uninstall(command string, repo types.Repository) error {
	if err := validateXbpsSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which xbps-remove"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"xbps", command,
				[]string{
					"Install Void Linux or run on Void-based system",
					"Manual removal: sudo xbps-remove " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"xbps", command, err)
	}

	log.Warn("XBPS uninstaller is not fully implemented yet")
	log.Info("To manually remove this package, run: sudo xbps-remove %s", command)
	log.Info("Remove recursively: sudo xbps-remove -R %s", command)

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"xbps", command,
		[]string{
			"Manual removal: sudo xbps-remove " + command,
			"Remove recursively: sudo xbps-remove -R " + command,
			"Remove orphans: sudo xbps-remove -o",
			"Clean cache: sudo xbps-remove -O",
		})
}

// IsInstalled checks if a package is installed using xbps-query
func (x *XbpsInstaller) IsInstalled(command string) (bool, error) {
	if err := validateXbpsSystem(); err != nil {
		return false, common.NewInstallerError(
			common.ErrorTypeSystemNotFound,
			"xbps", command, err)
	}

	log.Warn("XBPS IsInstalled is not fully implemented yet")
	log.Info("To manually check if this package is installed, run: xbps-query -l | grep %s", command)

	return false, common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"xbps", command,
		[]string{
			"Manual check: xbps-query -l | grep " + command,
			"Package info: xbps-query " + command,
			"Search packages: xbps-query -Rs " + command,
		})
}

// validateXbpsSystem validates that the XBPS system is available and functional
func validateXbpsSystem() error {
	// Check if xbps-install command exists
	if _, err := utils.CommandExec.RunShellCommand("which xbps-install"); err != nil {
		return fmt.Errorf("xbps-install command not found")
	}

	// Check if we're on a Void Linux system
	if output, err := utils.CommandExec.RunShellCommand("cat /etc/os-release 2>/dev/null | grep -i void"); err != nil || output == "" {
		// Not necessarily an error, but worth noting
		log.Debug("System may not be Void Linux")
	}

	// Check if xbps database exists
	if _, err := utils.CommandExec.RunShellCommand("test -d /var/db/xbps"); err != nil {
		return fmt.Errorf("XBPS database not found")
	}

	return nil
}
