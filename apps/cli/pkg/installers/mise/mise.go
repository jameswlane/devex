package mise

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// validMiseCommand ensures mise commands contain only safe characters for language/version specifications
var validMiseCommand = regexp.MustCompile(`^[a-zA-Z0-9@._-]+$`)

type MiseInstaller struct{}

func New() *MiseInstaller {
	return &MiseInstaller{}
}

func (m *MiseInstaller) Install(command string, repo types.Repository) error {
	log.Info("Mise Installer: Starting installation", "language", command)

	// Validate command to prevent injection attacks
	if !validMiseCommand.MatchString(command) {
		return fmt.Errorf("invalid mise command: %s (contains unsafe characters)", command)
	}

	// Additional validation: ensure command doesn't contain shell metacharacters
	if strings.ContainsAny(command, "|&;()<>$`\\\"' \t\n") {
		return fmt.Errorf("invalid mise command: %s (contains shell metacharacters)", command)
	}

	// Wrap the command into a types.AppConfig object
	appConfig := types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name: command,
		},
		InstallMethod:  "mise",
		InstallCommand: command,
	}

	// Check if the language is already installed
	isInstalled, err := utilities.IsAppInstalled(appConfig)
	if err != nil {
		log.Error("Failed to check if language is installed", err, "language", command)
		return fmt.Errorf("failed to check if language is installed: %w", err)
	}

	if isInstalled {
		log.Info("Language is already installed, skipping installation", "language", command)
		return nil
	}

	// Run `mise use --global` command with proper PATH and secure execution
	// Instead of using a shell command with string interpolation, build the command securely
	shellScript := `export PATH="$HOME/.local/bin:$PATH" && if command -v mise >/dev/null 2>&1; then mise use --global "` + command + `"; else echo "mise not found in PATH"; exit 1; fi`

	if _, err := utils.CommandExec.RunShellCommand("bash -c '" + strings.ReplaceAll(shellScript, "'", "'\"'\"'") + "'"); err != nil {
		log.Error("Failed to install language via Mise", err, "language", command)
		return fmt.Errorf("failed to install language via Mise '%s': %w", command, err)
	}

	log.Info("Language installed successfully via Mise", "language", command)

	// Add the language to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add language to repository", err, "language", command)
		return fmt.Errorf("failed to add language '%s' to repository: %w", command, err)
	}

	log.Info("Language added to repository successfully", "language", command)
	return nil
}
