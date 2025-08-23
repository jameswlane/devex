package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/shell"
	"github.com/spf13/cobra"
)

// NewSystemCmd creates the system command with shell configuration support
func NewSystemCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage system settings and configurations",
		Long: `Configure and optimize your system settings for development.

The system command manages system-level configurations including:
  • Git global configuration (aliases, user settings, SSH keys)
  • Shell configuration (Bash/Zsh/Fish profiles and dotfiles)
  • Desktop environment settings (GNOME, KDE themes and preferences)
  • Terminal configuration and color schemes
  • Font installation and management

This command provides comprehensive shell configuration management with
automatic backup, dotfile conversion, and append functionality.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add shell configuration subcommands
	cmd.AddCommand(newShellCmd(settings))

	return cmd
}

// newShellCmd creates the shell configuration subcommand
func newShellCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Manage shell configurations",
		Long: `Manage shell configuration files (Bash, Zsh, Fish).

Features:
  • Copy shell configs from assets to home directory with proper dotfile names
  • Append content to existing shell configs
  • Automatic backup before modifications
  • Support for Bash (.bashrc), Zsh (.zshrc), and Fish (.config/fish/config.fish)
  • Proper file permissions and directory creation`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add shell subcommands
	cmd.AddCommand(newShellCopyCmd(settings))
	cmd.AddCommand(newShellAppendCmd(settings))
	cmd.AddCommand(newShellStatusCmd(settings))
	cmd.AddCommand(newShellListCmd(settings))
	cmd.AddCommand(newShellDebugCmd(settings))
	cmd.AddCommand(newShellTestCopyCmd(settings))

	return cmd
}

// newShellCopyCmd creates the shell copy subcommand
func newShellCopyCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		overwrite bool
		shellType string
	)

	cmd := &cobra.Command{
		Use:   "copy [shell]",
		Short: "Copy shell configurations from assets to home directory",
		Long: `Copy shell configuration files from assets to home directory with proper dotfile names.

This command:
  • Creates automatic backups of existing configs
  • Converts asset filenames to proper dotfile names
  • Creates necessary directories (e.g., .config/fish/)
  • Sets correct file permissions
  • Supports bash, zsh, and fish shells`,
		Example: `  # Copy bash config to ~/.bashrc
  devex system shell copy bash

  # Copy zsh config to ~/.zshrc with overwrite
  devex system shell copy zsh --overwrite

  # Copy all available shell configs
  devex system shell copy --shell all`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellCopy(settings, args, shellType, overwrite)
		},
	}

	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing configuration files")
	cmd.Flags().StringVar(&shellType, "shell", "", "Specify shell type (bash, zsh, fish, all)")

	return cmd
}

// newShellAppendCmd creates the shell append subcommand
func newShellAppendCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		shellType string
		content   string
		marker    string
		file      string
	)

	cmd := &cobra.Command{
		Use:   "append",
		Short: "Append content to shell configuration files",
		Long: `Append content to existing shell configuration files.

This command:
  • Creates automatic backups before modifications
  • Creates config files if they don't exist
  • Supports marker-based idempotent appends
  • Handles proper newline formatting
  • Works with bash, zsh, and fish shells`,
		Example: `  # Append environment variable to bash
  devex system shell append --shell bash --content "export EDITOR=nvim"

  # Append with marker to prevent duplicates
  devex system shell append --shell zsh --content "source ~/.zsh_custom" --marker "Custom ZSH Config"

  # Append content from file
  devex system shell append --shell fish --file ~/.my_fish_config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellAppend(settings, shellType, content, marker, file)
		},
	}

	cmd.Flags().StringVar(&shellType, "shell", "", "Target shell (bash, zsh, fish)")
	cmd.Flags().StringVar(&content, "content", "", "Content to append")
	cmd.Flags().StringVar(&marker, "marker", "", "Marker comment to prevent duplicate appends")
	cmd.Flags().StringVar(&file, "file", "", "File containing content to append")

	_ = cmd.MarkFlagRequired("shell")

	return cmd
}

// newShellStatusCmd creates the shell status subcommand
func newShellStatusCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show shell configuration status",
		Long: `Display the current status of shell configurations.

Shows:
  • Which shell configs are installed
  • Available shell configs in assets
  • Current user's shell
  • Config file locations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellStatus(settings)
		},
	}

	return cmd
}

// newShellListCmd creates the shell list subcommand
func newShellListCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available shell configurations",
		Long: `List all available shell configurations from assets directory.

Shows which shell configurations are available for installation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellList(settings)
		},
	}

	return cmd
}

// runShellCopy executes the shell copy command
func runShellCopy(settings config.CrossPlatformSettings, args []string, shellType string, overwrite bool) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := settings.GetConfigDir()
	assetsDir := filepath.Join(configDir, "..", "assets")

	manager := shell.NewShellManagerSimple(homeDir, assetsDir, configDir)

	// Determine which shells to copy
	var shellsToCopy []shell.ShellType

	var targetShell string
	if len(args) > 0 {
		targetShell = args[0]
	} else if shellType != "" {
		targetShell = shellType
	}

	switch targetShell {
	case "bash":
		shellsToCopy = []shell.ShellType{shell.Bash}
	case "zsh":
		shellsToCopy = []shell.ShellType{shell.Zsh}
	case "fish":
		shellsToCopy = []shell.ShellType{shell.Fish}
	case "all":
		shellsToCopy = []shell.ShellType{shell.Bash, shell.Zsh, shell.Fish}
	case "":
		// Default to current user's shell
		detected := shell.DetectUserShell()
		shellsToCopy = []shell.ShellType{detected}
		fmt.Printf("Auto-detected shell: %s\n", detected)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish, all)", targetShell)
	}

	// Copy each shell configuration
	for _, shellType := range shellsToCopy {
		fmt.Printf("Copying %s configuration...\n", shellType)
		if err := manager.CopyShellConfig(shellType, overwrite); err != nil {
			fmt.Printf("Warning: Failed to copy %s config: %v\n", shellType, err)
			continue
		}

		configPath, _ := manager.GetConfigPath(shellType)
		fmt.Printf("✓ Copied %s configuration to %s\n", shellType, configPath)
	}

	return nil
}

// runShellAppend executes the shell append command
func runShellAppend(settings config.CrossPlatformSettings, shellType, content, marker, file string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := settings.GetConfigDir()
	assetsDir := filepath.Join(configDir, "..", "assets")

	manager := shell.NewShellManagerSimple(homeDir, assetsDir, configDir)

	// Parse shell type
	var targetShell shell.ShellType
	switch shellType {
	case "bash":
		targetShell = shell.Bash
	case "zsh":
		targetShell = shell.Zsh
	case "fish":
		targetShell = shell.Fish
	default:
		return fmt.Errorf("unsupported shell: %s", shellType)
	}

	// Get content from file if specified
	if file != "" {
		if content != "" {
			return fmt.Errorf("cannot specify both --content and --file")
		}

		contentBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", file, err)
		}
		content = string(contentBytes)
	}

	if content == "" {
		return fmt.Errorf("no content specified (use --content or --file)")
	}

	// Append content
	if marker != "" {
		err = manager.AppendWithMarker(targetShell, marker, content)
	} else {
		err = manager.AppendToShellConfig(targetShell, content)
	}

	if err != nil {
		return fmt.Errorf("failed to append to %s config: %w", targetShell, err)
	}

	configPath, _ := manager.GetConfigPath(targetShell)
	fmt.Printf("✓ Appended content to %s configuration: %s\n", targetShell, configPath)

	return nil
}

// runShellStatus executes the shell status command
func runShellStatus(settings config.CrossPlatformSettings) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := settings.GetConfigDir()
	assetsDir := filepath.Join(configDir, "..", "assets")

	manager := shell.NewShellManagerSimple(homeDir, assetsDir, configDir)

	// Show current user shell
	currentShell := shell.DetectUserShell()
	fmt.Printf("Current shell: %s\n\n", currentShell)

	// Show status for each shell
	shells := []shell.ShellType{shell.Bash, shell.Zsh, shell.Fish}
	fmt.Println("Shell Configuration Status:")
	fmt.Println("===========================")

	for _, shellType := range shells {
		fmt.Printf("\n%s:\n", shellType)

		// Check if installed
		installed, err := manager.IsConfigInstalled(shellType)
		if err != nil {
			fmt.Printf("  Status: Error - %v\n", err)
			continue
		}

		if installed {
			configPath, _ := manager.GetConfigPath(shellType)
			fmt.Printf("  Status: ✓ Installed\n")
			fmt.Printf("  Location: %s\n", configPath)
		} else {
			fmt.Printf("  Status: ✗ Not installed\n")
		}

		// Check if asset exists
		configs := manager.GetShellConfigs()
		if config, exists := configs[shellType]; exists {
			if _, err := os.Stat(config.AssetPath); err == nil {
				fmt.Printf("  Asset: ✓ Available\n")
				fmt.Printf("  Asset Path: %s\n", config.AssetPath)
			} else {
				fmt.Printf("  Asset: ✗ Not found\n")
			}
		}
	}

	return nil
}

// runShellList executes the shell list command
func runShellList(settings config.CrossPlatformSettings) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := settings.GetConfigDir()
	assetsDir := filepath.Join(configDir, "..", "assets")

	manager := shell.NewShellManagerSimple(homeDir, assetsDir, configDir)

	available, err := manager.ListAvailableConfigs()
	if err != nil {
		return fmt.Errorf("failed to list available configs: %w", err)
	}

	if len(available) == 0 {
		fmt.Println("No shell configurations available in assets directory.")
		return nil
	}

	fmt.Println("Available Shell Configurations:")
	fmt.Println("==============================")

	configs := manager.GetShellConfigs()
	for _, shellType := range available {
		config := configs[shellType]
		fmt.Printf("\n%s:\n", shellType)
		fmt.Printf("  Config File: %s\n", config.ConfigFile)
		fmt.Printf("  Home Location: %s\n", config.HomeConfigFile)
		fmt.Printf("  Asset Path: %s\n", config.AssetPath)
	}

	return nil
}

// newShellDebugCmd creates the shell debug subcommand
func newShellDebugCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug",
		Short: "Debug shell configuration paths and assets",
		Long: `Debug shell configuration system to troubleshoot issues.

This command shows detailed information about:
  • Path resolution and detection
  • Asset file locations and existence
  • Directory permissions and access
  • Configuration mappings and conversions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellDebug(settings)
		},
	}

	return cmd
}

// runShellDebug executes the shell debug command
func runShellDebug(settings config.CrossPlatformSettings) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := settings.GetConfigDir()
	assetsDir := filepath.Join(configDir, "..", "assets")

	manager := shell.NewShellManagerSimple(homeDir, assetsDir, configDir)

	fmt.Println("🔍 Shell Configuration Debug Information")
	fmt.Println("======================================")

	// Show environment
	fmt.Printf("\n📁 Environment:\n")
	fmt.Printf("  Home Directory: %s\n", homeDir)
	fmt.Printf("  Config Directory: %s\n", configDir)
	fmt.Printf("  Assets Directory: %s\n", assetsDir)
	fmt.Printf("  Current User: %s\n", os.Getenv("USER"))
	fmt.Printf("  Current Shell: %s\n", shell.DetectUserShell())

	// Check directory existence and permissions
	fmt.Printf("\n📂 Directory Status:\n")
	dirs := map[string]string{
		"Home":   homeDir,
		"Config": configDir,
		"Assets": assetsDir,
	}

	for name, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			fmt.Printf("  %s: ❌ %v\n", name, err)
		} else {
			fmt.Printf("  %s: ✅ Exists (mode: %s)\n", name, info.Mode())
		}
	}

	// Check shell configurations
	fmt.Printf("\n🐚 Shell Configuration Mappings:\n")
	configs := manager.GetShellConfigs()

	for shellType, config := range configs {
		fmt.Printf("\n  %s:\n", shellType)
		fmt.Printf("    Config File: %s\n", config.ConfigFile)
		fmt.Printf("    Home Config: %s\n", config.HomeConfigFile)
		fmt.Printf("    Full Home Path: %s\n", filepath.Join(homeDir, config.HomeConfigFile))
		fmt.Printf("    Asset Path: %s\n", config.AssetPath)
		fmt.Printf("    Permissions: %o\n", config.Permissions)

		// Check asset file
		if assetInfo, err := os.Stat(config.AssetPath); err != nil {
			fmt.Printf("    Asset Status: ❌ %v\n", err)
		} else {
			fmt.Printf("    Asset Status: ✅ Exists (size: %d bytes, mode: %s)\n",
				assetInfo.Size(), assetInfo.Mode())
		}

		// Check home config file
		homeConfigPath := filepath.Join(homeDir, config.HomeConfigFile)
		if homeInfo, err := os.Stat(homeConfigPath); err != nil {
			fmt.Printf("    Home Config: ❌ %v\n", err)
		} else {
			fmt.Printf("    Home Config: ✅ Exists (size: %d bytes, mode: %s)\n",
				homeInfo.Size(), homeInfo.Mode())
		}
	}

	// Test asset discovery
	fmt.Printf("\n🔍 Asset Discovery:\n")
	available, err := manager.ListAvailableConfigs()
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
	} else {
		if len(available) == 0 {
			fmt.Printf("  No available configurations found\n")
		} else {
			fmt.Printf("  Available: %v\n", available)
		}
	}

	// Show detailed asset directory contents
	fmt.Printf("\n📄 Asset Directory Contents:\n")
	err = filepath.Walk(assetsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("  Error walking %s: %v\n", path, err)
			return nil
		}

		relPath, _ := filepath.Rel(assetsDir, path)
		if info.IsDir() {
			fmt.Printf("  📁 %s/\n", relPath)
		} else {
			fmt.Printf("  📄 %s (size: %d bytes)\n", relPath, info.Size())
		}
		return nil
	})

	if err != nil {
		fmt.Printf("  Failed to walk assets directory: %v\n", err)
	}

	// Test backup system
	fmt.Printf("\n💾 Backup System Test:\n")
	testFile := filepath.Join(homeDir, ".test_backup_file")
	err = os.WriteFile(testFile, []byte("test content"), 0600)
	if err != nil {
		fmt.Printf("  Failed to create test file: %v\n", err)
	} else {
		defer os.Remove(testFile)

		err = manager.BackupExistingConfig(testFile)
		if err != nil {
			fmt.Printf("  Backup test: ❌ %v\n", err)
		} else {
			fmt.Printf("  Backup test: ✅ Success\n")
		}
	}

	return nil
}

// newShellTestCopyCmd creates the shell test-copy subcommand
func newShellTestCopyCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		overwrite bool
		shellType string
	)

	cmd := &cobra.Command{
		Use:   "test-copy [shell]",
		Short: "Test shell configuration copy with verbose debugging",
		Long: `Test copy shell configuration files with detailed debugging output.

This command provides step-by-step verbose output to debug the copy process:
  • Shows all paths being used
  • Checks file existence at each step  
  • Shows actual copy operations
  • Reports any errors with full context`,
		Example: `  # Test copy bash config with debugging
  devex system shell test-copy bash

  # Test copy with overwrite and see what happens
  devex system shell test-copy zsh --overwrite`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShellTestCopy(settings, args, shellType, overwrite)
		},
	}

	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing configuration files")
	cmd.Flags().StringVar(&shellType, "shell", "", "Specify shell type (bash, zsh, fish)")

	return cmd
}

// runShellTestCopy executes the shell test-copy command with verbose debugging
func runShellTestCopy(settings config.CrossPlatformSettings, args []string, shellType string, overwrite bool) error {
	fmt.Println("🧪 Shell Configuration Test Copy")
	fmt.Println("===============================")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configDir := settings.GetConfigDir()
	assetsDir := filepath.Join(configDir, "..", "assets")

	fmt.Printf("📁 Setup:\n")
	fmt.Printf("  Home Directory: %s\n", homeDir)
	fmt.Printf("  Config Directory: %s\n", configDir)
	fmt.Printf("  Assets Directory: %s\n", assetsDir)

	manager := shell.NewShellManagerSimple(homeDir, assetsDir, configDir)

	// Determine target shell
	var targetShellType shell.ShellType
	var targetShell string

	switch {
	case len(args) > 0:
		targetShell = args[0]
	case shellType != "":
		targetShell = shellType
	default:
		detected := shell.DetectUserShell()
		targetShellType = detected
		targetShell = string(detected)
		fmt.Printf("  Auto-detected shell: %s\n", detected)
	}

	// Only set targetShellType if not already set
	if targetShellType == "" {
		switch targetShell {
		case "bash":
			targetShellType = shell.Bash
		case "zsh":
			targetShellType = shell.Zsh
		case "fish":
			targetShellType = shell.Fish
		default:
			return fmt.Errorf("unsupported shell: %s", targetShell)
		}
	}

	fmt.Printf("\n🎯 Target: %s\n", targetShellType)

	// Get shell config
	configs := manager.GetShellConfigs()
	config, exists := configs[targetShellType]
	if !exists {
		return fmt.Errorf("no configuration found for shell: %s", targetShellType)
	}

	fmt.Printf("\n📋 Configuration:\n")
	fmt.Printf("  Config File: %s\n", config.ConfigFile)
	fmt.Printf("  Home Config File: %s\n", config.HomeConfigFile)
	fmt.Printf("  Asset Path: %s\n", config.AssetPath)
	fmt.Printf("  Permissions: %o\n", config.Permissions)

	// Check if source asset exists
	fmt.Printf("\n🔍 Source Asset Check:\n")
	assetInfo, err := os.Stat(config.AssetPath)
	if err != nil {
		fmt.Printf("  ❌ Asset file not found: %v\n", err)
		return fmt.Errorf("asset file not accessible: %w", err)
	}
	fmt.Printf("  ✅ Asset exists: %s (size: %d bytes, mode: %s)\n",
		config.AssetPath, assetInfo.Size(), assetInfo.Mode())

	// Show asset content preview
	fmt.Printf("\n📄 Asset Content Preview (first 200 chars):\n")
	assetContent, err := os.ReadFile(config.AssetPath)
	if err != nil {
		fmt.Printf("  ❌ Could not read asset: %v\n", err)
	} else {
		preview := string(assetContent)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("  Content: %q\n", preview)
	}

	// Check destination
	destPath := filepath.Join(homeDir, config.HomeConfigFile)
	fmt.Printf("\n🎯 Destination Check:\n")
	fmt.Printf("  Target Path: %s\n", destPath)

	destInfo, err := os.Stat(destPath)
	if err == nil {
		fmt.Printf("  ⚠️  File exists: size %d bytes, mode %s\n", destInfo.Size(), destInfo.Mode())
		if !overwrite {
			fmt.Printf("  ❌ File exists and overwrite=false\n")
			return fmt.Errorf("destination file exists (use --overwrite to replace)")
		}
		fmt.Printf("  ✅ Overwrite enabled, will replace existing file\n")
	} else {
		fmt.Printf("  ✅ Destination doesn't exist, safe to create\n")
	}

	// Test backup if file exists
	if destInfo != nil {
		fmt.Printf("\n💾 Backup Test:\n")
		err = manager.BackupExistingConfig(destPath)
		if err != nil {
			fmt.Printf("  ❌ Backup failed: %v\n", err)
		} else {
			fmt.Printf("  ✅ Backup created successfully\n")
		}
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	fmt.Printf("\n📁 Directory Creation:\n")
	fmt.Printf("  Target directory: %s\n", destDir)

	dirInfo, err := os.Stat(destDir)
	if err != nil {
		fmt.Printf("  Creating directory...\n")
		if err := os.MkdirAll(destDir, 0750); err != nil {
			fmt.Printf("  ❌ Failed to create directory: %v\n", err)
			return fmt.Errorf("failed to create directory: %w", err)
		}
		fmt.Printf("  ✅ Directory created\n")
	} else {
		fmt.Printf("  ✅ Directory exists (mode: %s)\n", dirInfo.Mode())
	}

	// Perform the actual copy
	fmt.Printf("\n📋 File Copy Operation:\n")
	fmt.Printf("  Source: %s\n", config.AssetPath)
	fmt.Printf("  Destination: %s\n", destPath)
	fmt.Printf("  Permissions: %o\n", config.Permissions)

	sourceFile, err := os.Open(config.AssetPath)
	if err != nil {
		fmt.Printf("  ❌ Failed to open source: %v\n", err)
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer sourceFile.Close()
	fmt.Printf("  ✅ Source file opened\n")

	destFile, err := os.Create(destPath)
	if err != nil {
		fmt.Printf("  ❌ Failed to create destination: %v\n", err)
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer destFile.Close()
	fmt.Printf("  ✅ Destination file created\n")

	written, err := io.Copy(destFile, sourceFile)
	if err != nil {
		fmt.Printf("  ❌ Failed to copy content: %v\n", err)
		return fmt.Errorf("failed to copy content: %w", err)
	}
	fmt.Printf("  ✅ Content copied: %d bytes\n", written)

	err = os.Chmod(destPath, config.Permissions)
	if err != nil {
		fmt.Printf("  ⚠️  Failed to set permissions: %v\n", err)
	} else {
		fmt.Printf("  ✅ Permissions set: %o\n", config.Permissions)
	}

	// Verify the result
	fmt.Printf("\n✅ Verification:\n")
	finalInfo, err := os.Stat(destPath)
	if err != nil {
		fmt.Printf("  ❌ Failed to stat final file: %v\n", err)
		return fmt.Errorf("verification failed: %w", err)
	}

	fmt.Printf("  File exists: %s\n", destPath)
	fmt.Printf("  Size: %d bytes\n", finalInfo.Size())
	fmt.Printf("  Mode: %s\n", finalInfo.Mode())
	fmt.Printf("  Modified: %s\n", finalInfo.ModTime().Format("2006-01-02 15:04:05"))

	// Show content preview of result
	fmt.Printf("\n📄 Result Content Preview (first 200 chars):\n")
	resultContent, err := os.ReadFile(destPath)
	if err != nil {
		fmt.Printf("  ❌ Could not read result: %v\n", err)
	} else {
		preview := string(resultContent)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		fmt.Printf("  Content: %q\n", preview)
	}

	fmt.Printf("\n🎉 Test copy completed successfully!\n")
	return nil
}
