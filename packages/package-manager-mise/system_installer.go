package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// HandleEnsureInstalled ensures Mise is installed on the system
func (m *MisePlugin) HandleEnsureInstalled(ctx context.Context, args []string) error {
	m.logger.Println("Checking if Mise is installed...")

	// Check if mise is already in PATH
	if err := sdk.ExecCommandWithContext(ctx, false, "which", "mise"); err == nil {
		m.logger.Success("Mise is already installed")
		return nil
	}

	m.logger.Println("Mise not found, installing...")

	// Install Mise using the official installation script
	if err := m.installMise(ctx); err != nil {
		return fmt.Errorf("failed to install Mise: %w", err)
	}

	// Configure shell integration
	if err := m.configureShellIntegration(); err != nil {
		m.logger.Warning("Failed to configure shell integration: %v", err)
		m.logger.Println("You may need to manually add Mise to your shell configuration")
	}

	m.logger.Success("Mise installed successfully")
	return nil
}

// installMise installs Mise using the official installation script
func (m *MisePlugin) installMise(ctx context.Context) error {
	m.logger.Println("Downloading and installing Mise...")

	// Use curl to download and execute the installation script
	installCmd := "curl https://mise.run | sh"
	if err := sdk.ExecCommandWithContext(ctx, false, "sh", "-c", installCmd); err != nil {
		return fmt.Errorf("failed to download and install Mise: %w", err)
	}

	return nil
}

// configureShellIntegration configures shell integration for Mise
func (m *MisePlugin) configureShellIntegration() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Detect shell and configure accordingly
	shell, err := sdk.SafeGetEnvWithDefault("SHELL", "/bin/bash")
	if err != nil {
		m.logger.Warning("SHELL environment variable validation failed: %v, using default", err)
		shell = "/bin/bash"
	}

	shellName := filepath.Base(shell)

	switch shellName {
	case "bash":
		return m.configureBashIntegration(homeDir)
	case "zsh":
		return m.configureZshIntegration(homeDir)
	case "fish":
		return m.configureFishIntegration(homeDir)
	default:
		return fmt.Errorf("unsupported shell: %s", shellName)
	}
}

// configureBashIntegration configures Mise integration for Bash
func (m *MisePlugin) configureBashIntegration(homeDir string) error {
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	bashProfilePath := filepath.Join(homeDir, ".bash_profile")

	// Check if already configured
	if m.isAlreadyConfigured(bashrcPath) || m.isAlreadyConfigured(bashProfilePath) {
		return nil
	}

	// Add configuration to .bashrc
	config := `
# Mise configuration
if command -v mise >/dev/null 2>&1; then
  eval "$(mise activate bash)"
fi
`

	return m.appendToFile(bashrcPath, config)
}

// configureZshIntegration configures Mise integration for Zsh
func (m *MisePlugin) configureZshIntegration(homeDir string) error {
	zshrcPath := filepath.Join(homeDir, ".zshrc")

	// Check if already configured
	if m.isAlreadyConfigured(zshrcPath) {
		return nil
	}

	// Add configuration to .zshrc
	config := `
# Mise configuration
if command -v mise >/dev/null 2>&1; then
  eval "$(mise activate zsh)"
fi
`

	return m.appendToFile(zshrcPath, config)
}

// configureFishIntegration configures Mise integration for Fish
func (m *MisePlugin) configureFishIntegration(homeDir string) error {
	fishConfigDir := filepath.Join(homeDir, ".config", "fish")
	configPath := filepath.Join(fishConfigDir, "config.fish")

	// Create fish config directory if it doesn't exist
	if err := os.MkdirAll(fishConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create fish config directory: %w", err)
	}

	// Check if already configured
	if m.isAlreadyConfigured(configPath) {
		return nil
	}

	// Add configuration to config.fish
	config := `
# Mise configuration
if command -v mise >/dev/null 2>&1
  mise activate fish | source
end
`

	return m.appendToFile(configPath, config)
}

// isAlreadyConfigured checks if Mise is already configured in a shell file
func (m *MisePlugin) isAlreadyConfigured(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	return strings.Contains(string(content), "mise activate") || strings.Contains(string(content), "eval \"$(mise")
}

// appendToFile appends content to a file
func (m *MisePlugin) appendToFile(filePath, content string) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filePath, err)
	}

	return nil
}
