package snap

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// SnapInstaller implements the Installer interface for Snap packages
type SnapInstaller struct{}

// NewSnapInstaller creates a new SnapInstaller instance
func NewSnapInstaller() *SnapInstaller {
	return &SnapInstaller{}
}

// Install installs packages using snap
func (s *SnapInstaller) Install(command string, repo types.Repository) error {
	if err := validateSnapSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which snap"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"snap", command,
				[]string{
					"Install snapd: sudo apt install snapd (Ubuntu/Debian)",
					"Install snapd: sudo dnf install snapd (Fedora)",
					"Manual installation: sudo snap install " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"snap", command, err)
	}

	log.Warn("Snap installer is not fully implemented yet")
	log.Info("To manually install this package, run: sudo snap install %s", command)
	log.Info("You can also specify channels: sudo snap install %s --channel=stable", command)

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"snap", command,
		[]string{
			"Manual installation: sudo snap install " + command,
			"Install from edge: sudo snap install " + command + " --edge",
			"Install classic: sudo snap install " + command + " --classic",
			"Search snaps: snap find " + command,
		})
}

// Uninstall removes packages using snap
func (s *SnapInstaller) Uninstall(command string, repo types.Repository) error {
	if err := validateSnapSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which snap"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"snap", command,
				[]string{
					"Install snapd first",
					"Manual removal: sudo snap remove " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"snap", command, err)
	}

	log.Warn("Snap uninstaller is not fully implemented yet")
	log.Info("To manually remove this package, run: sudo snap remove %s", command)
	log.Info("Remove with data: sudo snap remove %s --purge", command)

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"snap", command,
		[]string{
			"Manual removal: sudo snap remove " + command,
			"Remove with data: sudo snap remove " + command + " --purge",
			"List installed: snap list",
			"View info: snap info " + command,
		})
}

// IsInstalled checks if a package is installed using snap
func (s *SnapInstaller) IsInstalled(command string) (bool, error) {
	if err := validateSnapSystem(); err != nil {
		return false, common.NewInstallerError(
			common.ErrorTypeSystemNotFound,
			"snap", command, err)
	}

	log.Warn("Snap IsInstalled is not fully implemented yet")
	log.Info("To manually check if this package is installed, run: snap list %s", command)

	return false, common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"snap", command,
		[]string{
			"Manual check: snap list " + command,
			"List all snaps: snap list",
			"Search snaps: snap find " + command,
		})
}

// validateSnapSystem validates that the snap system is available and functional
func validateSnapSystem() error {
	// Check if snap command exists
	if _, err := utils.CommandExec.RunShellCommand("which snap"); err != nil {
		return fmt.Errorf("snap command not found")
	}

	// Check if snapd is running
	if _, err := utils.CommandExec.RunShellCommand("systemctl is-active snapd 2>/dev/null"); err != nil {
		log.Debug("snapd service may not be running")
	}

	// Check if snap core is installed
	if _, err := utils.CommandExec.RunShellCommand("snap list core 2>/dev/null"); err != nil {
		log.Debug("snap core may not be installed")
	}

	return nil
}
