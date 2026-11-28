package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// HandleEnsureInstalled ensures Mise is installed on the system
func (m *MisePlugin) HandleEnsureInstalled(ctx context.Context, args []string) error {
	return m.ensureInstalled(ctx, sdk.NewChanges(false))
}

// Describe reports what ensure-installed would change, from the same code path
// it runs.
func (m *MisePlugin) Describe(ctx context.Context, command string, args []string) (sdk.Description, error) {
	if command != "ensure-installed" {
		return sdk.Description{}, sdk.ErrDescribeUnsupported
	}
	addUserBinToPath()
	changes := sdk.NewChanges(true)
	if err := m.ensureInstalled(ctx, changes); err != nil {
		return sdk.Description{}, err
	}
	return changes.Description(m.Info().Name, command, args), nil
}

// ensureInstalled installs Mise when it is missing and configures shell
// integration, running or only recording each change through changes.
func (m *MisePlugin) ensureInstalled(ctx context.Context, changes *sdk.Changes) error {
	m.logger.Println("Checking if Mise is installed...")

	// Check if mise is already in PATH
	if err := sdk.ExecCommandWithContext(ctx, false, "which", "mise"); err == nil {
		m.logger.Success("Mise is already installed")
		m.configureShell(changes)
		return nil
	}

	if !changes.Describing() {
		m.logger.Println("Mise not found, installing...")
	}

	// Install Mise using the official installation script
	changes.Download(sdk.PlannedDownload{URL: sdk.MiseInstallScriptURL, Verification: sdk.MiseInstallVerification()})
	// The installer honors MISE_INSTALL_PATH, which the plugin inherits.
	destination := os.Getenv("MISE_INSTALL_PATH")
	if destination == "" {
		destination = "~/.local/bin/mise"
	}
	if err := changes.Do(sdk.PlannedAction{
		Kind:   sdk.ActionShell,
		Target: "sh <verified mise installer>",
		Scope:  sdk.ScopeUser,
		Detail: "installs mise to " + destination,
	}, func() error { return m.installMise(ctx) }); err != nil {
		return fmt.Errorf("failed to install Mise: %w", err)
	}

	m.configureShell(changes)

	if !changes.Describing() {
		m.logger.Success("Mise installed successfully")
	}
	return nil
}

// configureShell adds shell integration, warning instead of failing.
func (m *MisePlugin) configureShell(changes *sdk.Changes) {
	if err := m.configureShellIntegration(changes); err != nil {
		m.logger.Warning("Failed to configure shell integration: %v", err)
		m.logger.Println("You may need to manually add Mise to your shell configuration")
		changes.Note(fmt.Sprintf("Shell integration was not configured (%v); add `mise activate` to your shell config yourself", err))
	}
}

// maxMiseInstallerSize bounds the downloaded installer script.
const maxMiseInstallerSize = 5 * 1024 * 1024

// installMise installs Mise using the official installation script after
// verifying its signature from mise's pinned release key.
func (m *MisePlugin) installMise(ctx context.Context) error {
	return m.installMiseFrom(ctx, sdk.MiseInstallScriptURL, sdk.MiseInstallVerification())
}

// installMiseFrom downloads the installer, verifies it, and only then runs it.
// The verified script checks the mise binary against the checksums it embeds.
func (m *MisePlugin) installMiseFrom(ctx context.Context, scriptURL string, verification sdk.ArtifactVerification) error {
	m.logger.Println("Downloading and verifying the Mise installer...")

	script, err := os.CreateTemp("", "devex-mise-install-*.sh")
	if err != nil {
		return fmt.Errorf("failed to create installer file: %w", err)
	}
	defer func() { _ = os.Remove(script.Name()) }()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, scriptURL, nil)
	if err != nil {
		_ = script.Close()
		return fmt.Errorf("failed to create installer request: %w", err)
	}
	response, err := (&http.Client{Timeout: 60 * time.Second}).Do(request)
	if err != nil {
		_ = script.Close()
		return fmt.Errorf("failed to download the Mise installer: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		_ = script.Close()
		return fmt.Errorf("failed to download the Mise installer: HTTP %d", response.StatusCode)
	}
	written, copyErr := io.Copy(script, io.LimitReader(response.Body, maxMiseInstallerSize+1))
	closeErr := script.Close()
	if copyErr != nil {
		return fmt.Errorf("failed to download the Mise installer: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to write the Mise installer: %w", closeErr)
	}
	if written > maxMiseInstallerSize {
		return fmt.Errorf("the Mise installer exceeds %d bytes", maxMiseInstallerSize)
	}

	result, err := sdk.VerifyArtifact(ctx, script.Name(), verification)
	if err != nil {
		return fmt.Errorf("mise installer verification failed; not running it: %w", err)
	}
	if !result.SignatureVerified && !result.ChecksumVerified {
		return fmt.Errorf("mise installer verification was skipped; not running it")
	}
	m.logger.Println("Verified the Mise installer (checksum=%t signature=%t)", result.ChecksumVerified, result.SignatureVerified)

	if err := sdk.ExecCommandWithContext(ctx, false, "sh", script.Name()); err != nil {
		return fmt.Errorf("failed to install Mise: %w", err)
	}
	return nil
}

// configureShellIntegration configures shell integration for Mise
func (m *MisePlugin) configureShellIntegration(changes *sdk.Changes) error {
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
		return m.configureBashIntegration(changes, homeDir)
	case "zsh":
		return m.configureZshIntegration(changes, homeDir)
	case "fish":
		return m.configureFishIntegration(changes, homeDir)
	default:
		return fmt.Errorf("unsupported shell: %s", shellName)
	}
}

// configureBashIntegration configures Mise integration for Bash
func (m *MisePlugin) configureBashIntegration(changes *sdk.Changes, homeDir string) error {
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	bashProfilePath := filepath.Join(homeDir, ".bash_profile")

	// Check if already configured
	if m.isAlreadyConfigured(bashrcPath) || m.isAlreadyConfigured(bashProfilePath) {
		return nil
	}

	// The installer puts mise in ~/.local/bin, which is often not on PATH yet
	// when .bashrc runs, so fall back to that path.
	config := `
# Mise configuration
if command -v mise >/dev/null 2>&1; then
  eval "$(mise activate bash)"
elif [ -x "$HOME/.local/bin/mise" ]; then
  eval "$("$HOME/.local/bin/mise" activate bash)"
fi
`

	return m.appendActivation(changes, bashrcPath, config)
}

// configureZshIntegration configures Mise integration for Zsh
func (m *MisePlugin) configureZshIntegration(changes *sdk.Changes, homeDir string) error {
	zshrcPath := filepath.Join(homeDir, ".zshrc")

	// Check if already configured
	if m.isAlreadyConfigured(zshrcPath) {
		return nil
	}

	// The installer puts mise in ~/.local/bin, which is often not on PATH yet
	// when .zshrc runs, so fall back to that path.
	config := `
# Mise configuration
if command -v mise >/dev/null 2>&1; then
  eval "$(mise activate zsh)"
elif [ -x "$HOME/.local/bin/mise" ]; then
  eval "$("$HOME/.local/bin/mise" activate zsh)"
fi
`

	return m.appendActivation(changes, zshrcPath, config)
}

// configureFishIntegration configures Mise integration for Fish
func (m *MisePlugin) configureFishIntegration(changes *sdk.Changes, homeDir string) error {
	fishConfigDir := filepath.Join(homeDir, ".config", "fish")
	configPath := filepath.Join(fishConfigDir, "config.fish")

	// Check if already configured
	if m.isAlreadyConfigured(configPath) {
		return nil
	}

	// The installer puts mise in ~/.local/bin, which is often not on PATH yet
	// when config.fish runs, so fall back to that path.
	config := `
# Mise configuration
if command -v mise >/dev/null 2>&1
  mise activate fish | source
else if test -x "$HOME/.local/bin/mise"
  "$HOME/.local/bin/mise" activate fish | source
end
`

	return m.appendActivation(changes, configPath, config, func() error {
		// Create fish config directory if it doesn't exist
		if err := os.MkdirAll(fishConfigDir, 0755); err != nil {
			return fmt.Errorf("failed to create fish config directory: %w", err)
		}
		return nil
	})
}

// appendActivation appends mise activation to an rc file through changes,
// running prepare first.
func (m *MisePlugin) appendActivation(changes *sdk.Changes, filePath, content string, prepare ...func() error) error {
	return changes.Do(sdk.PlannedAction{
		Kind:   sdk.ActionFile,
		Target: filePath,
		Scope:  sdk.ScopeUser,
		Detail: "append mise activation",
	}, func() error {
		for _, step := range prepare {
			if err := step(); err != nil {
				return err
			}
		}
		return m.appendToFile(filePath, content)
	})
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
