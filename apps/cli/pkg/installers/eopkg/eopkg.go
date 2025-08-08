package eopkg

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// EopkgInstaller implements the Installer interface for Eopkg (Solus)
type EopkgInstaller struct{}

// NewEopkgInstaller creates a new EopkgInstaller instance
func NewEopkgInstaller() *EopkgInstaller {
	return &EopkgInstaller{}
}

// Install installs packages using eopkg
func (e *EopkgInstaller) Install(command string, repo types.Repository) error {
	log.Info("Eopkg installer not yet implemented")
	log.Info("Would run: eopkg install %s", command)
	return fmt.Errorf("eopkg installer not yet implemented")
}

// Uninstall removes packages using eopkg
func (e *EopkgInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Eopkg uninstaller not yet implemented")
	log.Info("Would run: eopkg remove %s", command)
	return fmt.Errorf("eopkg uninstaller not yet implemented")
}

// IsInstalled checks if a package is installed using eopkg
func (e *EopkgInstaller) IsInstalled(command string) (bool, error) {
	log.Info("Eopkg IsInstalled not yet implemented")
	return false, fmt.Errorf("eopkg IsInstalled not yet implemented")
}
