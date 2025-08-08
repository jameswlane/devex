package xbps

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// XbpsInstaller implements the Installer interface for XBPS (Void Linux)
type XbpsInstaller struct{}

// NewXbpsInstaller creates a new XbpsInstaller instance
func NewXbpsInstaller() *XbpsInstaller {
	return &XbpsInstaller{}
}

// Install installs packages using xbps-install
func (x *XbpsInstaller) Install(command string, repo types.Repository) error {
	log.Info("XBPS installer not yet implemented")
	log.Info("Would run: xbps-install -S %s", command)
	return fmt.Errorf("xbps installer not yet implemented")
}

// Uninstall removes packages using xbps-remove
func (x *XbpsInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("XBPS uninstaller not yet implemented")
	log.Info("Would run: xbps-remove %s", command)
	return fmt.Errorf("xbps uninstaller not yet implemented")
}

// IsInstalled checks if a package is installed using xbps-query
func (x *XbpsInstaller) IsInstalled(command string) (bool, error) {
	log.Info("XBPS IsInstalled not yet implemented")
	return false, fmt.Errorf("xbps IsInstalled not yet implemented")
}
