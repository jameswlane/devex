package nixflake

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// NixFlakeInstaller implements the Installer interface for Nix Flakes
type NixFlakeInstaller struct{}

// NewNixFlakeInstaller creates a new NixFlakeInstaller instance
func NewNixFlakeInstaller() *NixFlakeInstaller {
	return &NixFlakeInstaller{}
}

// Install installs packages using nix flake
func (n *NixFlakeInstaller) Install(command string, repo types.Repository) error {
	log.Info("Nix Flake installer not yet implemented")
	log.Info("Would run: nix profile install %s", command)
	return fmt.Errorf("nix flake installer not yet implemented")
}
