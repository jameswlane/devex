package emerge

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// EmergeInstaller implements the Installer interface for Portage (Gentoo)
type EmergeInstaller struct{}

// NewEmergeInstaller creates a new EmergeInstaller instance
func NewEmergeInstaller() *EmergeInstaller {
	return &EmergeInstaller{}
}

// Install installs packages using emerge
func (e *EmergeInstaller) Install(command string, repo types.Repository) error {
	log.Info("Emerge installer not yet implemented")
	log.Info("Would run: emerge %s", command)
	return fmt.Errorf("emerge installer not yet implemented")
}

// Uninstall removes packages using emerge
func (e *EmergeInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Emerge uninstaller not yet implemented")
	log.Info("Would run: emerge --unmerge %s", command)
	return fmt.Errorf("emerge uninstaller not yet implemented")
}

// IsInstalled checks if a package is installed using emerge
func (e *EmergeInstaller) IsInstalled(command string) (bool, error) {
	log.Info("Emerge IsInstalled not yet implemented")
	return false, fmt.Errorf("emerge IsInstalled not yet implemented")
}
