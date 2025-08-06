package yay

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// YayInstaller implements the Installer interface for Yay (AUR helper)
type YayInstaller struct{}

// NewYayInstaller creates a new YayInstaller instance
func NewYayInstaller() *YayInstaller {
	return &YayInstaller{}
}

// Install installs packages using yay
func (y *YayInstaller) Install(command string, repo types.Repository) error {
	log.Info("Yay installer not yet implemented")
	log.Info("Would run: yay -S %s", command)
	return fmt.Errorf("yay installer not yet implemented")
}
