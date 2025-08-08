package snap

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// SnapInstaller implements the Installer interface for Snap packages
type SnapInstaller struct{}

// NewSnapInstaller creates a new SnapInstaller instance
func NewSnapInstaller() *SnapInstaller {
	return &SnapInstaller{}
}

// Install installs packages using snap
func (s *SnapInstaller) Install(command string, repo types.Repository) error {
	log.Info("Snap installer not yet implemented")
	log.Info("Would run: snap install %s", command)
	return fmt.Errorf("snap installer not yet implemented")
}

// Uninstall removes packages using snap
func (s *SnapInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Snap uninstaller not yet implemented")
	log.Info("Would run: snap remove %s", command)
	return fmt.Errorf("snap uninstaller not yet implemented")
}

// IsInstalled checks if a package is installed using snap
func (s *SnapInstaller) IsInstalled(command string) (bool, error) {
	log.Info("Snap IsInstalled not yet implemented")
	return false, fmt.Errorf("snap IsInstalled not yet implemented")
}
