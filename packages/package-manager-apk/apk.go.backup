package apk

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// ApkInstaller implements the Installer interface for Alpine Package Keeper (apk)
type ApkInstaller struct{}

// NewApkInstaller creates a new ApkInstaller instance
func NewApkInstaller() *ApkInstaller {
	return &ApkInstaller{}
}

// Install installs packages using apk (Alpine Package Keeper)
func (a *ApkInstaller) Install(command string, repo types.Repository) error {
	log.Warn("APK installer is not yet fully implemented", "package", command)

	// Check if apk is available on the system
	if err := validateApkSystem(); err != nil {
		// Return appropriate structured error based on the validation failure
		if _, cmdErr := utils.CommandExec.RunShellCommand("which apk"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"apk",
				command,
				[]string{
					"Install Alpine Linux or run on Alpine-based system",
					"Use alternative package managers (apt, dnf, pacman) on other distributions",
				},
			)
		}
		return common.NewInstallerErrorWithCause(common.ErrorTypeSystemNotFunctional, "apk", command, err)
	}

	log.Info("APK system detected but installer needs implementation", "package", command)
	log.Info("Manual installation command: apk add %s", command)
	log.Info("Contributing: This installer needs implementation - PRs welcome!")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"apk",
		command,
		[]string{
			fmt.Sprintf("Run manually: sudo apk add %s", command),
			"Contribute to the project by implementing this installer",
			"Check Alpine Linux documentation for package installation",
		},
	)
}

// validateApkSystem checks if APK is available and functional
func validateApkSystem() error {
	// Check if apk command is available
	if _, err := utils.CommandExec.RunShellCommand("which apk"); err != nil {
		return fmt.Errorf("apk not found - not running on Alpine Linux: %w", err)
	}

	// Check if we can access apk (basic version check)
	if _, err := utils.CommandExec.RunShellCommand("apk --version"); err != nil {
		return fmt.Errorf("apk not functional: %w", err)
	}

	return nil
}

// Uninstall removes packages using apk
func (a *ApkInstaller) Uninstall(command string, repo types.Repository) error {
	log.Warn("APK uninstaller not yet implemented", "package", command)

	// Check if apk is available
	if err := validateApkSystem(); err != nil {
		return fmt.Errorf("apk system validation failed: %w", err)
	}

	log.Info("Manual uninstallation command: apk del %s", command)
	return fmt.Errorf("apk uninstaller not yet implemented - use manual command above")
}

// IsInstalled checks if a package is installed using apk
func (a *ApkInstaller) IsInstalled(command string) (bool, error) {
	log.Debug("APK IsInstalled not yet implemented", "package", command)

	// Check if apk is available
	if err := validateApkSystem(); err != nil {
		return false, fmt.Errorf("apk system validation failed: %w", err)
	}

	log.Debug("Manual check command: apk info -e %s", command)
	return false, fmt.Errorf("apk IsInstalled not yet implemented")
}
