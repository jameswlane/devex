package rpm

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// RpmInstaller implements the Installer interface for RPM packages
type RpmInstaller struct{}

// NewRpmInstaller creates a new RpmInstaller instance
func NewRpmInstaller() *RpmInstaller {
	return &RpmInstaller{}
}

// Install installs packages using rpm
func (r *RpmInstaller) Install(command string, repo types.Repository) error {
	log.Info("RPM installer not yet implemented")
	log.Info("Would run: rpm -i %s", command)
	return fmt.Errorf("rpm installer not yet implemented")
}

// Uninstall removes packages using rpm
func (r *RpmInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("RPM uninstaller not yet implemented")
	log.Info("Would run: rpm -e %s", command)
	return fmt.Errorf("rpm uninstaller not yet implemented")
}

// IsInstalled checks if a package is installed using rpm
func (r *RpmInstaller) IsInstalled(command string) (bool, error) {
	log.Info("RPM IsInstalled not yet implemented")
	return false, fmt.Errorf("rpm IsInstalled not yet implemented")
}
