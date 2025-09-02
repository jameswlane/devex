package emerge

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// EmergeInstaller implements the Installer interface for Portage (Gentoo)
type EmergeInstaller struct{}

// NewEmergeInstaller creates a new EmergeInstaller instance
func NewEmergeInstaller() *EmergeInstaller {
	return &EmergeInstaller{}
}

// Install installs packages using emerge
func (e *EmergeInstaller) Install(command string, repo types.Repository) error {
	if err := validateEmergeSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which emerge"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"emerge", command,
				[]string{
					"Install Gentoo Linux or run on Gentoo-based system",
					"Manual installation: emerge " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"emerge", command, err)
	}

	log.Warn("Emerge installer is not fully implemented yet")
	log.Info("To manually install this package, run: emerge %s", command)
	log.Info("You may need to unmask the package or update your portage tree first")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"emerge", command,
		[]string{
			"Manual installation: emerge " + command,
			"Update portage tree: emerge --sync",
			"Unmask if needed: echo '>=category/package-version' >> /etc/portage/package.accept_keywords",
		})
}

// Uninstall removes packages using emerge
func (e *EmergeInstaller) Uninstall(command string, repo types.Repository) error {
	if err := validateEmergeSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which emerge"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"emerge", command,
				[]string{
					"Install Gentoo Linux or run on Gentoo-based system",
					"Manual removal: emerge --unmerge " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"emerge", command, err)
	}

	log.Warn("Emerge uninstaller is not fully implemented yet")
	log.Info("To manually remove this package, run: emerge --unmerge %s", command)
	log.Info("Or use: emerge --depclean %s for a clean removal", command)

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"emerge", command,
		[]string{
			"Manual removal: emerge --unmerge " + command,
			"Clean removal: emerge --depclean " + command,
			"Check dependencies: emerge --pretend --depclean " + command,
		})
}

// IsInstalled checks if a package is installed using emerge
func (e *EmergeInstaller) IsInstalled(command string) (bool, error) {
	if err := validateEmergeSystem(); err != nil {
		return false, common.NewInstallerError(
			common.ErrorTypeSystemNotFound,
			"emerge", command, err)
	}

	log.Warn("Emerge IsInstalled is not fully implemented yet")
	log.Info("To manually check if this package is installed, run: qlist -I | grep %s", command)

	return false, common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"emerge", command,
		[]string{
			"Manual check: qlist -I | grep " + command,
			"Search packages: emerge --search " + command,
			"Check if installed: emerge --pretend " + command,
		})
}

// validateEmergeSystem validates that the emerge system is available and functional
func validateEmergeSystem() error {
	// Check if emerge command exists
	if _, err := utils.CommandExec.RunShellCommand("which emerge"); err != nil {
		return fmt.Errorf("emerge command not found")
	}

	// Check if we're on a Gentoo-based system
	if output, err := utils.CommandExec.RunShellCommand("cat /etc/os-release 2>/dev/null | grep -i gentoo"); err != nil || output == "" {
		// Not necessarily an error, but worth noting
		log.Debug("System may not be Gentoo-based")
	}

	// Check if portage directory exists
	if _, err := utils.CommandExec.RunShellCommand("test -d /var/db/pkg"); err != nil {
		return fmt.Errorf("portage package database not found")
	}

	return nil
}
