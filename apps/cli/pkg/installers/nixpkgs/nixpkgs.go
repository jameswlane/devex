package nixpkgs

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// NixpkgsInstaller implements the Installer interface for Nix package manager
type NixpkgsInstaller struct{}

// NewNixpkgsInstaller creates a new NixpkgsInstaller instance
func NewNixpkgsInstaller() *NixpkgsInstaller {
	return &NixpkgsInstaller{}
}

// Install installs packages using nix-env
func (n *NixpkgsInstaller) Install(command string, repo types.Repository) error {
	log.Info("Nixpkgs installer not yet implemented")
	log.Info("Would run: nix-env -iA nixpkgs.%s", command)
	return fmt.Errorf("nixpkgs installer not yet implemented")
}
