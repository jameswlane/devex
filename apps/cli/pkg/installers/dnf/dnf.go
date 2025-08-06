package dnf

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// DnfInstaller implements the Installer interface for DNF (Fedora/RHEL)
type DnfInstaller struct{}

// NewDnfInstaller creates a new DnfInstaller instance
func NewDnfInstaller() *DnfInstaller {
	return &DnfInstaller{}
}

// Install installs packages using dnf
func (d *DnfInstaller) Install(command string, repo types.Repository) error {
	log.Info("DNF installer not yet implemented")
	log.Info("Would run: dnf install %s", command)
	return fmt.Errorf("dnf installer not yet implemented")
}
