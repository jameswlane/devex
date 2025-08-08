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

// Uninstall removes packages using nix profile
func (n *NixFlakeInstaller) Uninstall(command string, repo types.Repository) error {
	log.Info("Nix Flake uninstaller not yet implemented")
	log.Info("Would run: nix profile remove %s", command)
	return fmt.Errorf("nix flake uninstaller not yet implemented")
}

// IsInstalled checks if a package is installed using nix profile
func (n *NixFlakeInstaller) IsInstalled(command string) (bool, error) {
	log.Info("Nix Flake IsInstalled not yet implemented")
	return false, fmt.Errorf("nix flake IsInstalled not yet implemented")
}
