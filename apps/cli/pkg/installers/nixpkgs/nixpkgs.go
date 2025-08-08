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

// Uninstall removes packages using nix-env
func (n *NixpkgsInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Nixpkgs uninstaller not yet implemented")
	log.Info("Would run: nix-env --uninstall %s", command)
	return fmt.Errorf("nixpkgs uninstaller not yet implemented")
}

// IsInstalled checks if a package is installed using nix-env
func (n *NixpkgsInstaller) IsInstalled(command string) (bool, error) {
	log.Info("Nixpkgs IsInstalled not yet implemented")
	return false, fmt.Errorf("nixpkgs IsInstalled not yet implemented")
}
