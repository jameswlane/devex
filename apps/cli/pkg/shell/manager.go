package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// ShellManager handles comprehensive shell setup and management
type ShellManager struct {
	homeDir    string
	assetsDir  string
	settings   config.CrossPlatformSettings
	repository types.Repository
}

// NewShellManager creates a new shell manager instance
func NewShellManager(settings config.CrossPlatformSettings, repository types.Repository) *ShellManager {
	homeDir := os.Getenv("HOME")
	assetsDir := detectAssetsDir()

	return &ShellManager{
		homeDir:    homeDir,
		assetsDir:  assetsDir,
		settings:   settings,
		repository: repository,
	}
}

// SetupShell provides comprehensive shell setup: verify, install, configure, and switch
func (sm *ShellManager) SetupShell(ctx context.Context, shellName string) error {
	log.Info("Starting comprehensive shell setup", "shell", shellName)

	// 1. Verify shell exists or install it
	if err := sm.ensureShellInstalled(ctx, shellName); err != nil {
		return fmt.Errorf("failed to ensure shell is installed: %w", err)
	}

	// 2. Backup existing shell configuration
	if err := sm.backupExistingConfig(shellName); err != nil {
		log.Warn("Failed to backup existing configuration", "shell", shellName, "error", err)
	}

	// 3. Deploy DevEx shell configuration
	if err := sm.deployShellConfiguration(shellName); err != nil {
		return fmt.Errorf("failed to deploy shell configuration: %w", err)
	}

	// 4. Switch default shell if needed
	if err := sm.switchToShell(ctx, shellName); err != nil {
		return fmt.Errorf("failed to switch to shell: %w", err)
	}

	// 5. Add shell activation hint
	sm.addShellActivationHint(shellName)

	log.Info("Shell setup completed successfully", "shell", shellName)
	return nil
}

// ensureShellInstalled checks if shell exists and installs if necessary
func (sm *ShellManager) ensureShellInstalled(ctx context.Context, shellName string) error {
	// Check if shell is already available
	if sm.isShellAvailable(shellName) {
		log.Info("Shell already available", "shell", shellName)
		return nil
	}

	log.Info("Installing shell", "shell", shellName)

	// Get shell app from configuration
	allApps := sm.settings.GetAllApps()
	for _, app := range allApps {
		if app.Name == shellName {
			return sm.installShellApp(ctx, app)
		}
	}

	// Fallback to system package manager
	return sm.installShellViaSystem(ctx, shellName)
}

// isShellAvailable checks if a shell is available in the system
func (sm *ShellManager) isShellAvailable(shellName string) bool {
	_, err := exec.LookPath(shellName)
	return err == nil
}

// installShellApp installs shell using DevEx app configuration
func (sm *ShellManager) installShellApp(ctx context.Context, app types.CrossPlatformApp) error {
	// This would integrate with the existing installer system
	// For now, we'll use a simplified approach
	log.Info("Installing shell via DevEx configuration", "app", app.Name)
	// TODO: Integrate with installers.InstallCrossPlatformApp
	return nil
}

// installShellViaSystem installs shell using system package manager
func (sm *ShellManager) installShellViaSystem(ctx context.Context, shellName string) error {
	log.Info("Installing shell via system package manager", "shell", shellName)

	installCmd := fmt.Sprintf("sudo apt-get update && sudo apt-get install -y %s", shellName)
	cmd := exec.CommandContext(ctx, "bash", "-c", installCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to install shell via apt", err, "shell", shellName, "output", string(output))
		return fmt.Errorf("failed to install %s: %w", shellName, err)
	}

	return nil
}

// backupExistingConfig creates backup of existing shell configuration
func (sm *ShellManager) backupExistingConfig(shellName string) error {
	configFile := sm.getShellConfigFile(shellName)
	backupFile := configFile + ".devex-backup"

	if _, err := os.Stat(configFile); err == nil {
		log.Info("Backing up existing shell configuration", "config", configFile, "backup", backupFile)
		return sm.copyFile(configFile, backupFile)
	}

	log.Info("No existing shell configuration to backup", "config", configFile)
	return nil
}

// deployShellConfiguration deploys DevEx shell configuration
func (sm *ShellManager) deployShellConfiguration(shellName string) error {
	log.Info("Deploying DevEx shell configuration", "shell", shellName)

	switch shellName {
	case "bash":
		return sm.deployBashConfiguration()
	case "zsh":
		return sm.deployZshConfiguration()
	case "fish":
		return sm.deployFishConfiguration()
	default:
		return fmt.Errorf("unsupported shell: %s", shellName)
	}
}

// deployBashConfiguration deploys bash-specific configuration
func (sm *ShellManager) deployBashConfiguration() error {
	// 1. Deploy main bashrc
	srcBashrc := filepath.Join(sm.assetsDir, "bash", "bashrc")
	dstBashrc := filepath.Join(sm.homeDir, ".bashrc")

	if err := sm.copyFile(srcBashrc, dstBashrc); err != nil {
		return fmt.Errorf("failed to deploy .bashrc: %w", err)
	}

	// 2. Create bash config directory
	bashConfigDir := filepath.Join(sm.homeDir, ".local", "share", "devex", "defaults", "bash")
	if err := os.MkdirAll(bashConfigDir, 0750); err != nil {
		return fmt.Errorf("failed to create bash config directory: %w", err)
	}

	// 3. Deploy bash modules
	bashFiles := []string{"aliases", "extra", "init", "oh-my-bash", "prompt", "rc", "shell"}
	for _, file := range bashFiles {
		src := filepath.Join(sm.assetsDir, "bash", "bash", file)
		dst := filepath.Join(bashConfigDir, file)
		if err := sm.copyFile(src, dst); err != nil {
			log.Warn("Failed to deploy bash module", "file", file, "error", err)
		}
	}

	// 4. Deploy inputrc and bash_profile
	inputrcSrc := filepath.Join(sm.assetsDir, "bash", "inputrc")
	inputrcDst := filepath.Join(sm.homeDir, ".inputrc")
	_ = sm.copyFile(inputrcSrc, inputrcDst) // Best effort - inputrc is optional

	bashProfileSrc := filepath.Join(sm.assetsDir, "bash", "bash_profile")
	bashProfileDst := filepath.Join(sm.homeDir, ".bash_profile")
	_ = sm.copyFile(bashProfileSrc, bashProfileDst) // Best effort - bash_profile is optional

	return nil
}

// deployZshConfiguration deploys zsh-specific configuration
func (sm *ShellManager) deployZshConfiguration() error {
	// 1. Deploy main zshrc
	srcZshrc := filepath.Join(sm.assetsDir, "zsh", "zshrc")
	dstZshrc := filepath.Join(sm.homeDir, ".zshrc")

	if err := sm.copyFile(srcZshrc, dstZshrc); err != nil {
		return fmt.Errorf("failed to deploy .zshrc: %w", err)
	}

	// 2. Create zsh config directory
	zshConfigDir := filepath.Join(sm.homeDir, ".local", "share", "devex", "defaults", "zsh")
	if err := os.MkdirAll(zshConfigDir, 0750); err != nil {
		return fmt.Errorf("failed to create zsh config directory: %w", err)
	}

	// 3. Deploy zsh modules
	zshFiles := []string{"aliases", "extra", "init", "oh-my-zsh", "prompt", "rc", "shell", "zplug"}
	for _, file := range zshFiles {
		src := filepath.Join(sm.assetsDir, "zsh", "zsh", file)
		dst := filepath.Join(zshConfigDir, file)
		if err := sm.copyFile(src, dst); err != nil {
			log.Warn("Failed to deploy zsh module", "file", file, "error", err)
		}
	}

	// 4. Deploy inputrc
	inputrcSrc := filepath.Join(sm.assetsDir, "zsh", "inputrc")
	inputrcDst := filepath.Join(sm.homeDir, ".inputrc")
	_ = sm.copyFile(inputrcSrc, inputrcDst) // Best effort - inputrc is optional

	return nil
}

// deployFishConfiguration deploys fish-specific configuration
func (sm *ShellManager) deployFishConfiguration() error {
	// 1. Create fish config directory
	fishConfigDir := filepath.Join(sm.homeDir, ".config", "fish")
	if err := os.MkdirAll(fishConfigDir, 0750); err != nil {
		return fmt.Errorf("failed to create fish config directory: %w", err)
	}

	// 2. Deploy main config.fish
	srcConfig := filepath.Join(sm.assetsDir, "fish", "shell")
	dstConfig := filepath.Join(fishConfigDir, "config.fish")

	if err := sm.copyFile(srcConfig, dstConfig); err != nil {
		return fmt.Errorf("failed to deploy config.fish: %w", err)
	}

	// 3. Create fish defaults directory
	fishDefaultsDir := filepath.Join(sm.homeDir, ".local", "share", "devex", "defaults", "fish")
	if err := os.MkdirAll(fishDefaultsDir, 0750); err != nil {
		return fmt.Errorf("failed to create fish defaults directory: %w", err)
	}

	// 4. Deploy fish modules
	fishFiles := []string{"aliases", "shell", "init", "prompt", "extra", "oh-my-fish"}
	for _, file := range fishFiles {
		src := filepath.Join(sm.assetsDir, "fish", file)
		dst := filepath.Join(fishDefaultsDir, file)
		if err := sm.copyFile(src, dst); err != nil {
			log.Warn("Failed to deploy fish module", "file", file, "error", err)
		}
	}

	return nil
}

// switchToShell changes the user's default shell
func (sm *ShellManager) switchToShell(ctx context.Context, shellName string) error {
	shellPath, err := exec.LookPath(shellName)
	if err != nil {
		return fmt.Errorf("%s not found: %w", shellName, err)
	}

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		// Fallback to whoami
		whoamiCmd := exec.CommandContext(ctx, "whoami")
		output, err := whoamiCmd.Output()
		if err != nil {
			return fmt.Errorf("unable to determine current user: %w", err)
		}
		currentUser = strings.TrimSpace(string(output))
	}

	// Check if current shell matches desired shell
	currentShell, err := utils.GetUserShell(currentUser)
	if err != nil {
		log.Warn("Could not detect current shell", "error", err, "user", currentUser)
	} else {
		currentShellName := filepath.Base(currentShell)
		selectedShellName := filepath.Base(shellPath)

		if currentShellName == selectedShellName {
			log.Info("User is already using the selected shell", "shell", shellName, "user", currentUser)
			return nil
		}
	}

	log.Info("Switching to shell", "shell", shellName, "path", shellPath, "user", currentUser)

	// Change user shell
	chshCmd := exec.CommandContext(ctx, "sudo", "chsh", "-s", shellPath, currentUser)
	if err := chshCmd.Run(); err != nil {
		return fmt.Errorf("failed to change shell for user %s: %w", currentUser, err)
	}

	log.Info("Successfully switched shell", "shell", shellName, "user", currentUser)
	return nil
}

// addShellActivationHint adds instructions for activating the new shell
func (sm *ShellManager) addShellActivationHint(shellName string) {
	configFile := sm.getShellConfigFile(shellName)
	log.Info("Shell configuration deployed", "shell", shellName, "config", configFile)
	log.Info("To activate the new shell configuration, run:", "command", fmt.Sprintf("source %s", configFile))
}

// getShellConfigFile returns the main configuration file for a shell
func (sm *ShellManager) getShellConfigFile(shellName string) string {
	switch shellName {
	case "bash":
		return filepath.Join(sm.homeDir, ".bashrc")
	case "zsh":
		return filepath.Join(sm.homeDir, ".zshrc")
	case "fish":
		return filepath.Join(sm.homeDir, ".config", "fish", "config.fish")
	default:
		return ""
	}
}

// copyFile copies a file from source to destination
func (sm *ShellManager) copyFile(src, dst string) error {
	// Validate source exists
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source file not accessible: %w", err)
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer destFile.Close()

	_, err = sourceFile.WriteTo(destFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	log.Info("File copied successfully", "src", src, "dst", dst)
	return nil
}

// detectAssetsDir detects the location of built-in assets
func detectAssetsDir() string {
	possiblePaths := []string{
		"assets",                  // Development mode (relative to binary)
		"./assets",                // Current directory
		"/usr/share/devex/assets", // System install
		"/opt/devex/assets",       // Alternative system install
		"/home/testuser/.local/share/devex/assets", // Docker container
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default fallback
	return "assets"
}
