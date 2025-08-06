package zypper

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// ZypperInstaller implements the Installer interface for Zypper (openSUSE)
type ZypperInstaller struct{}

// NewZypperInstaller creates a new ZypperInstaller instance
func NewZypperInstaller() *ZypperInstaller {
	return &ZypperInstaller{}
}

// Install installs packages using zypper
func (z *ZypperInstaller) Install(command string, repo types.Repository) error {
	log.Info("Zypper installer not yet implemented")
	log.Info("Would run: zypper install %s", command)
	return fmt.Errorf("zypper installer not yet implemented")
}
