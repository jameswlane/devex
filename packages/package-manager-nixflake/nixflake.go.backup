package nixflake

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// NixFlakeInstaller implements the Installer interface for Nix Flakes
type NixFlakeInstaller struct{}

// NewNixFlakeInstaller creates a new NixFlakeInstaller instance
func NewNixFlakeInstaller() *NixFlakeInstaller {
	return &NixFlakeInstaller{}
}

// Install installs packages using nix flake
func (n *NixFlakeInstaller) Install(command string, repo types.Repository) error {
	if err := validateNixSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which nix"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"nix-flake", command,
				[]string{
					"Install Nix package manager: curl -L https://nixos.org/nix/install | sh",
					"Enable flakes: echo 'experimental-features = nix-command flakes' >> ~/.config/nix/nix.conf",
					"Manual installation: nix profile install " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"nix-flake", command, err)
	}

	log.Warn("Nix Flake installer is not fully implemented yet")
	log.Info("To manually install this package, run: nix profile install %s", command)
	log.Info("Ensure flakes are enabled in your Nix configuration")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"nix-flake", command,
		[]string{
			"Manual installation: nix profile install " + command,
			"Enable flakes: echo 'experimental-features = nix-command flakes' >> ~/.config/nix/nix.conf",
			"Search flake: nix search nixpkgs " + command,
			"Show flake info: nix flake show " + command,
		})
}

// Uninstall removes packages using nix profile
func (n *NixFlakeInstaller) Uninstall(command string, repo types.Repository) error {
	if err := validateNixSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which nix"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"nix-flake", command,
				[]string{
					"Install Nix package manager: curl -L https://nixos.org/nix/install | sh",
					"Manual removal: nix profile remove " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"nix-flake", command, err)
	}

	log.Warn("Nix Flake uninstaller is not fully implemented yet")
	log.Info("To manually remove this package, run: nix profile remove %s", command)
	log.Info("List installed packages: nix profile list")

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"nix-flake", command,
		[]string{
			"Manual removal: nix profile remove " + command,
			"List installed: nix profile list",
			"Remove by index: nix profile remove <index>",
			"Cleanup old generations: nix profile wipe-history",
		})
}

// IsInstalled checks if a package is installed using nix profile
func (n *NixFlakeInstaller) IsInstalled(command string) (bool, error) {
	if err := validateNixSystem(); err != nil {
		return false, common.NewInstallerError(
			common.ErrorTypeSystemNotFound,
			"nix-flake", command, err)
	}

	log.Warn("Nix Flake IsInstalled is not fully implemented yet")
	log.Info("To manually check if this package is installed, run: nix profile list | grep %s", command)

	return false, common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"nix-flake", command,
		[]string{
			"Manual check: nix profile list | grep " + command,
			"Search flakes: nix search nixpkgs " + command,
			"Check store: which <pkg_name> | grep nix/store",
		})
}

// validateNixSystem validates that the Nix system is available and functional
func validateNixSystem() error {
	// Check if nix command exists
	if _, err := utils.CommandExec.RunShellCommand("which nix"); err != nil {
		return fmt.Errorf("nix command not found")
	}

	// Check if flakes are enabled
	if output, err := utils.CommandExec.RunShellCommand("nix --version 2>&1"); err == nil {
		if !strings.Contains(output, "flake") {
			// Check if flakes are enabled in config
			if _, err := utils.CommandExec.RunShellCommand("nix flake --version 2>/dev/null"); err != nil {
				log.Debug("Nix flakes may not be enabled")
			}
		}
	}

	// Check if nix store is accessible
	if _, err := utils.CommandExec.RunShellCommand("test -d /nix/store"); err != nil {
		return fmt.Errorf("nix store not found")
	}

	return nil
}
