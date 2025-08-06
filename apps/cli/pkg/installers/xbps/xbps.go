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
