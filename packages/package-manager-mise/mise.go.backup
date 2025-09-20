package mise

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/log"
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
	log.Debug("Mise Installer: Starting installation", "language", command)

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
	// Use structured command execution to avoid shell injection vulnerabilities
	miseScript := fmt.Sprintf(`export PATH="$HOME/.local/bin:$PATH" && if command -v mise >/dev/null 2>&1; then mise use --global %s; else echo "mise not found in PATH"; exit 1; fi`,
		strings.ReplaceAll(command, "'", "'\"'\"'")) // Escape single quotes properly

	if _, err := utils.CommandExec.RunShellCommand("bash -c '" + strings.ReplaceAll(miseScript, "'", "'\"'\"'") + "'"); err != nil {
		log.Error("Failed to install language via Mise", err, "language", command)
		return fmt.Errorf("failed to install language via Mise '%s': %w", command, err)
	}

	log.Debug("Language installed successfully via Mise", "language", command)

	// Add the language to the repository
	if err := repo.AddApp(command); err != nil {
		log.Error("Failed to add language to repository", err, "language", command)
		return fmt.Errorf("failed to add language '%s' to repository: %w", command, err)
	}

	log.Debug("Language added to repository successfully", "language", command)
	return nil
}

// Uninstall removes languages using mise
func (m *MiseInstaller) Uninstall(command string, repo types.Repository) error {
	log.Debug("Mise Installer: Starting uninstallation", "language", command)

	// Validate command to prevent injection attacks
	if !validMiseCommand.MatchString(command) {
		return fmt.Errorf("invalid mise command: %s (contains unsafe characters)", command)
	}

	// Additional validation: ensure command doesn't contain shell metacharacters
	if strings.ContainsAny(command, "|&;()<>$`\\\"' \t\n") {
		return fmt.Errorf("invalid mise command: %s (contains shell metacharacters)", command)
	}

	// Check if the language is installed
	isInstalled, err := m.IsInstalled(command)
	if err != nil {
		log.Error("Failed to check if language is installed", err, "language", command)
		return fmt.Errorf("failed to check if language is installed: %w", err)
	}

	if !isInstalled {
		log.Info("Language not installed, skipping uninstallation", "language", command)
		return nil
	}

	// Run `mise uninstall` command with proper PATH and secure execution
	miseScript := fmt.Sprintf(`export PATH="$HOME/.local/bin:$PATH" && if command -v mise >/dev/null 2>&1; then mise uninstall %s; else echo "mise not found in PATH"; exit 1; fi`,
		strings.ReplaceAll(command, "'", "'\"'\"'")) // Escape single quotes properly

	if _, err := utils.CommandExec.RunShellCommand("bash -c '" + strings.ReplaceAll(miseScript, "'", "'\"'\"'") + "'"); err != nil {
		log.Error("Failed to uninstall language via Mise", err, "language", command)
		return fmt.Errorf("failed to uninstall language via Mise '%s': %w", command, err)
	}

	log.Debug("Language uninstalled successfully via Mise", "language", command)

	// Remove the language from the repository
	if err := repo.DeleteApp(command); err != nil {
		log.Error("Failed to remove language from repository", err, "language", command)
		return fmt.Errorf("failed to remove language from repository: %w", err)
	}

	log.Debug("Language removed from repository successfully", "language", command)
	return nil
}

// IsInstalled checks if a language is installed using mise
func (m *MiseInstaller) IsInstalled(command string) (bool, error) {
	// Validate command to prevent injection attacks
	if !validMiseCommand.MatchString(command) {
		return false, fmt.Errorf("invalid mise command: %s (contains unsafe characters)", command)
	}

	// Additional validation: ensure command doesn't contain shell metacharacters
	if strings.ContainsAny(command, "|&;()<>$`\\\"' \t\n") {
		return false, fmt.Errorf("invalid mise command: %s (contains shell metacharacters)", command)
	}

	// Check if the language/version is installed using mise list
	miseScript := fmt.Sprintf(`export PATH="$HOME/.local/bin:$PATH" && if command -v mise >/dev/null 2>&1; then mise list %s; else echo "mise not found in PATH"; exit 1; fi`,
		strings.ReplaceAll(strings.Split(command, "@")[0], "'", "'\"'\"'")) // Extract language name and escape single quotes

	output, err := utils.CommandExec.RunShellCommand("bash -c '" + strings.ReplaceAll(miseScript, "'", "'\"'\"'") + "'")
	if err != nil {
		// If mise list fails, language is likely not installed
		return false, nil
	}

	// Check if the output contains the requested version
	return strings.Contains(output, command) || strings.Contains(output, strings.Split(command, "@")[0]), nil
}
