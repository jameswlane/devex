package rpm

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// RpmInstaller implements the Installer interface for RPM packages
type RpmInstaller struct{}

// NewRpmInstaller creates a new RpmInstaller instance
func NewRpmInstaller() *RpmInstaller {
	return &RpmInstaller{}
}

// Install installs packages using rpm
func (r *RpmInstaller) Install(command string, repo types.Repository) error {
	if err := validateRpmSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which rpm"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"rpm", command,
				[]string{
					"Install on RPM-based system (Red Hat, SUSE, etc.)",
					"Use DNF/YUM for online installation instead",
					"Manual installation: sudo rpm -i " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"rpm", command, err)
	}

	log.Warn("RPM installer is not fully implemented yet")
	log.Info("To manually install this package, run: sudo rpm -i %s", command)
	log.Info("Note: RPM requires local .rpm files. Use DNF/YUM for repository packages")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"rpm", command,
		[]string{
			"Manual installation: sudo rpm -i " + command,
			"Upgrade package: sudo rpm -U " + command,
			"For repository packages, use: sudo dnf install " + command,
			"For repository packages, use: sudo yum install " + command,
		})
}

// Uninstall removes packages using rpm
func (r *RpmInstaller) Uninstall(command string, repo types.Repository) error {
	if err := validateRpmSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which rpm"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"rpm", command,
				[]string{
					"Install on RPM-based system (Red Hat, SUSE, etc.)",
					"Manual removal: sudo rpm -e " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"rpm", command, err)
	}

	log.Warn("RPM uninstaller is not fully implemented yet")
	log.Info("To manually remove this package, run: sudo rpm -e %s", command)
	log.Info("Or use DNF/YUM for managed removal")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"rpm", command,
		[]string{
			"Manual removal: sudo rpm -e " + command,
			"Force removal: sudo rpm -e --force " + command,
			"For managed removal, use: sudo dnf remove " + command,
			"For managed removal, use: sudo yum remove " + command,
		})
}

// IsInstalled checks if a package is installed using rpm
func (r *RpmInstaller) IsInstalled(command string) (bool, error) {
	if err := validateRpmSystem(); err != nil {
		return false, common.NewInstallerError(
			common.ErrorTypeSystemNotFound,
			"rpm", command, err)
	}

	// Check if package is installed using rpm -q
	checkCommand := fmt.Sprintf("rpm -q %s", command)
	_, err := utils.CommandExec.RunShellCommand(checkCommand)
	if err == nil {
		return true, nil
	}

	// If rpm -q fails, assume not installed
	return false, nil
}

// validateRpmSystem validates that the RPM system is available and functional
func validateRpmSystem() error {
	// Check if rpm command exists
	if _, err := utils.CommandExec.RunShellCommand("which rpm"); err != nil {
		return fmt.Errorf("rpm command not found")
	}

	// Check if we're on an RPM-based system
	if _, err := utils.CommandExec.RunShellCommand("test -f /etc/redhat-release -o -f /etc/suse-release -o -f /etc/fedora-release"); err != nil {
		log.Debug("System may not be RPM-based")
	}

	// Check if RPM database exists
	if _, err := utils.CommandExec.RunShellCommand("test -d /var/lib/rpm"); err != nil {
		return fmt.Errorf("RPM database not found")
	}

	return nil
}
