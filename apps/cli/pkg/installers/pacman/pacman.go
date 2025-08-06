package pacman

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// PacmanInstaller implements the Installer interface for Pacman (Arch Linux)
type PacmanInstaller struct{}

// NewPacmanInstaller creates a new PacmanInstaller instance
func NewPacmanInstaller() *PacmanInstaller {
	return &PacmanInstaller{}
}

// Install installs packages using pacman
func (p *PacmanInstaller) Install(command string, repo types.Repository) error {
	log.Info("Pacman installer not yet implemented")
	log.Info("Would run: pacman -S %s", command)
	return fmt.Errorf("pacman installer not yet implemented")
}
