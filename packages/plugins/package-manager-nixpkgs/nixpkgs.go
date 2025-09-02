package nixpkgs

import (
	"fmt"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// NixpkgsInstaller implements the Installer interface for Nix package manager
type NixpkgsInstaller struct{}

// NewNixpkgsInstaller creates a new NixpkgsInstaller instance
func NewNixpkgsInstaller() *NixpkgsInstaller {
	return &NixpkgsInstaller{}
}

// Install installs packages using nix-env
func (n *NixpkgsInstaller) Install(command string, repo types.Repository) error {
	if err := validateNixpkgsSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which nix-env"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"nixpkgs", command,
				[]string{
					"Install Nix package manager: curl -L https://nixos.org/nix/install | sh",
					"Manual installation: nix-env -iA nixpkgs." + command,
					"Alternative: nix-env -i " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"nixpkgs", command, err)
	}

	log.Warn("Nixpkgs installer is not fully implemented yet")
	log.Info("To manually install this package, run: nix-env -iA nixpkgs.%s", command)
	log.Info("Or try: nix-env -i %s", command)

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"nixpkgs", command,
		[]string{
			"Manual installation: nix-env -iA nixpkgs." + command,
			"Alternative syntax: nix-env -i " + command,
			"Search packages: nix search nixpkgs " + command,
			"Update channels: nix-channel --update",
		})
}

// Uninstall removes packages using nix-env
func (n *NixpkgsInstaller) Uninstall(command string, repo types.Repository) error {
	if err := validateNixpkgsSystem(); err != nil {
		if _, cmdErr := utils.CommandExec.RunShellCommand("which nix-env"); cmdErr != nil {
			return common.NewInstallerErrorWithSuggestions(
				common.ErrorTypeSystemNotFound,
				"nixpkgs", command,
				[]string{
					"Install Nix package manager: curl -L https://nixos.org/nix/install | sh",
					"Manual removal: nix-env --uninstall " + command,
				})
		}
		return common.NewInstallerError(
			common.ErrorTypeValidationFailed,
			"nixpkgs", command, err)
	}

	log.Warn("Nixpkgs uninstaller is not fully implemented yet")
	log.Info("To manually remove this package, run: nix-env --uninstall %s", command)
	log.Info("Or by derivation: nix-env -e %s", command)

	return common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"nixpkgs", command,
		[]string{
			"Manual removal: nix-env --uninstall " + command,
			"Remove by derivation: nix-env -e " + command,
			"List installed: nix-env -q",
			"Rollback if needed: nix-env --rollback",
		})
}

// IsInstalled checks if a package is installed using nix-env
func (n *NixpkgsInstaller) IsInstalled(command string) (bool, error) {
	if err := validateNixpkgsSystem(); err != nil {
		return false, common.NewInstallerError(
			common.ErrorTypeSystemNotFound,
			"nixpkgs", command, err)
	}

	log.Warn("Nixpkgs IsInstalled is not fully implemented yet")
	log.Info("To manually check if this package is installed, run: nix-env -q | grep %s", command)

	return false, common.NewInstallerErrorWithSuggestions(
		common.ErrorTypeNotImplemented,
		"nixpkgs", command,
		[]string{
			"Manual check: nix-env -q | grep " + command,
			"Search packages: nix-env -qaP " + command,
			"Check store: which " + command + " | grep nix/store",
		})
}

// validateNixpkgsSystem validates that the nix-env system is available and functional
func validateNixpkgsSystem() error {
	// Check if nix-env command exists
	if _, err := utils.CommandExec.RunShellCommand("which nix-env"); err != nil {
		return fmt.Errorf("nix-env command not found")
	}

	// Check if nix channels are configured
	if output, err := utils.CommandExec.RunShellCommand("nix-channel --list 2>/dev/null | grep -q nixpkgs"); err != nil || output == "" {
		log.Debug("nixpkgs channel may not be configured")
	}

	// Check if nix store is accessible
	if _, err := utils.CommandExec.RunShellCommand("test -d /nix/store"); err != nil {
		return fmt.Errorf("nix store not found")
	}

	return nil
}
