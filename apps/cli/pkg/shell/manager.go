package shell

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/pkg/backup"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// ShellType represents different shell types
type ShellType string

const (
	Bash ShellType = "bash"
	Zsh  ShellType = "zsh"
	Fish ShellType = "fish"
)

// ShellConfig represents a shell configuration mapping
type ShellConfig struct {
	Shell          ShellType
	ConfigFile     string // e.g., "bashrc", "zshrc", "config.fish"
	HomeConfigFile string // e.g., ".bashrc", ".zshrc", ".config/fish/config.fish"
	AssetPath      string // Path in assets directory
	Permissions    os.FileMode
}

// ShellManager handles comprehensive shell setup and management
type ShellManager struct {
	homeDir    string
	assetsDir  string
	configDir  string
	settings   config.CrossPlatformSettings
	repository types.Repository
	backupMgr  *backup.BackupManager
}

// NewShellManager creates a new shell manager instance
func NewShellManager(settings config.CrossPlatformSettings, repository types.Repository) *ShellManager {
	homeDir := os.Getenv("HOME")
	assetsDir := detectAssetsDir()
	configDir := settings.GetConfigDir()

	// Backup manager expects the parent directory since it adds "/config" internally
	backupBaseDir := filepath.Dir(configDir)

	return &ShellManager{
		homeDir:    homeDir,
		assetsDir:  assetsDir,
		configDir:  configDir,
		settings:   settings,
		repository: repository,
		backupMgr:  backup.NewBackupManager(backupBaseDir),
	}
}

// NewShellManagerSimple creates a simple shell manager for basic operations
func NewShellManagerSimple(homeDir, assetsDir, configDir string) *ShellManager {
	// Backup manager expects the parent directory since it adds "/config" internally
	backupBaseDir := filepath.Dir(configDir)

	return &ShellManager{
		homeDir:   homeDir,
		assetsDir: assetsDir,
		configDir: configDir,
		backupMgr: backup.NewBackupManager(backupBaseDir),
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
		return sm.copyFileWithPermissions(configFile, backupFile, 0644)
	}

	log.Info("No existing shell configuration to backup", "config", configFile)
	return nil
}

// deployShellConfiguration deploys DevEx shell configuration
func (sm *ShellManager) deployShellConfiguration(shellName string) error {
	log.Info("Deploying DevEx shell configuration", "shell", shellName)

	// Convert shell name to ShellType and use our proven working shell configuration system
	var shellType ShellType
	switch shellName {
	case "bash":
		shellType = Bash
	case "zsh":
		shellType = Zsh
	case "fish":
		shellType = Fish
	default:
		return fmt.Errorf("unsupported shell: %s", shellName)
	}

	// Use CopyShellConfig which handles backup, copying, and permissions
	return sm.CopyShellConfig(shellType, true) // overwrite = true for setup
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

// detectAssetsDir detects the location of built-in assets
func detectAssetsDir() string {
	// Get current working directory for development
	cwd, _ := os.Getwd()

	possiblePaths := []string{
		"assets",                     // Development mode (relative to binary)
		"./assets",                   // Current directory
		filepath.Join(cwd, "assets"), // Explicit current working directory
		"/usr/share/devex/assets",    // System install
		"/opt/devex/assets",          // Alternative system install
		"/home/testuser/.local/share/devex/assets", // Docker container
		"../assets",             // One directory up
		"../../apps/cli/assets", // From root of project
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			absPath, _ := filepath.Abs(path)
			return absPath
		}
	}

	// Default fallback
	return "assets"
}

// GetShellConfigs returns configuration mapping for all supported shells
func (sm *ShellManager) GetShellConfigs() map[ShellType]ShellConfig {
	return map[ShellType]ShellConfig{
		Bash: {
			Shell:          Bash,
			ConfigFile:     "bashrc",
			HomeConfigFile: ".bashrc",
			AssetPath:      filepath.Join(sm.assetsDir, "bash", "bashrc"),
			Permissions:    0644,
		},
		Zsh: {
			Shell:          Zsh,
			ConfigFile:     "zshrc",
			HomeConfigFile: ".zshrc",
			AssetPath:      filepath.Join(sm.assetsDir, "zsh", "zshrc"),
			Permissions:    0644,
		},
		Fish: {
			Shell:          Fish,
			ConfigFile:     "config.fish",
			HomeConfigFile: ".config/fish/config.fish",
			AssetPath:      filepath.Join(sm.assetsDir, "fish", "config.fish"),
			Permissions:    0644,
		},
	}
}

// BackupExistingConfig creates a backup of an existing config file using backup manager
func (sm *ShellManager) BackupExistingConfig(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // No file to backup
	}

	// Create backup before modifying
	_, err := sm.backupMgr.CreateBackup(backup.BackupOptions{
		Description: fmt.Sprintf("Backup before modifying %s", filepath.Base(configPath)),
		Type:        "pre-config",
		Tags:        []string{"shell", "auto"},
		Compress:    false,
		Include:     []string{configPath},
	})

	return err
}

// CopyShellConfig copies a shell config from assets to home directory with proper naming
func (sm *ShellManager) CopyShellConfig(shell ShellType, overwrite bool) error {
	configs := sm.GetShellConfigs()
	config, exists := configs[shell]
	if !exists {
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	// Check if source asset exists
	if _, err := os.Stat(config.AssetPath); os.IsNotExist(err) {
		return fmt.Errorf("shell config asset not found: %s", config.AssetPath)
	}

	// Determine destination path
	destPath := filepath.Join(sm.homeDir, config.HomeConfigFile)

	// Create backup if file exists
	if err := sm.BackupExistingConfig(destPath); err != nil {
		return fmt.Errorf("failed to backup existing config: %w", err)
	}

	// Check if destination exists and overwrite flag
	if _, err := os.Stat(destPath); err == nil && !overwrite {
		return fmt.Errorf("config file already exists: %s (use --overwrite to replace)", destPath)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// Copy the file
	if err := sm.copyFileWithPermissions(config.AssetPath, destPath, config.Permissions); err != nil {
		return fmt.Errorf("failed to copy %s to %s: %w", config.AssetPath, destPath, err)
	}

	return nil
}

// copyFileWithPermissions copies a file with specific permissions
func (sm *ShellManager) copyFileWithPermissions(src, dst string, permissions os.FileMode) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	destDir := filepath.Dir(dst)
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	return os.Chmod(dst, permissions)
}

// AppendToShellConfig appends content to an existing shell config file
func (sm *ShellManager) AppendToShellConfig(shell ShellType, content string) error {
	configs := sm.GetShellConfigs()
	config, exists := configs[shell]
	if !exists {
		return fmt.Errorf("unsupported shell: %s", shell)
	}

	configPath := filepath.Join(sm.homeDir, config.HomeConfigFile)

	// Create backup before modifying
	if err := sm.BackupExistingConfig(configPath); err != nil {
		return fmt.Errorf("failed to backup config before append: %w", err)
	}

	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Open file for appending, create if it doesn't exist
	file, err := os.OpenFile(configPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, config.Permissions)
	if err != nil {
		return fmt.Errorf("failed to open config file for appending: %w", err)
	}
	defer file.Close()

	// Add newline before content if file is not empty
	if stat, err := file.Stat(); err == nil && stat.Size() > 0 {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write newline: %w", err)
		}
	}

	// Append the content
	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to append content: %w", err)
	}

	// Ensure content ends with newline
	if !strings.HasSuffix(content, "\n") {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("failed to write final newline: %w", err)
		}
	}

	return nil
}

// GetConfigPath returns the full path to a shell's config file in the home directory
func (sm *ShellManager) GetConfigPath(shell ShellType) (string, error) {
	configs := sm.GetShellConfigs()
	config, exists := configs[shell]
	if !exists {
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}

	return filepath.Join(sm.homeDir, config.HomeConfigFile), nil
}

// IsConfigInstalled checks if a shell config file exists in the home directory
func (sm *ShellManager) IsConfigInstalled(shell ShellType) (bool, error) {
	configPath, err := sm.GetConfigPath(shell)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// ListAvailableConfigs returns list of available shell configs from assets
func (sm *ShellManager) ListAvailableConfigs() ([]ShellType, error) {
	var available []ShellType
	configs := sm.GetShellConfigs()

	for shell, config := range configs {
		if _, err := os.Stat(config.AssetPath); err == nil {
			available = append(available, shell)
		}
	}

	return available, nil
}

// DetectUserShell attempts to detect the user's current shell
func DetectUserShell() ShellType {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return Bash // Default fallback
	}

	shell = filepath.Base(shell)
	switch shell {
	case "bash":
		return Bash
	case "zsh":
		return Zsh
	case "fish":
		return Fish
	default:
		return Bash // Default fallback
	}
}

// HasMarker checks if a config file contains a specific marker comment
func (sm *ShellManager) HasMarker(shell ShellType, marker string) (bool, error) {
	configPath, err := sm.GetConfigPath(shell)
	if err != nil {
		return false, err
	}

	file, err := os.Open(configPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, marker) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// AppendWithMarker appends content to a shell config only if the marker doesn't already exist
func (sm *ShellManager) AppendWithMarker(shell ShellType, marker, content string) error {
	// Check if marker already exists
	hasMarker, err := sm.HasMarker(shell, marker)
	if err != nil {
		return err
	}

	if hasMarker {
		return nil // Already exists, nothing to do
	}

	// Add marker and content
	markerComment := fmt.Sprintf("# %s", marker)
	fullContent := fmt.Sprintf("%s\n%s", markerComment, content)

	return sm.AppendToShellConfig(shell, fullContent)
}
