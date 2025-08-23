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
	cmd := exec.CommandContext(ctx, "sudo", "apt-get", "install", "-y", app.Name)
	return cmd.Run()
}

// installShellViaSystem installs shell via system package manager
func (sm *ShellManager) installShellViaSystem(ctx context.Context, shellName string) error {
	// Detect package manager and install
	// This is simplified - would integrate with package manager detection
	// Try different package managers
	if _, err := exec.LookPath("apt-get"); err == nil {
		cmd := exec.CommandContext(ctx, "sudo", "apt-get", "install", "-y", shellName)
		return cmd.Run()
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		cmd := exec.CommandContext(ctx, "sudo", "dnf", "install", "-y", shellName)
		return cmd.Run()
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		cmd := exec.CommandContext(ctx, "sudo", "pacman", "-S", "--noconfirm", shellName)
		return cmd.Run()
	}
	return fmt.Errorf("unable to detect package manager for shell installation")
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

	// 1. Copy main shell configuration file
	if err := sm.CopyShellConfig(shellType, true); err != nil {
		return fmt.Errorf("failed to copy main %s config: %w", shellName, err)
	}

	// 2. Deploy shell module files to defaults directory
	if err := sm.DeployShellModules(shellName); err != nil {
		return fmt.Errorf("failed to deploy %s modules: %w", shellName, err)
	}

	return nil
}

// DeployShellModules deploys shell module files to the defaults directory
func (sm *ShellManager) DeployShellModules(shellName string) error {
	// Create shell defaults directory
	shellDefaultsDir := filepath.Join(sm.homeDir, ".local", "share", "devex", "defaults", shellName)
	if err := os.MkdirAll(shellDefaultsDir, 0750); err != nil {
		return fmt.Errorf("failed to create %s defaults directory: %w", shellName, err)
	}

	// Define shell module files to copy
	var moduleFiles []string
	var sourceSubDir string

	switch shellName {
	case "bash":
		sourceSubDir = "bash"
		moduleFiles = []string{"aliases", "extra", "init", "oh-my-bash", "prompt", "rc", "shell"}
	case "zsh":
		sourceSubDir = "zsh"
		moduleFiles = []string{"aliases", "extra", "init", "oh-my-zsh", "prompt", "rc", "shell", "zplug"}
	case "fish":
		sourceSubDir = "fish"
		moduleFiles = []string{"aliases", "extra", "init", "oh-my-fish", "prompt", "shell"}
	default:
		return fmt.Errorf("unsupported shell for module deployment: %s", shellName)
	}

	// Copy each module file with fallback path logic
	for _, file := range moduleFiles {
		src := filepath.Join(sm.assetsDir, sourceSubDir, sourceSubDir, file) // e.g., assets/bash/bash/rc
		dst := filepath.Join(shellDefaultsDir, file)

		// If the double subdirectory doesn't exist, try single subdirectory
		if _, err := os.Stat(src); os.IsNotExist(err) {
			src = filepath.Join(sm.assetsDir, sourceSubDir, file) // e.g., assets/bash/rc
		}

		if err := sm.copyFileWithPermissions(src, dst, 0644); err != nil {
			log.Warn("Failed to deploy shell module", "shell", shellName, "file", file, "error", err)
			continue // Don't fail the entire deployment for missing optional files
		}

		log.Debug("Deployed shell module", "shell", shellName, "file", file)
	}

	// Special handling for inputrc and bash_profile (bash only)
	if shellName == "bash" {
		// Deploy inputrc
		inputrcSrc := filepath.Join(sm.assetsDir, "bash", "inputrc")
		inputrcDst := filepath.Join(sm.homeDir, ".inputrc")
		if _, err := os.Stat(inputrcSrc); err == nil {
			_ = sm.copyFileWithPermissions(inputrcSrc, inputrcDst, 0644) // Best effort
		}

		// Deploy bash_profile
		bashProfileSrc := filepath.Join(sm.assetsDir, "bash", "bash_profile")
		bashProfileDst := filepath.Join(sm.homeDir, ".bash_profile")
		if _, err := os.Stat(bashProfileSrc); err == nil {
			_ = sm.copyFileWithPermissions(bashProfileSrc, bashProfileDst, 0644) // Best effort
		}
	}

	log.Info("Shell modules deployed successfully", "shell", shellName, "destination", shellDefaultsDir)
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
		return fmt.Errorf("unable to determine current user")
	}

	// Check if already using this shell
	currentShell := os.Getenv("SHELL")
	if strings.HasSuffix(currentShell, "/"+shellName) {
		log.Info("Already using shell", "shell", shellName)
		return nil
	}

	log.Info("Switching default shell", "shell", shellName, "path", shellPath)

	// Use chsh to change shell
	cmd := exec.CommandContext(ctx, "sudo", "chsh", "-s", shellPath, currentUser)
	if err := cmd.Run(); err != nil {
		// Try without sudo
		cmd = exec.CommandContext(ctx, "chsh", "-s", shellPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to change shell: %w", err)
		}
	}

	log.Info("Shell changed successfully", "shell", shellName)
	return nil
}

// addShellActivationHint provides user guidance for shell activation
func (sm *ShellManager) addShellActivationHint(shellName string) {
	log.Info("Shell setup complete", "shell", shellName)
	log.Info("To activate your new shell environment:", "hint", fmt.Sprintf("exec %s", shellName))
	log.Info("Or restart your terminal for changes to take effect")
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

// BackupExistingConfig creates a backup of existing shell config
func (sm *ShellManager) BackupExistingConfig(configPath string) error {
	if sm.backupMgr == nil {
		return fmt.Errorf("backup manager not initialized")
	}

	// Create backup using the backup manager
	options := backup.BackupOptions{
		Description: fmt.Sprintf("Shell config backup for %s", filepath.Base(configPath)),
		Type:        "shell-config",
	}
	if _, err := sm.backupMgr.CreateBackup(options); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	log.Info("Created backup of existing config", "config", configPath)
	return nil
}

// CopyShellConfig copies a shell config from assets to home directory with proper naming
func (sm *ShellManager) CopyShellConfig(shell ShellType, overwrite bool) error {
	configs := sm.GetShellConfigs()
	config, exists := configs[shell]
	if !exists {
		return fmt.Errorf("unsupported shell type: %s", shell)
	}

	// Check if source asset exists
	if _, err := os.Stat(config.AssetPath); err != nil {
		return fmt.Errorf("shell config asset not found: %w", err)
	}

	// Determine destination path
	destPath := filepath.Join(sm.homeDir, config.HomeConfigFile)

	// Check if destination exists
	if _, err := os.Stat(destPath); err == nil && !overwrite {
		return fmt.Errorf("destination file exists (use overwrite to replace)")
	}

	// Backup existing config if it exists
	if _, err := os.Stat(destPath); err == nil {
		if err := sm.BackupExistingConfig(destPath); err != nil {
			log.Warn("Failed to backup existing config", "error", err)
			// Continue anyway
		}
	}

	// Ensure destination directory exists (important for fish)
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy the config file with proper permissions
	if err := sm.copyFileWithPermissions(config.AssetPath, destPath, config.Permissions); err != nil {
		return fmt.Errorf("failed to copy shell config: %w", err)
	}

	log.Info("Shell config copied successfully", "shell", shell, "destination", destPath)
	return nil
}

// copyFileWithPermissions copies a file with specific permissions
func (sm *ShellManager) copyFileWithPermissions(src, dst string, permissions os.FileMode) error {
	// Validate source exists
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source file not accessible: %w", err)
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Set permissions
	if err := os.Chmod(dst, permissions); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// AppendToShellConfig appends content to an existing shell config file
func (sm *ShellManager) AppendToShellConfig(shell ShellType, content string) error {
	configs := sm.GetShellConfigs()
	config, exists := configs[shell]
	if !exists {
		return fmt.Errorf("unsupported shell type: %s", shell)
	}

	destPath := filepath.Join(sm.homeDir, config.HomeConfigFile)

	// Ensure file exists
	if _, err := os.Stat(destPath); err != nil {
		return fmt.Errorf("shell config file does not exist: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open config file for appending: %w", err)
	}
	defer file.Close()

	// Write content
	if _, err := file.WriteString("\n" + content + "\n"); err != nil {
		return fmt.Errorf("failed to append to config file: %w", err)
	}

	log.Info("Content appended to shell config", "shell", shell)
	return nil
}

// GetConfigPath returns the home path for a shell configuration
func (sm *ShellManager) GetConfigPath(shell ShellType) (string, error) {
	configs := sm.GetShellConfigs()
	config, exists := configs[shell]
	if !exists {
		return "", fmt.Errorf("unsupported shell type: %s", shell)
	}
	return filepath.Join(sm.homeDir, config.HomeConfigFile), nil
}

// IsConfigInstalled checks if a shell configuration is installed
func (sm *ShellManager) IsConfigInstalled(shell ShellType) (bool, error) {
	configPath, err := sm.GetConfigPath(shell)
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// ListAvailableConfigs returns list of shell configs available in assets
func (sm *ShellManager) ListAvailableConfigs() ([]ShellType, error) {
	var available []ShellType
	configs := sm.GetShellConfigs()

	for shellType, config := range configs {
		if _, err := os.Stat(config.AssetPath); err == nil {
			available = append(available, shellType)
		}
	}

	return available, nil
}

// DetectUserShell detects the current user's shell
func DetectUserShell() ShellType {
	// Check SHELL environment variable
	shellEnv := os.Getenv("SHELL")
	if shellEnv != "" {
		if strings.Contains(shellEnv, "bash") {
			return Bash
		}
		if strings.Contains(shellEnv, "zsh") {
			return Zsh
		}
		if strings.Contains(shellEnv, "fish") {
			return Fish
		}
	}

	// Default to bash
	return Bash
}

// HasMarker checks if a shell config file has a specific marker
func (sm *ShellManager) HasMarker(shell ShellType, marker string) (bool, error) {
	configPath, err := sm.GetConfigPath(shell)
	if err != nil {
		return false, err
	}

	file, err := os.Open(configPath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	markerComment := fmt.Sprintf("# %s", marker)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), markerComment) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// AppendWithMarker appends content with a marker to prevent duplicates
func (sm *ShellManager) AppendWithMarker(shell ShellType, marker, content string) error {
	// Check if marker already exists
	exists, err := sm.HasMarker(shell, marker)
	if err != nil {
		return err
	}

	if exists {
		log.Info("Marker already exists in config, skipping", "shell", shell, "marker", marker)
		return nil
	}

	// Add marker and content
	markerComment := fmt.Sprintf("# %s", marker)
	fullContent := fmt.Sprintf("%s\n%s", markerComment, content)

	return sm.AppendToShellConfig(shell, fullContent)
}
