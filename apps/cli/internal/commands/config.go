package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

// ConfigInfo represents configuration file information
type ConfigInfo struct {
	Path         string    `yaml:"path"`
	Size         int64     `yaml:"size"`
	ModTime      time.Time `yaml:"modified"`
	Exists       bool      `yaml:"exists"`
	Valid        bool      `yaml:"valid"`
	ErrorMessage string    `yaml:"error,omitempty"`
}

// ConfigSummary represents a summary of all configuration files
type ConfigSummary struct {
	ConfigDir        string     `yaml:"config_dir"`
	Applications     ConfigInfo `yaml:"applications"`
	Environment      ConfigInfo `yaml:"environment"`
	System           ConfigInfo `yaml:"system"`
	Desktop          ConfigInfo `yaml:"desktop"`
	InitMetadata     ConfigInfo `yaml:"init_metadata"`
	Backups          []string   `yaml:"backups"`
	TotalSize        int64      `yaml:"total_size"`
	LastModified     time.Time  `yaml:"last_modified"`
	ValidationErrors []string   `yaml:"validation_errors,omitempty"`
}

// NewConfigCmd creates a new config command
func NewConfigCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage DevEx configuration files",
		Long: `Manage and inspect DevEx configuration files.

This command provides utilities for:
  • Viewing current configuration status
  • Editing configuration files  
  • Validating configuration syntax and content
  • Comparing configurations
  • Managing configuration backups

Examples:
  # Show configuration summary
  devex config show
  
  # Edit applications configuration
  devex config edit applications
  
  # Validate all configuration files
  devex config validate
  
  # Compare current config with backup
  devex config diff backup-20240817.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newConfigShowCmd(settings))
	cmd.AddCommand(newConfigEditCmd(settings))
	cmd.AddCommand(newConfigValidateCmd(settings))
	cmd.AddCommand(newConfigDiffCmd(settings))
	cmd.AddCommand(newConfigInheritanceCmd(settings))
	cmd.AddCommand(newConfigTeamCmd(settings))
	cmd.AddCommand(newConfigEnvironmentCmd(settings))
	cmd.AddCommand(newConfigExportCmd(settings))
	cmd.AddCommand(newConfigImportCmd(settings))
	cmd.AddCommand(newConfigBackupCmd(settings))
	cmd.AddCommand(newConfigVersionCmd(settings))
	cmd.AddCommand(newConfigUndoCmd(settings))

	// Add contextual help integration
	AddContextualHelp(cmd, help.ContextConfiguration, "config")

	return cmd
}

// newConfigShowCmd creates the show subcommand
func newConfigShowCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		format   string
		detailed bool
	)

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration status",
		Long: `Display information about your DevEx configuration files.

Shows file status, sizes, modification times, and validation status
for all configuration files in your DevEx setup.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			summary, err := buildConfigSummary(settings)
			if err != nil {
				return fmt.Errorf("failed to build config summary: %w", err)
			}

			switch format {
			case "json":
				return outputJSON(summary)
			case "yaml":
				return outputYAML(summary)
			default:
				return outputConfigTable(summary, detailed)
			}
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().BoolVarP(&detailed, "detailed", "d", false, "Show detailed information")

	return cmd
}

// newConfigEditCmd creates the edit subcommand
func newConfigEditCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var editor string

	cmd := &cobra.Command{
		Use:   "edit [config-type]",
		Short: "Edit configuration files",
		Long: `Edit DevEx configuration files using your preferred editor.

Available configuration types:
  • applications - Application installations and settings
  • environment  - Programming languages and shell settings  
  • system       - Git, SSH, and terminal configurations
  • desktop      - Desktop environment and theme settings
  • all          - Open all configuration files

Examples:
  # Edit applications configuration
  devex config edit applications
  
  # Edit all configurations
  devex config edit all
  
  # Use specific editor
  devex config edit system --editor nano`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configType := "all"
			if len(args) > 0 {
				configType = args[0]
			}

			return editConfiguration(settings, configType, editor)
		},
	}

	cmd.Flags().StringVarP(&editor, "editor", "e", "", "Editor to use (defaults to $EDITOR or vim)")

	return cmd
}

// newConfigValidateCmd creates the validate subcommand
func newConfigValidateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		fix    bool
		strict bool
	)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration files",
		Long: `Validate DevEx configuration files for syntax and content errors.

Checks for:
  • YAML syntax errors
  • Required fields and structure
  • Valid application configurations
  • Dependency consistency
  • File permissions and accessibility

Examples:
  # Basic validation
  devex config validate
  
  # Strict validation with detailed checks
  devex config validate --strict
  
  # Validate and attempt to fix common issues
  devex config validate --fix`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return validateConfigurationFiles(settings, fix, strict)
		},
	}

	cmd.Flags().BoolVar(&fix, "fix", false, "Attempt to fix common validation issues")
	cmd.Flags().BoolVar(&strict, "strict", false, "Perform strict validation checks")

	return cmd
}

// newConfigDiffCmd creates the diff subcommand
func newConfigDiffCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		backup string
		tool   string
	)

	cmd := &cobra.Command{
		Use:   "diff [backup-file]",
		Short: "Compare configurations",
		Long: `Compare current configuration with a backup or different version.

Shows differences between your current configuration and:
  • A specific backup file
  • The default configuration
  • Another configuration directory

Examples:
  # Compare with latest backup
  devex config diff
  
  # Compare with specific backup
  devex config diff backup-20240817.yaml
  
  # Use specific diff tool
  devex config diff --tool vimdiff`,
		RunE: func(cmd *cobra.Command, args []string) error {
			backupFile := backup
			if len(args) > 0 {
				backupFile = args[0]
			}

			return compareConfigurations(settings, backupFile, tool)
		},
	}

	cmd.Flags().StringVarP(&backup, "backup", "b", "", "Specific backup file to compare against")
	cmd.Flags().StringVarP(&tool, "tool", "t", "", "Diff tool to use (diff, vimdiff, code --diff)")

	return cmd
}

// buildConfigSummary creates a summary of all configuration files
func buildConfigSummary(settings config.CrossPlatformSettings) (*ConfigSummary, error) {
	configDir := settings.GetConfigDir()

	summary := &ConfigSummary{
		ConfigDir: configDir,
	}

	// Check each configuration file
	configFiles := map[string]*ConfigInfo{
		"applications.yaml": &summary.Applications,
		"environment.yaml":  &summary.Environment,
		"system.yaml":       &summary.System,
		"desktop.yaml":      &summary.Desktop,
		"devex.yaml":        &summary.InitMetadata,
	}

	var totalSize int64
	var lastModified time.Time

	for filename, configInfo := range configFiles {
		path := filepath.Join(configDir, filename)
		configInfo.Path = path

		if stat, err := os.Stat(path); err == nil {
			configInfo.Exists = true
			configInfo.Size = stat.Size()
			configInfo.ModTime = stat.ModTime()
			totalSize += stat.Size()

			if stat.ModTime().After(lastModified) {
				lastModified = stat.ModTime()
			}

			// Validate YAML syntax
			if data, err := os.ReadFile(path); err == nil {
				var content interface{}
				if err := yaml.Unmarshal(data, &content); err != nil {
					configInfo.Valid = false
					configInfo.ErrorMessage = err.Error()
					summary.ValidationErrors = append(summary.ValidationErrors,
						fmt.Sprintf("%s: %v", filename, err))
				} else {
					configInfo.Valid = true
				}
			}
		} else {
			configInfo.Exists = false
			// Desktop config is optional
			if filename != "desktop.yaml" && filename != "devex.yaml" {
				summary.ValidationErrors = append(summary.ValidationErrors,
					fmt.Sprintf("%s: file not found", filename))
			}
		}
	}

	summary.TotalSize = totalSize
	summary.LastModified = lastModified

	// Check for backups
	backupDir := filepath.Join(configDir, "backups")
	if entries, err := os.ReadDir(backupDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
				summary.Backups = append(summary.Backups, entry.Name())
			}
		}
	}

	return summary, nil
}

// outputConfigTable displays the configuration summary as a formatted table
func outputConfigTable(summary *ConfigSummary, detailed bool) error {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("%s DevEx Configuration Summary\n", cyan("📋"))
	fmt.Printf("Config Directory: %s\n\n", summary.ConfigDir)

	// Configuration files table
	fmt.Printf("%s Configuration Files:\n", cyan("📁"))
	fmt.Printf("%-15s %-8s %-10s %-20s %s\n", "FILE", "STATUS", "SIZE", "MODIFIED", "NOTES")
	fmt.Println(strings.Repeat("─", 80))

	configs := []struct {
		name string
		info ConfigInfo
	}{
		{"applications", summary.Applications},
		{"environment", summary.Environment},
		{"system", summary.System},
		{"desktop", summary.Desktop},
		{"init metadata", summary.InitMetadata},
	}

	for _, cfg := range configs {
		status := red("✗ Missing")
		size := "-"
		modified := "-"
		notes := ""

		if cfg.info.Exists {
			if cfg.info.Valid {
				status = green("✓ Valid")
			} else {
				status = yellow("⚠ Invalid")
				notes = cfg.info.ErrorMessage
			}
			size = formatSize(cfg.info.Size)
			modified = cfg.info.ModTime.Format("Jan 02 15:04")
		} else if cfg.name == "desktop" || cfg.name == "init metadata" {
			status = yellow("- Optional")
		}

		fmt.Printf("%-15s %-8s %-10s %-20s %s\n", cfg.name, status, size, modified, notes)
	}

	fmt.Println()

	// Summary statistics
	fmt.Printf("%s Statistics:\n", cyan("📊"))
	fmt.Printf("Total Size: %s\n", formatSize(summary.TotalSize))
	if !summary.LastModified.IsZero() {
		fmt.Printf("Last Modified: %s\n", summary.LastModified.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Backups: %d\n", len(summary.Backups))

	// Validation errors
	if len(summary.ValidationErrors) > 0 {
		fmt.Printf("\n%s Validation Issues:\n", red("⚠️"))
		for _, err := range summary.ValidationErrors {
			fmt.Printf("  • %s\n", err)
		}
	} else {
		fmt.Printf("\n%s All configuration files are valid!\n", green("✅"))
	}

	// Backup information
	if detailed && len(summary.Backups) > 0 {
		fmt.Printf("\n%s Available Backups:\n", cyan("💾"))
		for _, backup := range summary.Backups {
			fmt.Printf("  • %s\n", backup)
		}
	}

	return nil
}

// outputJSON displays the configuration summary as JSON
func outputJSON(summary *ConfigSummary) error {
	data, err := yaml.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	// Convert YAML to JSON for better formatting
	var jsonData interface{}
	if err := yaml.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("failed to convert to JSON: %w", err)
	}

	fmt.Printf("%+v\n", jsonData)
	return nil
}

// outputYAML displays the configuration summary as YAML
func outputYAML(summary *ConfigSummary) error {
	data, err := yaml.Marshal(summary)
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

// editConfiguration opens configuration files for editing
func editConfiguration(settings config.CrossPlatformSettings, configType, editor string) error {
	configDir := settings.GetConfigDir()

	// Determine editor
	if editor == "" {
		editor = os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}
	}

	// Determine files to edit
	var files []string
	switch configType {
	case "applications":
		files = []string{filepath.Join(configDir, "applications.yaml")}
	case "environment":
		files = []string{filepath.Join(configDir, "environment.yaml")}
	case "system":
		files = []string{filepath.Join(configDir, "system.yaml")}
	case "desktop":
		files = []string{filepath.Join(configDir, "desktop.yaml")}
	case "all":
		files = []string{
			filepath.Join(configDir, "applications.yaml"),
			filepath.Join(configDir, "environment.yaml"),
			filepath.Join(configDir, "system.yaml"),
			filepath.Join(configDir, "desktop.yaml"),
		}
	default:
		return fmt.Errorf("unknown configuration type: %s", configType)
	}

	// Create files if they don't exist
	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(file), 0750); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Create minimal valid YAML
			content := "# DevEx Configuration\n# Add your configuration below\n\n"
			if err := os.WriteFile(file, []byte(content), 0600); err != nil {
				return fmt.Errorf("failed to create config file %s: %w", file, err)
			}
		}
	}

	// Open editor
	args := append([]string{}, files...)
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, editor, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Opening %d file(s) with %s...\n", len(files), editor)
	return cmd.Run()
}

// validateConfigurationFiles validates all configuration files
func validateConfigurationFiles(settings config.CrossPlatformSettings, fix, strict bool) error {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	fmt.Printf("%s Validating DevEx Configuration...\n\n", yellow("🔍"))

	summary, err := buildConfigSummary(settings)
	if err != nil {
		return fmt.Errorf("failed to build config summary: %w", err)
	}

	errors := 0
	warnings := 0

	// Validate individual files
	configs := []struct {
		name     string
		info     ConfigInfo
		required bool
	}{
		{"applications.yaml", summary.Applications, true},
		{"environment.yaml", summary.Environment, true},
		{"system.yaml", summary.System, true},
		{"desktop.yaml", summary.Desktop, false},
	}

	for _, cfg := range configs {
		fmt.Printf("Checking %s... ", cfg.name)

		if !cfg.info.Exists {
			if cfg.required {
				fmt.Printf("%s\n", red("❌ Missing (required)"))
				errors++
			} else {
				fmt.Printf("%s\n", yellow("⚠️ Missing (optional)"))
				warnings++
			}
			continue
		}

		if !cfg.info.Valid {
			fmt.Printf("%s\n", red("❌ Invalid YAML"))
			fmt.Printf("  Error: %s\n", cfg.info.ErrorMessage)
			errors++
			continue
		}

		// Additional validation for strict mode
		if strict {
			if err := validateFileContent(cfg.info.Path, settings); err != nil {
				fmt.Printf("%s\n", yellow("⚠️ Content issues"))
				fmt.Printf("  Warning: %s\n", err)
				warnings++
			} else {
				fmt.Printf("%s\n", green("✅ Valid"))
			}
		} else {
			fmt.Printf("%s\n", green("✅ Valid"))
		}
	}

	// Summary
	fmt.Println()
	if errors == 0 && warnings == 0 {
		fmt.Printf("%s All configuration files are valid!\n", green("🎉"))
	} else {
		if errors > 0 {
			fmt.Printf("%s Found %d error(s)\n", red("❌"), errors)
		}
		if warnings > 0 {
			fmt.Printf("%s Found %d warning(s)\n", yellow("⚠️"), warnings)
		}

		if fix && errors > 0 {
			fmt.Printf("\n%s Attempting to fix issues...\n", yellow("🔧"))
			// TODO: Implement auto-fix logic
			fmt.Printf("%s Auto-fix not yet implemented\n", yellow("ℹ️"))
		}
	}

	if errors > 0 {
		return fmt.Errorf("validation failed with %d errors", errors)
	}

	return nil
}

// validateFileContent performs content validation on configuration files
func validateFileContent(filePath string, settings config.CrossPlatformSettings) error {
	// This is a placeholder for more sophisticated content validation
	// Could check for:
	// - Valid application names
	// - Dependency consistency
	// - Platform compatibility
	// - Required fields
	return nil
}

// compareConfigurations compares current config with a backup
func compareConfigurations(settings config.CrossPlatformSettings, backupFile, tool string) error {
	configDir := settings.GetConfigDir()

	// Find backup file if not specified
	if backupFile == "" {
		backupDir := filepath.Join(configDir, "backups")
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			return fmt.Errorf("no backups directory found: %w", err)
		}

		// Find the most recent backup
		var latestBackup string
		var latestTime time.Time
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".yaml") {
				if info, err := entry.Info(); err == nil {
					if info.ModTime().After(latestTime) {
						latestTime = info.ModTime()
						latestBackup = entry.Name()
					}
				}
			}
		}

		if latestBackup == "" {
			return fmt.Errorf("no backup files found")
		}

		backupFile = filepath.Join(backupDir, latestBackup)
		fmt.Printf("Comparing with latest backup: %s\n", latestBackup)
	} else if !filepath.IsAbs(backupFile) {
		// Relative path, assume it's in the backups directory
		backupFile = filepath.Join(configDir, "backups", backupFile)
	}

	// Check if backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	// Determine diff tool
	if tool == "" {
		switch runtime.GOOS {
		case "windows":
			tool = "fc"
		default:
			tool = "diff"
		}
	}

	// Compare each configuration file
	configFiles := []string{"applications.yaml", "environment.yaml", "system.yaml", "desktop.yaml"}

	for _, configFile := range configFiles {
		currentFile := filepath.Join(configDir, configFile)
		if _, err := os.Stat(currentFile); os.IsNotExist(err) {
			continue // Skip missing files
		}

		fmt.Printf("\n📄 Comparing %s:\n", configFile)

		ctx := context.Background()
		var cmd *exec.Cmd
		switch tool {
		case "code":
			cmd = exec.CommandContext(ctx, "code", "--diff", backupFile, currentFile)
		case "vimdiff":
			cmd = exec.CommandContext(ctx, "vimdiff", backupFile, currentFile)
		default:
			cmd = exec.CommandContext(ctx, tool, "-u", backupFile, currentFile)
		}

		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			// diff returns exit code 1 when files differ, which is normal
			if cmd.ProcessState.ExitCode() != 1 {
				fmt.Printf("Warning: diff command failed: %v\n", err)
			}
		}
	}

	return nil
}

// formatSize formats a file size in bytes to a human-readable string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// newConfigInheritanceCmd creates the inheritance subcommand
func newConfigInheritanceCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inheritance",
		Short: "Show configuration inheritance hierarchy",
		Long: `Display how configuration files are inherited from defaults to user overrides.

This shows the environment-aware configuration inheritance system where:
- Default configs are loaded from ~/.local/share/devex/config/
- Default environment configs from ~/.local/share/devex/config/environments/{env}/
- Team configs are applied from ~/.devex/team/ (or $DEVEX_TEAM_CONFIG_DIR)
- Team environment configs from ~/.devex/team/environments/{env}/
- User configs are applied from ~/.devex/config/
- User environment configs from ~/.devex/config/environments/{env}/
- Each tier overrides the previous with environment-specific configs having higher priority`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showConfigInheritance(settings)
		},
	}

	return cmd
}

// showConfigInheritance displays the configuration inheritance hierarchy
func showConfigInheritance(settings config.CrossPlatformSettings) error {
	defaultDir, teamDir, userDir, envDirs := settings.GetConfigDirsWithEnvironment()
	currentEnv := settings.GetEnvironment()

	fmt.Printf("📁 Environment-Aware Configuration Inheritance\n\n")
	fmt.Printf("Current Environment: %s\n\n", currentEnv)

	fmt.Printf("1. Default Configs (loaded first - lowest priority):\n")
	fmt.Printf("   📂 %s\n", defaultDir)

	// List default config files
	showConfigTier(defaultDir, "📄", false)

	fmt.Printf("\n2. Default Environment Configs (%s):\n", currentEnv)
	fmt.Printf("   📂 %s\n", envDirs["default"])
	showConfigTier(envDirs["default"], "🌍", true)

	fmt.Printf("\n3. Team Configs:\n")
	fmt.Printf("   📂 %s\n", teamDir)
	if teamConfigEnv := os.Getenv("DEVEX_TEAM_CONFIG_DIR"); teamConfigEnv != "" {
		fmt.Printf("   ℹ️  Using custom team config from DEVEX_TEAM_CONFIG_DIR\n")
	}
	showConfigTier(teamDir, "🏢", true)

	fmt.Printf("\n4. Team Environment Configs (%s):\n", currentEnv)
	fmt.Printf("   📂 %s\n", envDirs["team"])
	showConfigTier(envDirs["team"], "🏢🌍", true)

	fmt.Printf("\n5. User Configs:\n")
	fmt.Printf("   📂 %s\n", userDir)
	showConfigTier(userDir, "👤", true)

	fmt.Printf("\n6. User Environment Configs (%s - highest priority):\n", currentEnv)
	fmt.Printf("   📂 %s\n", envDirs["user"])
	showConfigTier(envDirs["user"], "👤🌍", true)

	fmt.Printf("\n💡 Tips:\n")
	fmt.Printf("• Set DEVEX_ENV, ENVIRONMENT, or NODE_ENV to change environment (current: %s)\n", currentEnv)
	fmt.Printf("• Create environment-specific configs in environments/{env}/ subdirectories\n")
	fmt.Printf("• Environment configs override base configs in the same tier\n")
	fmt.Printf("• Set DEVEX_TEAM_CONFIG_DIR to use a shared team configuration location\n")
	fmt.Printf("• Use 'devex config diff <file>' to compare between layers\n")

	return nil
}

// showConfigTier displays configuration files for a specific tier
func showConfigTier(dir, icon string, showEmpty bool) {
	if entries, err := os.ReadDir(dir); err == nil {
		hasConfigs := false
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
				hasConfigs = true
				info, _ := entry.Info()
				fmt.Printf("   %s %s (%s)\n", icon, entry.Name(), formatSize(info.Size()))
			}
		}
		if !hasConfigs && showEmpty {
			fmt.Printf("   📭 No configs found\n")
		}
	} else {
		if showEmpty {
			fmt.Printf("   📭 Directory not found\n")
		} else {
			fmt.Printf("   ⚠️  Directory not found\n")
		}
	}
}

// newConfigTeamCmd creates the team subcommand
func newConfigTeamCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Manage team/organization configurations",
		Long: `Manage team or organization-wide configuration files.

Team configurations provide a middle layer in the inheritance hierarchy:
- Default configs (lowest priority)
- Team configs (middle priority) - shared across team/organization
- User configs (highest priority) - personal overrides

Team configs can be stored in:
- ~/.devex/team/ (default)
- Custom location via DEVEX_TEAM_CONFIG_DIR environment variable

Examples:
  # Show team config status
  devex config team status
  
  # Initialize team configs from current user configs
  devex config team init
  
  # Sync team configs from a shared repository
  devex config team sync https://github.com/company/devex-team-config.git`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add team subcommands
	cmd.AddCommand(newConfigTeamStatusCmd(settings))
	cmd.AddCommand(newConfigTeamInitCmd(settings))
	cmd.AddCommand(newConfigTeamSyncCmd(settings))

	return cmd
}

// newConfigTeamStatusCmd creates the team status subcommand
func newConfigTeamStatusCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show team configuration status",
		Long:  `Display information about team configuration files and their status.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showTeamConfigStatus(settings)
		},
	}

	return cmd
}

// newConfigTeamInitCmd creates the team init subcommand
func newConfigTeamInitCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		fromUser bool
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize team configuration",
		Long: `Initialize team configuration files.

This creates team configuration files that can be shared across your organization.
You can initialize from your current user configs or start with minimal defaults.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return initTeamConfig(settings, fromUser, force)
		},
	}

	cmd.Flags().BoolVar(&fromUser, "from-user", false, "Initialize team config from current user config")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing team config")

	return cmd
}

// newConfigTeamSyncCmd creates the team sync subcommand
func newConfigTeamSyncCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		branch string
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "sync [repository-url]",
		Short: "Sync team configuration from repository",
		Long: `Synchronize team configuration from a Git repository.

This allows teams to maintain shared configurations in version control
and sync them across team members' development environments.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repoURL := ""
			if len(args) > 0 {
				repoURL = args[0]
			}
			return syncTeamConfig(settings, repoURL, branch, force)
		},
	}

	cmd.Flags().StringVar(&branch, "branch", "main", "Git branch to sync from")
	cmd.Flags().BoolVar(&force, "force", false, "Force sync, overwriting local changes")

	return cmd
}

// showTeamConfigStatus displays team configuration status
func showTeamConfigStatus(settings config.CrossPlatformSettings) error {
	green := color.New(color.FgGreen).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	teamDir := settings.GetTeamConfigDir()

	fmt.Printf("%s Team Configuration Status\n\n", cyan("🏢"))
	fmt.Printf("Team Config Directory: %s\n", teamDir)

	if teamConfigEnv := os.Getenv("DEVEX_TEAM_CONFIG_DIR"); teamConfigEnv != "" {
		fmt.Printf("Source: DEVEX_TEAM_CONFIG_DIR environment variable\n")
	} else {
		fmt.Printf("Source: Default location\n")
	}

	// Check if directory exists
	if _, err := os.Stat(teamDir); os.IsNotExist(err) {
		fmt.Printf("Status: %s\n\n", red("❌ Not initialized"))
		fmt.Printf("%s To get started:\n", yellow("💡"))
		fmt.Printf("  • Run 'devex config team init' to create team configs\n")
		fmt.Printf("  • Or run 'devex config team init --from-user' to copy from your current setup\n")
		fmt.Printf("  • Or run 'devex config team sync <repo-url>' to sync from a repository\n")
		return nil
	}

	fmt.Printf("Status: %s\n\n", green("✅ Initialized"))

	// List team config files
	configFiles := []string{"applications.yaml", "environment.yaml", "system.yaml", "desktop.yaml"}
	fmt.Printf("%s Team Configuration Files:\n", cyan("📁"))
	fmt.Printf("%-15s %-8s %-10s %-20s\n", "FILE", "STATUS", "SIZE", "MODIFIED")
	fmt.Println(strings.Repeat("─", 60))

	hasConfigs := false
	for _, configFile := range configFiles {
		path := filepath.Join(teamDir, configFile)
		if stat, err := os.Stat(path); err == nil {
			hasConfigs = true
			status := green("✓ Present")
			size := formatSize(stat.Size())
			modified := stat.ModTime().Format("Jan 02 15:04")
			fmt.Printf("%-15s %-8s %-10s %-20s\n", configFile, status, size, modified)
		} else {
			status := yellow("- Missing")
			fmt.Printf("%-15s %-8s %-10s %-20s\n", configFile, status, "-", "-")
		}
	}

	if !hasConfigs {
		fmt.Printf("\n%s No team configuration files found\n", yellow("⚠️"))
	}

	// Check for Git repository
	gitDir := filepath.Join(teamDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		fmt.Printf("\n%s Git repository detected\n", green("🔗"))
		fmt.Printf("Team configs are version controlled\n")
	}

	return nil
}

// initTeamConfig initializes team configuration
func initTeamConfig(settings config.CrossPlatformSettings, fromUser, force bool) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	teamDir := settings.GetTeamConfigDir()

	fmt.Printf("%s Initializing Team Configuration\n\n", cyan("🏢"))
	fmt.Printf("Team directory: %s\n", teamDir)

	// Check if already exists
	if _, err := os.Stat(teamDir); !os.IsNotExist(err) && !force {
		fmt.Printf("%s Team configuration already exists\n", yellow("⚠️"))
		fmt.Printf("Use --force to overwrite existing configuration\n")
		return nil
	}

	// Create team directory
	if err := os.MkdirAll(teamDir, 0750); err != nil {
		return fmt.Errorf("failed to create team directory: %w", err)
	}

	if fromUser {
		// Copy from user config
		userDir := settings.GetUserConfigDir()
		fmt.Printf("Copying configuration from: %s\n", userDir)

		configFiles := []string{"applications.yaml", "environment.yaml", "system.yaml", "desktop.yaml"}
		for _, configFile := range configFiles {
			userPath := filepath.Join(userDir, configFile)
			teamPath := filepath.Join(teamDir, configFile)

			if data, err := os.ReadFile(userPath); err == nil {
				// Add header indicating it's a team config
				header := fmt.Sprintf("# DevEx Team Configuration - %s\n# Generated from user config\n# Share this with your team/organization\n\n", configFile)
				content := header + string(data)

				if err := os.WriteFile(teamPath, []byte(content), 0600); err != nil {
					fmt.Printf("Warning: failed to copy %s: %v\n", configFile, err)
				} else {
					fmt.Printf("  ✓ Copied %s\n", configFile)
				}
			}
		}
	} else {
		// Create minimal team configs
		fmt.Printf("Creating minimal team configuration\n")

		minimalConfigs := map[string]string{
			"applications.yaml": `# DevEx Team Applications Configuration
# Add applications that all team members should have
applications:
  - baseconfig:
      name: git
      description: Version control system
      category: development
    install_method: apt
    install_command: git
    default: true
`,
			"environment.yaml": `# DevEx Team Environment Configuration
# Define shared environment settings
shell: bash
editor: vim
`,
			"system.yaml": `# DevEx Team System Configuration
# Shared system settings
git:
  - key: user.name
    value: ""
    scope: global
  - key: user.email
    value: ""
    scope: global
`,
		}

		for filename, content := range minimalConfigs {
			path := filepath.Join(teamDir, filename)
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				fmt.Printf("Warning: failed to create %s: %v\n", filename, err)
			} else {
				fmt.Printf("  ✓ Created %s\n", filename)
			}
		}
	}

	fmt.Printf("\n%s Team configuration initialized successfully!\n", green("🎉"))
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit team configs: devex config edit\n")
	fmt.Printf("  2. Version control: git init && git add . && git commit\n")
	fmt.Printf("  3. Share with team: git remote add origin <repo-url> && git push\n")
	fmt.Printf("  4. Team members sync: devex config team sync <repo-url>\n")

	return nil
}

// syncTeamConfig syncs team configuration from a repository
func syncTeamConfig(settings config.CrossPlatformSettings, repoURL, branch string, force bool) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	teamDir := settings.GetTeamConfigDir()

	if repoURL == "" {
		// Check if already a git repository
		gitDir := filepath.Join(teamDir, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			return fmt.Errorf("no repository URL provided and team config is not a git repository")
		}

		fmt.Printf("%s Syncing team configuration from existing repository\n\n", cyan("🔄"))

		// Pull from existing repository
		ctx := context.Background()
		cmd := exec.CommandContext(ctx, "git", "-C", teamDir, "pull", "origin", branch)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to pull from repository: %w", err)
		}

		fmt.Printf("\n%s Team configuration synced successfully!\n", green("✅"))
		return nil
	}

	fmt.Printf("%s Syncing team configuration from repository\n\n", cyan("🔄"))
	fmt.Printf("Repository: %s\n", repoURL)
	fmt.Printf("Branch: %s\n", branch)
	fmt.Printf("Target: %s\n\n", teamDir)

	// Check if directory exists and has content
	if _, err := os.Stat(teamDir); !os.IsNotExist(err) && !force {
		if entries, err := os.ReadDir(teamDir); err == nil && len(entries) > 0 {
			fmt.Printf("%s Team directory exists and is not empty\n", yellow("⚠️"))
			fmt.Printf("Use --force to overwrite existing configuration\n")
			return nil
		}
	}

	// Remove existing directory if forcing
	if force {
		if err := os.RemoveAll(teamDir); err != nil {
			return fmt.Errorf("failed to remove existing team config: %w", err)
		}
	}

	// Clone repository
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "clone", "-b", branch, repoURL, teamDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	fmt.Printf("\n%s Team configuration synced successfully!\n", green("🎉"))
	fmt.Printf("\nTeam configuration is now available at: %s\n", teamDir)
	fmt.Printf("Run 'devex config inheritance' to see the full hierarchy\n")

	return nil
}

// newConfigEnvironmentCmd creates the environment subcommand
func newConfigEnvironmentCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "environment",
		Aliases: []string{"env"},
		Short:   "Manage environment-specific configurations",
		Long: `Manage environment-specific configuration files (dev, staging, prod, etc.).

Environment-specific configurations provide targeted overrides for different deployment environments:
- Development (dev) - Local development settings
- Staging (staging) - Pre-production testing environment
- Production (prod) - Live production environment
- Custom environments (test, integration, etc.)

Environment configs are stored in environments/{env}/ subdirectories within each tier:
- ~/.local/share/devex/config/environments/{env}/
- ~/.devex/team/environments/{env}/
- ~/.devex/config/environments/{env}/

Examples:
  # Show current environment and configs
  devex config env status
  
  # Create environment-specific configs
  devex config env init staging
  
  # List available environments
  devex config env list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add environment subcommands
	cmd.AddCommand(newConfigEnvStatusCmd(settings))
	cmd.AddCommand(newConfigEnvInitCmd(settings))
	cmd.AddCommand(newConfigEnvListCmd(settings))

	return cmd
}

// newConfigEnvStatusCmd creates the environment status subcommand
func newConfigEnvStatusCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current environment configuration status",
		Long:  `Display information about the current environment and its configuration files.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showEnvironmentStatus(settings)
		},
	}

	return cmd
}

// newConfigEnvInitCmd creates the environment init subcommand
func newConfigEnvInitCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		tier     string
		copyFrom string
		force    bool
	)

	cmd := &cobra.Command{
		Use:   "init [environment]",
		Short: "Initialize environment-specific configuration",
		Long: `Initialize configuration files for a specific environment.

This creates environment-specific configuration files that override base configs
for the specified environment (dev, staging, prod, etc.).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := "dev"
			if len(args) > 0 {
				env = args[0]
			}
			return initEnvironmentConfig(settings, env, tier, copyFrom, force)
		},
	}

	cmd.Flags().StringVar(&tier, "tier", "user", "Configuration tier (user, team)")
	cmd.Flags().StringVar(&copyFrom, "copy-from", "", "Copy from another environment")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing environment config")

	return cmd
}

// newConfigEnvListCmd creates the environment list subcommand
func newConfigEnvListCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available environment configurations",
		Long:  `List all available environment configurations across all tiers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listEnvironmentConfigs(settings)
		},
	}

	return cmd
}

// showEnvironmentStatus displays current environment status
func showEnvironmentStatus(settings config.CrossPlatformSettings) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	currentEnv := settings.GetEnvironment()
	_, _, _, envDirs := settings.GetConfigDirsWithEnvironment()

	fmt.Printf("%s Environment Configuration Status\n\n", cyan("🌍"))
	fmt.Printf("Current Environment: %s\n", green(currentEnv))

	// Show environment variable sources
	fmt.Printf("\nEnvironment Detection:\n")
	if devexEnv := os.Getenv("DEVEX_ENV"); devexEnv != "" {
		fmt.Printf("  DEVEX_ENV: %s (primary)\n", devexEnv)
	}
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		fmt.Printf("  ENVIRONMENT: %s\n", env)
	}
	if nodeEnv := os.Getenv("NODE_ENV"); nodeEnv != "" {
		fmt.Printf("  NODE_ENV: %s\n", nodeEnv)
	}
	if currentEnv == "dev" {
		fmt.Printf("  Default: dev (no environment variables set)\n")
	}

	// Show environment-specific config directories
	fmt.Printf("\n%s Environment Configuration Directories:\n", cyan("📁"))

	tiers := []struct {
		name  string
		path  string
		label string
	}{
		{"Default", envDirs["default"], "🌍"},
		{"Team", envDirs["team"], "🏢🌍"},
		{"User", envDirs["user"], "👤🌍"},
	}

	for _, tier := range tiers {
		fmt.Printf("\n%s %s Environment (%s):\n", tier.label, tier.name, currentEnv)
		fmt.Printf("   📂 %s\n", tier.path)

		if _, err := os.Stat(tier.path); os.IsNotExist(err) {
			fmt.Printf("   📭 Directory not found\n")
			continue
		}

		configFiles := []string{"applications.yaml", "environment.yaml", "system.yaml", "desktop.yaml"}
		hasConfigs := false

		for _, configFile := range configFiles {
			path := filepath.Join(tier.path, configFile)
			if stat, err := os.Stat(path); err == nil {
				hasConfigs = true
				size := formatSize(stat.Size())
				modified := stat.ModTime().Format("Jan 02 15:04")
				fmt.Printf("   %s %s (%s) - %s\n", tier.label, configFile, size, modified)
			}
		}

		if !hasConfigs {
			fmt.Printf("   📭 No environment configs found\n")
		}
	}

	fmt.Printf("\n%s Quick Actions:\n", yellow("💡"))
	fmt.Printf("  • Change environment: export DEVEX_ENV=staging\n")
	fmt.Printf("  • Create environment configs: devex config env init %s\n", currentEnv)
	fmt.Printf("  • View full hierarchy: devex config inheritance\n")

	return nil
}

// initEnvironmentConfig initializes environment-specific configuration
func initEnvironmentConfig(settings config.CrossPlatformSettings, env, tier, copyFrom string, force bool) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	// Determine target directory based on tier
	var baseDir string
	switch tier {
	case "user":
		baseDir = settings.GetUserConfigDir()
	case "team":
		baseDir = settings.GetTeamConfigDir()
	default:
		return fmt.Errorf("invalid tier '%s'. Use 'user' or 'team'", tier)
	}

	envDir := filepath.Join(baseDir, "environments", env)

	fmt.Printf("%s Initializing Environment Configuration\n\n", cyan("🌍"))
	fmt.Printf("Environment: %s\n", env)
	fmt.Printf("Tier: %s\n", tier)
	fmt.Printf("Directory: %s\n", envDir)

	// Check if already exists
	if _, err := os.Stat(envDir); !os.IsNotExist(err) && !force {
		fmt.Printf("%s Environment configuration already exists\n", yellow("⚠️"))
		fmt.Printf("Use --force to overwrite existing configuration\n")
		return nil
	}

	// Create environment directory
	if err := os.MkdirAll(envDir, 0750); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	if copyFrom != "" {
		// Copy from another environment
		srcDir := filepath.Join(baseDir, "environments", copyFrom)
		fmt.Printf("Copying configuration from environment: %s\n", copyFrom)

		configFiles := []string{"applications.yaml", "environment.yaml", "system.yaml", "desktop.yaml"}
		for _, configFile := range configFiles {
			srcPath := filepath.Join(srcDir, configFile)
			destPath := filepath.Join(envDir, configFile)

			if data, err := os.ReadFile(srcPath); err == nil {
				// Add header indicating it's an environment config
				header := fmt.Sprintf("# DevEx %s Environment Configuration - %s\n# Copied from %s environment\n# Environment-specific overrides\n\n", env, configFile, copyFrom)
				content := header + string(data)

				if err := os.WriteFile(destPath, []byte(content), 0600); err != nil {
					fmt.Printf("Warning: failed to copy %s: %v\n", configFile, err)
				} else {
					fmt.Printf("  ✓ Copied %s\n", configFile)
				}
			}
		}
	} else {
		// Create minimal environment configs
		fmt.Printf("Creating minimal environment configuration\n")

		envConfigs := map[string]string{
			"applications.yaml": fmt.Sprintf(`# DevEx %s Environment Applications Configuration
# Environment-specific application overrides
# These applications will be added/modified for the %s environment only
applications: []
`, env, env),
			"environment.yaml": fmt.Sprintf(`# DevEx %s Environment Configuration
# Environment-specific settings
# Example: different shell or editor for %s
shell: bash
editor: vim
`, env, env),
			"system.yaml": fmt.Sprintf(`# DevEx %s Environment System Configuration
# Environment-specific system settings
# Example: different git configs for %s
git: []
`, env, env),
		}

		for filename, content := range envConfigs {
			path := filepath.Join(envDir, filename)
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				fmt.Printf("Warning: failed to create %s: %v\n", filename, err)
			} else {
				fmt.Printf("  ✓ Created %s\n", filename)
			}
		}
	}

	fmt.Printf("\n%s Environment configuration initialized successfully!\n", green("🎉"))
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit environment configs: devex config edit\n")
	fmt.Printf("  2. Set environment: export DEVEX_ENV=%s\n", env)
	fmt.Printf("  3. Test configuration: devex config inheritance\n")

	return nil
}

// listEnvironmentConfigs lists all available environment configurations
func listEnvironmentConfigs(settings config.CrossPlatformSettings) error {
	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()

	currentEnv := settings.GetEnvironment()
	defaultDir, teamDir, userDir, _ := settings.GetConfigDirsWithEnvironment()

	fmt.Printf("%s Available Environment Configurations\n\n", cyan("🌍"))
	fmt.Printf("Current Environment: %s\n\n", green(currentEnv))

	// Collect all environments from all tiers
	envMap := make(map[string]map[string]bool) // env -> tier -> hasConfigs

	tiers := []struct {
		name string
		path string
	}{
		{"Default", defaultDir},
		{"Team", teamDir},
		{"User", userDir},
	}

	for _, tier := range tiers {
		envPath := filepath.Join(tier.path, "environments")
		if entries, err := os.ReadDir(envPath); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					envName := entry.Name()
					if envMap[envName] == nil {
						envMap[envName] = make(map[string]bool)
					}

					// Check if this environment has any config files
					envDir := filepath.Join(envPath, envName)
					if envEntries, err := os.ReadDir(envDir); err == nil {
						hasConfigs := false
						for _, envEntry := range envEntries {
							if !envEntry.IsDir() && strings.HasSuffix(envEntry.Name(), ".yaml") {
								hasConfigs = true
								break
							}
						}
						envMap[envName][tier.name] = hasConfigs
					}
				}
			}
		}
	}

	if len(envMap) == 0 {
		fmt.Printf("%s No environment configurations found\n\n", yellow("📭"))
		fmt.Printf("Create your first environment:\n")
		fmt.Printf("  devex config env init dev\n")
		fmt.Printf("  devex config env init staging\n")
		fmt.Printf("  devex config env init prod\n")
		return nil
	}

	// Display environments
	fmt.Printf("%-12s %-8s %-8s %-8s %s\n", "ENVIRONMENT", "DEFAULT", "TEAM", "USER", "STATUS")
	fmt.Println(strings.Repeat("─", 50))

	for env := range envMap {
		status := ""
		if env == currentEnv {
			status = green("(current)")
		}

		defaultIcon := "❌"
		teamIcon := "❌"
		userIcon := "❌"

		if envMap[env]["Default"] {
			defaultIcon = "✅"
		}
		if envMap[env]["Team"] {
			teamIcon = "✅"
		}
		if envMap[env]["User"] {
			userIcon = "✅"
		}

		fmt.Printf("%-12s %-8s %-8s %-8s %s\n", env, defaultIcon, teamIcon, userIcon, status)
	}

	fmt.Printf("\n%s Legend:\n", cyan("💡"))
	fmt.Printf("  ✅ Environment has configuration files\n")
	fmt.Printf("  ❌ Environment has no configuration files\n")
	fmt.Printf("\nCreate new environment: devex config env init <environment>\n")

	return nil
}

// newConfigExportCmd creates the export subcommand
func newConfigExportCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		format   string
		output   string
		include  []string
		exclude  []string
		bundle   bool
		compress bool
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export DevEx configurations",
		Long: `Export DevEx configuration files for backup, sharing, or migration.

This command allows you to export your configurations in various formats:
- YAML (default) - Individual config files
- JSON - Machine-readable format
- Bundle - Combined archive with all configs
- Archive - Compressed bundle for storage/transfer

Examples:
  # Export all configs to YAML
  devex config export

  # Export to JSON format
  devex config export --format json --output configs.json

  # Create compressed bundle
  devex config export --bundle --compress --output devex-config.tar.gz

  # Export only specific configs
  devex config export --include applications,environment`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for --no-tui flag
			noTUI, _ := cmd.Flags().GetBool("no-tui")

			if !noTUI {
				return runConfigExportWithProgress(settings, format, output, include, exclude, bundle, compress)
			}

			// Fallback to original implementation
			return exportConfiguration(settings, format, output, include, exclude, bundle, compress)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "yaml", "Export format (yaml, json, bundle, archive)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file/directory (defaults to current directory)")
	cmd.Flags().StringSliceVar(&include, "include", []string{}, "Config types to include (applications, environment, system, desktop)")
	cmd.Flags().StringSliceVar(&exclude, "exclude", []string{}, "Config types to exclude")
	cmd.Flags().BoolVar(&bundle, "bundle", false, "Create a bundled archive")
	cmd.Flags().BoolVar(&compress, "compress", false, "Compress the output (with bundle)")
	cmd.Flags().Bool("no-tui", false, "Disable TUI progress display")

	return cmd
}

// newConfigImportCmd creates the import subcommand
func newConfigImportCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		merge    bool
		force    bool
		backup   bool
		validate bool
		dryRun   bool
	)

	cmd := &cobra.Command{
		Use:   "import [file]",
		Short: "Import DevEx configurations",
		Long: `Import DevEx configuration files from backup, shared configs, or migration.

This command allows you to import configurations from various sources:
- Individual YAML/JSON files
- Bundle archives created with export
- Remote configurations via URL

Import modes:
- Merge: Combine with existing configuration (default)
- Replace: Overwrite existing configuration completely
- Validate: Check configuration without applying

Examples:
  # Import from bundle
  devex config import devex-config.tar.gz

  # Import with merge
  devex config import configs.yaml --merge

  # Import with backup of current configs
  devex config import configs.json --backup

  # Dry run to validate before import
  devex config import configs.yaml --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := ""
			if len(args) > 0 {
				inputFile = args[0]
			}
			return importConfiguration(settings, inputFile, merge, force, backup, validate, dryRun)
		},
	}

	cmd.Flags().BoolVar(&merge, "merge", true, "Merge with existing configuration")
	cmd.Flags().BoolVar(&force, "force", false, "Force import, overwriting conflicts")
	cmd.Flags().BoolVar(&backup, "backup", true, "Create backup before import")
	cmd.Flags().BoolVar(&validate, "validate", true, "Validate configuration before import")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be imported without applying")

	return cmd
}

// exportConfiguration handles the configuration export functionality
func exportConfiguration(settings config.CrossPlatformSettings, format, output string, include, exclude []string, bundle, compress bool) error {
	cyan := color.New(color.FgCyan).SprintFunc()

	fmt.Printf("%s Exporting DevEx Configuration\n\n", cyan("📤"))

	// Get all configuration directories (default, team, user, environment-specific)
	defaultDir, teamDir, userDir, envDirs := settings.GetConfigDirsWithEnvironment()
	currentEnv := settings.GetEnvironment()

	// Collect all configuration data
	exportData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"export_time":   time.Now().Format(time.RFC3339),
			"devex_version": "1.0.0", // TODO: Get from build info
			"environment":   currentEnv,
			"platform":      runtime.GOOS,
		},
		"configurations": map[string]interface{}{},
	}

	// Define configuration files to export
	configFiles := []string{"applications.yaml", "environment.yaml", "system.yaml", "desktop.yaml"}

	// Filter config files based on include/exclude
	if len(include) > 0 {
		filteredFiles := []string{}
		for _, configFile := range configFiles {
			configType := strings.TrimSuffix(configFile, ".yaml")
			for _, incl := range include {
				if configType == incl {
					filteredFiles = append(filteredFiles, configFile)
					break
				}
			}
		}
		configFiles = filteredFiles
	}

	if len(exclude) > 0 {
		filteredFiles := []string{}
		for _, configFile := range configFiles {
			configType := strings.TrimSuffix(configFile, ".yaml")
			excluded := false
			for _, excl := range exclude {
				if configType == excl {
					excluded = true
					break
				}
			}
			if !excluded {
				filteredFiles = append(filteredFiles, configFile)
			}
		}
		configFiles = filteredFiles
	}

	fmt.Printf("Exporting configuration files: %v\n", configFiles)
	fmt.Printf("Environment: %s\n", currentEnv)

	// Collect configurations from all tiers
	tiers := []struct {
		name string
		path string
		key  string
	}{
		{"Default", defaultDir, "default"},
		{"Team", teamDir, "team"},
		{"User", userDir, "user"},
		{"Default Environment", envDirs["default"], "default_env"},
		{"Team Environment", envDirs["team"], "team_env"},
		{"User Environment", envDirs["user"], "user_env"},
	}

	configurations := make(map[string]interface{})
	for _, tier := range tiers {
		tierConfigs := make(map[string]interface{})

		for _, configFile := range configFiles {
			configPath := filepath.Join(tier.path, configFile)
			if data, err := os.ReadFile(configPath); err == nil {
				var configData interface{}
				if err := yaml.Unmarshal(data, &configData); err == nil {
					configType := strings.TrimSuffix(configFile, ".yaml")
					tierConfigs[configType] = configData
					fmt.Printf("  ✓ Collected %s/%s\n", tier.name, configFile)
				} else {
					fmt.Printf("  ⚠ Warning: Failed to parse %s/%s: %v\n", tier.name, configFile, err)
				}
			}
		}

		if len(tierConfigs) > 0 {
			configurations[tier.key] = tierConfigs
		}
	}

	exportData["configurations"] = configurations

	// Handle output format and location
	if bundle {
		return exportAsBundle(exportData, output, compress)
	}

	switch format {
	case "json":
		return exportAsJSON(exportData, output)
	case "yaml":
		return exportAsYAML(exportData, output)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// exportAsYAML exports configuration as YAML format
func exportAsYAML(data map[string]interface{}, output string) error {
	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if output == "" {
		output = fmt.Sprintf("devex-config-%s.yaml", time.Now().Format("20060102-150405"))
	}

	if err := os.WriteFile(output, yamlData, 0600); err != nil {
		return fmt.Errorf("failed to write YAML file: %w", err)
	}

	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("\n%s Configuration exported to: %s\n", green("✅"), output)
	return nil
}

// exportAsJSON exports configuration as JSON format
func exportAsJSON(data map[string]interface{}, output string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if output == "" {
		output = fmt.Sprintf("devex-config-%s.json", time.Now().Format("20060102-150405"))
	}

	if err := os.WriteFile(output, jsonData, 0600); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("\n%s Configuration exported to: %s\n", green("✅"), output)
	return nil
}

// exportAsBundle exports configuration as a compressed bundle
func exportAsBundle(data map[string]interface{}, output string, compress bool) error {
	if output == "" {
		if compress {
			output = fmt.Sprintf("devex-config-%s.tar.gz", time.Now().Format("20060102-150405"))
		} else {
			output = fmt.Sprintf("devex-config-%s.tar", time.Now().Format("20060102-150405"))
		}
	}

	// Create temporary directory for bundle contents
	tmpDir, err := os.MkdirTemp("", "devex-export-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up temp directory %s: %v\n", tmpDir, removeErr)
		}
	}()

	// Write metadata
	metadataData, err := yaml.Marshal(data["metadata"])
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "metadata.yaml"), metadataData, 0600); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write individual config files
	configurations, ok := data["configurations"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid configurations data")
	}
	for tier, tierConfigs := range configurations {
		tierDir := filepath.Join(tmpDir, tier)
		if err := os.MkdirAll(tierDir, 0750); err != nil {
			return fmt.Errorf("failed to create tier directory: %w", err)
		}

		tierConfigsMap, ok := tierConfigs.(map[string]interface{})
		if !ok {
			continue
		}
		for configType, configData := range tierConfigsMap {
			configYAML, err := yaml.Marshal(configData)
			if err != nil {
				return fmt.Errorf("failed to marshal config %s/%s: %w", tier, configType, err)
			}

			configFile := filepath.Join(tierDir, configType+".yaml")
			if err := os.WriteFile(configFile, configYAML, 0600); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}
		}
	}

	// Create archive
	ctx := context.Background()
	var cmd *exec.Cmd
	if compress {
		cmd = exec.CommandContext(ctx, "tar", "-czf", output, "-C", tmpDir, ".")
	} else {
		cmd = exec.CommandContext(ctx, "tar", "-cf", output, "-C", tmpDir, ".")
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	green := color.New(color.FgGreen).SprintFunc()
	fmt.Printf("\n%s Configuration bundle exported to: %s\n", green("✅"), output)
	return nil
}

// importConfiguration handles the configuration import functionality
func importConfiguration(settings config.CrossPlatformSettings, inputFile string, merge, force, backup, validate, dryRun bool) error {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()

	if inputFile == "" {
		return fmt.Errorf("input file is required")
	}

	fmt.Printf("%s Importing DevEx Configuration\n\n", cyan("📥"))
	fmt.Printf("Input file: %s\n", inputFile)
	fmt.Printf("Mode: %s\n", map[bool]string{true: "merge", false: "replace"}[merge])

	// Check if input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file not found: %s", inputFile)
	}

	// Determine file type and extract data
	var importData map[string]interface{}
	var err error

	switch {
	case strings.HasSuffix(inputFile, ".tar.gz"), strings.HasSuffix(inputFile, ".tar"):
		importData, err = importFromBundle(inputFile)
	case strings.HasSuffix(inputFile, ".json"):
		importData, err = importFromJSON(inputFile)
	case strings.HasSuffix(inputFile, ".yaml"), strings.HasSuffix(inputFile, ".yml"):
		importData, err = importFromYAML(inputFile)
	default:
		return fmt.Errorf("unsupported file format: %s", inputFile)
	}

	if err != nil {
		return fmt.Errorf("failed to read import data: %w", err)
	}

	// Validate import data structure
	if validate {
		if err := validateImportData(importData); err != nil {
			return fmt.Errorf("import validation failed: %w", err)
		}
		fmt.Printf("%s Import data validation passed\n", green("✓"))
	}

	if dryRun {
		fmt.Printf("\n%s Dry run - showing what would be imported:\n", yellow("🔍"))
		return showImportPreview(importData, settings)
	}

	// Create backup if requested
	if backup {
		backupFile := fmt.Sprintf("devex-config-backup-%s.yaml", time.Now().Format("20060102-150405"))
		if err := exportConfiguration(settings, "yaml", backupFile, []string{}, []string{}, false, false); err != nil {
			fmt.Printf("%s Warning: Failed to create backup: %v\n", yellow("⚠"), err)
		} else {
			fmt.Printf("%s Backup created: %s\n", green("💾"), backupFile)
		}
	}

	// Apply configurations
	if err := applyImportedConfigurations(importData, settings, merge, force); err != nil {
		return fmt.Errorf("failed to apply configurations: %w", err)
	}

	fmt.Printf("\n%s Configuration imported successfully!\n", green("🎉"))
	return nil
}

// importFromBundle extracts and reads configuration from a tar bundle
func importFromBundle(bundlePath string) (map[string]interface{}, error) {
	// Create temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "devex-import-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clean up temp directory %s: %v\n", tmpDir, removeErr)
		}
	}()

	// Extract bundle
	ctx := context.Background()
	var cmd *exec.Cmd
	if strings.HasSuffix(bundlePath, ".tar.gz") {
		cmd = exec.CommandContext(ctx, "tar", "-xzf", bundlePath, "-C", tmpDir)
	} else {
		cmd = exec.CommandContext(ctx, "tar", "-xf", bundlePath, "-C", tmpDir)
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract bundle: %w", err)
	}

	// Read metadata
	metadataPath := filepath.Join(tmpDir, "metadata.yaml")
	var metadata map[string]interface{}
	if data, err := os.ReadFile(metadataPath); err == nil {
		if err := yaml.Unmarshal(data, &metadata); err != nil {
			// Metadata is optional, just log and continue
			fmt.Printf("Warning: failed to parse metadata: %v\n", err)
		}
	}

	// Read configurations from tier directories
	configurations := make(map[string]interface{})
	tiers := []string{"default", "team", "user", "default_env", "team_env", "user_env"}

	for _, tier := range tiers {
		tierDir := filepath.Join(tmpDir, tier)
		if _, err := os.Stat(tierDir); os.IsNotExist(err) {
			continue
		}

		tierConfigs := make(map[string]interface{})
		configFiles := []string{"applications.yaml", "environment.yaml", "system.yaml", "desktop.yaml"}

		for _, configFile := range configFiles {
			configPath := filepath.Join(tierDir, configFile)
			if data, err := os.ReadFile(configPath); err == nil {
				var configData interface{}
				if err := yaml.Unmarshal(data, &configData); err == nil {
					configType := strings.TrimSuffix(configFile, ".yaml")
					tierConfigs[configType] = configData
				}
			}
		}

		if len(tierConfigs) > 0 {
			configurations[tier] = tierConfigs
		}
	}

	return map[string]interface{}{
		"metadata":       metadata,
		"configurations": configurations,
	}, nil
}

// importFromJSON reads configuration from JSON file
func importFromJSON(jsonPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	var importData map[string]interface{}
	if err := json.Unmarshal(data, &importData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return importData, nil
}

// importFromYAML reads configuration from YAML file
func importFromYAML(yamlPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var importData map[string]interface{}
	if err := yaml.Unmarshal(data, &importData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return importData, nil
}

// validateImportData validates the structure of import data
func validateImportData(data map[string]interface{}) error {
	// Check for required top-level keys
	if _, ok := data["configurations"]; !ok {
		return fmt.Errorf("missing 'configurations' key in import data")
	}

	// Validate configurations structure
	configurations, ok := data["configurations"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("'configurations' must be a map")
	}

	// Validate tier structure
	validTiers := map[string]bool{
		"default": true, "team": true, "user": true,
		"default_env": true, "team_env": true, "user_env": true,
	}

	for tier := range configurations {
		if !validTiers[tier] {
			return fmt.Errorf("invalid tier: %s", tier)
		}
	}

	return nil
}

// showImportPreview shows what would be imported without applying changes
func showImportPreview(data map[string]interface{}, settings config.CrossPlatformSettings) error {
	cyan := color.New(color.FgCyan).SprintFunc()

	configurations, ok := data["configurations"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid configurations data")
	}

	fmt.Printf("\n%s Import Preview:\n", cyan("👀"))

	for tier, tierConfigs := range configurations {
		fmt.Printf("\n%s:\n", tier)
		tierConfigsMap, ok := tierConfigs.(map[string]interface{})
		if !ok {
			continue
		}
		for configType, configData := range tierConfigsMap {
			fmt.Printf("  • %s.yaml\n", configType)

			// Show some details about the config
			if configMap, ok := configData.(map[string]interface{}); ok {
				for key := range configMap {
					fmt.Printf("    - %s\n", key)
				}
			}
		}
	}

	return nil
}

// applyImportedConfigurations applies the imported configurations to the file system
func applyImportedConfigurations(data map[string]interface{}, settings config.CrossPlatformSettings, merge, force bool) error {
	green := color.New(color.FgGreen).SprintFunc()

	configurations, ok := data["configurations"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid configurations data")
	}

	// Get target directories
	defaultDir, teamDir, userDir, envDirs := settings.GetConfigDirsWithEnvironment()

	tierDirs := map[string]string{
		"default":     defaultDir,
		"team":        teamDir,
		"user":        userDir,
		"default_env": envDirs["default"],
		"team_env":    envDirs["team"],
		"user_env":    envDirs["user"],
	}

	for tier, tierConfigs := range configurations {
		targetDir, ok := tierDirs[tier]
		if !ok {
			continue
		}

		// Create target directory if it doesn't exist
		if err := os.MkdirAll(targetDir, 0750); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
		}

		tierConfigsMap, ok := tierConfigs.(map[string]interface{})
		if !ok {
			continue
		}
		for configType, configData := range tierConfigsMap {
			configFile := filepath.Join(targetDir, configType+".yaml")

			finalData := configData

			// Handle merge mode
			if merge {
				if existingData, err := os.ReadFile(configFile); err == nil {
					var existing interface{}
					if err := yaml.Unmarshal(existingData, &existing); err == nil {
						// Simple merge - new data overwrites existing
						// TODO: Implement smart merge logic for complex cases
						finalData = configData
					}
				}
			}

			// Write configuration file
			yamlData, err := yaml.Marshal(finalData)
			if err != nil {
				return fmt.Errorf("failed to marshal %s/%s: %w", tier, configType, err)
			}

			if err := os.WriteFile(configFile, yamlData, 0600); err != nil {
				return fmt.Errorf("failed to write %s: %w", configFile, err)
			}

			fmt.Printf("  %s Applied %s/%s.yaml\n", green("✓"), tier, configType)
		}
	}

	return nil
}

// newConfigBackupCmd creates the backup subcommand
func newConfigBackupCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage configuration backups",
		Long: `Create, restore, and manage backups of your DevEx configuration.

The backup system automatically creates backups before major operations
and allows manual backup creation and restoration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add backup subcommands
	cmd.AddCommand(newConfigBackupCreateCmd(settings))
	cmd.AddCommand(newConfigBackupListCmd(settings))
	cmd.AddCommand(newConfigBackupRestoreCmd(settings))
	cmd.AddCommand(newConfigBackupDeleteCmd(settings))
	cmd.AddCommand(newConfigBackupCompareCmd(settings))

	return cmd
}

// newConfigBackupCreateCmd creates a new backup
func newConfigBackupCreateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		description string
		tags        []string
		compress    bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new configuration backup",
		Long: `Create a backup of your current DevEx configuration.

Examples:
  # Create a simple backup
  devex config backup create
  
  # Create backup with description
  devex config backup create --description "Before major update"
  
  # Create compressed backup with tags
  devex config backup create --compress --tags "stable,pre-update"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check for --no-tui flag
			noTUI, _ := cmd.Flags().GetBool("no-tui")

			if !noTUI {
				return runConfigBackupWithProgress(settings, description, tags, compress)
			}

			// Fallback to original implementation
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			manager := backup.NewBackupManager(baseDir)

			backupMetadata, err := manager.CreateBackup(backup.BackupOptions{
				Description: description,
				Type:        "manual",
				Tags:        tags,
				Compress:    compress,
				MaxBackups:  backup.MaxBackups,
			})

			if err != nil {
				return fmt.Errorf("failed to create backup: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			fmt.Printf("%s Created backup: %s\n", green("✓"), backupMetadata.ID)
			fmt.Printf("  Size: %s\n", formatBytes(backupMetadata.Size))
			fmt.Printf("  Files: %d\n", len(backupMetadata.Files))
			if description != "" {
				fmt.Printf("  Description: %s\n", description)
			}
			if len(tags) > 0 {
				fmt.Printf("  Tags: %s\n", strings.Join(tags, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "Backup description")
	cmd.Flags().StringSliceVarP(&tags, "tags", "t", []string{}, "Tags for the backup")
	cmd.Flags().BoolVarP(&compress, "compress", "c", true, "Compress the backup")
	cmd.Flags().Bool("no-tui", false, "Disable TUI progress display")

	return cmd
}

// newConfigBackupListCmd lists available backups
func newConfigBackupListCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		filter string
		limit  int
		format string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available configuration backups",
		Long: `List all available configuration backups.

Examples:
  # List all backups
  devex config backup list
  
  # List backups with filter
  devex config backup list --filter "stable"
  
  # List last 5 backups
  devex config backup list --limit 5`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			manager := backup.NewBackupManager(baseDir)

			backups, err := manager.ListBackups(filter, limit)
			if err != nil {
				return fmt.Errorf("failed to list backups: %w", err)
			}

			if len(backups) == 0 {
				fmt.Println("No backups found")
				return nil
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(backups, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
				return nil
			case "yaml":
				data, err := yaml.Marshal(backups)
				if err != nil {
					return fmt.Errorf("failed to marshal YAML: %w", err)
				}
				fmt.Println(string(data))
				return nil
			default:
				green := color.New(color.FgGreen).SprintFunc()
				fmt.Printf("Found %d backups:\n\n", len(backups))
				for _, b := range backups {
					fmt.Printf("%s %s\n", green("•"), b.ID)
					fmt.Printf("  Created: %s\n", b.Timestamp.Format("2006-01-02 15:04:05"))
					fmt.Printf("  Type: %s\n", b.Type)
					fmt.Printf("  Size: %s\n", formatBytes(b.Size))
					if b.Description != "" {
						fmt.Printf("  Description: %s\n", b.Description)
					}
					if len(b.Tags) > 0 {
						fmt.Printf("  Tags: %s\n", strings.Join(b.Tags, ", "))
					}
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&filter, "filter", "f", "", "Filter backups by ID, type, or tag")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit number of results")
	cmd.Flags().StringVarP(&format, "output", "o", "table", "Output format (table, json, yaml)")

	return cmd
}

// newConfigBackupRestoreCmd restores a backup
func newConfigBackupRestoreCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		force bool
	)

	cmd := &cobra.Command{
		Use:   "restore [backup-id]",
		Short: "Restore a configuration backup",
		Long: `Restore a previously created configuration backup.

This operation will create a pre-restore backup automatically
before applying the selected backup.

Examples:
  # Restore a specific backup
  devex config backup restore backup-20240817-143022
  
  # Force restore without confirmation
  devex config backup restore backup-20240817-143022 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("backup ID required")
			}

			backupID := args[0]
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			manager := backup.NewBackupManager(baseDir)

			// Get backup info
			backupInfo, err := manager.GetBackup(backupID)
			if err != nil {
				return fmt.Errorf("failed to get backup: %w", err)
			}

			// Confirm restore
			if !force {
				fmt.Printf("Restore backup %s?\n", backupID)
				fmt.Printf("Created: %s\n", backupInfo.Timestamp.Format("2006-01-02 15:04:05"))
				if backupInfo.Description != "" {
					fmt.Printf("Description: %s\n", backupInfo.Description)
				}
				yellow := color.New(color.FgYellow).SprintFunc()
				fmt.Printf("\n%s This will replace your current configuration.\n", yellow("⚠"))
				fmt.Printf("Continue? [y/N]: ")

				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					// If scan fails, default to 'no' for safety
					fmt.Println("Restore cancelled")
					return nil
				}
				if strings.ToLower(response) != "y" {
					fmt.Println("Restore cancelled")
					return nil
				}
			}

			// Perform restore
			if err := manager.RestoreBackup(backupID, ""); err != nil {
				return fmt.Errorf("failed to restore backup: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			fmt.Printf("%s Successfully restored backup: %s\n", green("✓"), backupID)
			fmt.Println("A pre-restore backup was created automatically")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// newConfigBackupDeleteCmd deletes a backup
func newConfigBackupDeleteCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete [backup-id]",
		Short: "Delete a configuration backup",
		Long: `Delete a specific configuration backup.

Examples:
  # Delete a backup
  devex config backup delete backup-20240817-143022
  
  # Force delete without confirmation
  devex config backup delete backup-20240817-143022 --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("backup ID required")
			}

			backupID := args[0]
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			manager := backup.NewBackupManager(baseDir)

			// Confirm deletion
			if !force {
				fmt.Printf("Delete backup %s? [y/N]: ", backupID)
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					// If scan fails, default to 'no' for safety
					fmt.Println("Deletion cancelled")
					return nil
				}
				if strings.ToLower(response) != "y" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}

			if err := manager.DeleteBackup(backupID); err != nil {
				return fmt.Errorf("failed to delete backup: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			fmt.Printf("%s Deleted backup: %s\n", green("✓"), backupID)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// newConfigBackupCompareCmd compares two backups
func newConfigBackupCompareCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare [backup-id-1] [backup-id-2]",
		Short: "Compare two configuration backups",
		Long: `Compare two configuration backups to see differences.

Examples:
  # Compare two backups
  devex config backup compare backup-20240817-143022 backup-20240817-150000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("two backup IDs required")
			}

			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			manager := backup.NewBackupManager(baseDir)

			comparison, err := manager.CompareBackups(args[0], args[1])
			if err != nil {
				return fmt.Errorf("failed to compare backups: %w", err)
			}

			fmt.Printf("Comparing %s with %s:\n\n", args[0], args[1])

			green := color.New(color.FgGreen).SprintFunc()
			red := color.New(color.FgRed).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			if len(comparison.AddedFiles) > 0 {
				fmt.Printf("%s Added files:\n", green("+"))
				for _, file := range comparison.AddedFiles {
					fmt.Printf("  + %s\n", file)
				}
				fmt.Println()
			}

			if len(comparison.RemovedFiles) > 0 {
				fmt.Printf("%s Removed files:\n", red("-"))
				for _, file := range comparison.RemovedFiles {
					fmt.Printf("  - %s\n", file)
				}
				fmt.Println()
			}

			if len(comparison.ModifiedFiles) > 0 {
				fmt.Printf("%s Modified files:\n", yellow("~"))
				for _, file := range comparison.ModifiedFiles {
					fmt.Printf("  ~ %s\n", file)
				}
			}

			if len(comparison.AddedFiles) == 0 && len(comparison.RemovedFiles) == 0 && len(comparison.ModifiedFiles) == 0 {
				fmt.Println("No differences found")
			}

			return nil
		},
	}

	return cmd
}

// formatBytes formats bytes to human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// newConfigVersionCmd creates the version subcommand
func newConfigVersionCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Manage configuration versions and migrations",
		Long: `Track, migrate, and rollback configuration versions.

The version system automatically tracks configuration changes and 
provides migration paths between different versions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add version subcommands
	cmd.AddCommand(newConfigVersionShowCmd(settings))
	cmd.AddCommand(newConfigVersionListCmd(settings))
	cmd.AddCommand(newConfigVersionMigrateCmd(settings))
	cmd.AddCommand(newConfigVersionRollbackCmd(settings))
	cmd.AddCommand(newConfigVersionCompatibilityCmd(settings))
	cmd.AddCommand(newConfigVersionUpdateCmd(settings))

	return cmd
}

// newConfigVersionShowCmd shows current version info
func newConfigVersionShowCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration version",
		Long: `Display information about the current configuration version.

Shows version number, timestamp, description, changes, and hash.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			vm := version.NewVersionManager(baseDir)

			currentVersion, err := vm.GetCurrentVersion()
			if err != nil {
				return fmt.Errorf("failed to get current version: %w", err)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(currentVersion, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(currentVersion)
				if err != nil {
					return fmt.Errorf("failed to marshal YAML: %w", err)
				}
				fmt.Println(string(data))
			default:
				green := color.New(color.FgGreen).SprintFunc()
				blue := color.New(color.FgBlue).SprintFunc()

				fmt.Printf("%s Configuration Version\n\n", green("●"))
				fmt.Printf("Version: %s\n", blue(currentVersion.Version))
				fmt.Printf("Timestamp: %s\n", currentVersion.Timestamp.Format("2006-01-02 15:04:05"))
				if currentVersion.Description != "" {
					fmt.Printf("Description: %s\n", currentVersion.Description)
				}
				if currentVersion.Author != "" {
					fmt.Printf("Author: %s\n", currentVersion.Author)
				}
				fmt.Printf("Hash: %s\n", currentVersion.Hash[:16])
				if currentVersion.BackupID != "" {
					fmt.Printf("Backup ID: %s\n", currentVersion.BackupID)
				}
				if len(currentVersion.Changes) > 0 {
					fmt.Println("\nChanges:")
					for _, change := range currentVersion.Changes {
						fmt.Printf("  • %s\n", change)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")

	return cmd
}

// newConfigVersionListCmd lists all configuration versions
func newConfigVersionListCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		format string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all configuration versions",
		Long: `List all configuration versions in chronological order.

Shows version history with timestamps and descriptions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			vm := version.NewVersionManager(baseDir)

			versions, err := vm.ListVersions()
			if err != nil {
				return fmt.Errorf("failed to list versions: %w", err)
			}

			if limit > 0 && len(versions) > limit {
				versions = versions[:limit]
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(versions, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(versions)
				if err != nil {
					return fmt.Errorf("failed to marshal YAML: %w", err)
				}
				fmt.Println(string(data))
			default:
				green := color.New(color.FgGreen).SprintFunc()
				blue := color.New(color.FgBlue).SprintFunc()
				gray := color.New(color.FgHiBlack).SprintFunc()

				if len(versions) == 0 {
					fmt.Println("No versions found")
					return nil
				}

				fmt.Printf("Found %d versions:\n\n", len(versions))
				for i, v := range versions {
					var prefix string
					if i == 0 {
						prefix = green("●")
					} else {
						prefix = gray("○")
					}

					fmt.Printf("%s %s", prefix, blue(v.Version))
					if i == 0 {
						fmt.Printf(" %s", green("(current)"))
					}
					fmt.Println()
					fmt.Printf("    %s\n", v.Timestamp.Format("2006-01-02 15:04:05"))
					if v.Description != "" {
						fmt.Printf("    %s\n", v.Description)
					}
					if v.Author != "" {
						fmt.Printf("    by %s\n", v.Author)
					}
					fmt.Println()
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 0, "Limit number of versions to show")

	return cmd
}

// newConfigVersionMigrateCmd migrates to a specific version
func newConfigVersionMigrateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "migrate [target-version]",
		Short: "Migrate configuration to a specific version",
		Long: `Migrate configuration to a target version using available migrations.

Creates automatic backups before migration and validates compatibility.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("target version required")
			}

			targetVersion := args[0]
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			vm := version.NewVersionManager(baseDir)

			// Check compatibility first
			if !force {
				report, err := vm.CheckCompatibility(targetVersion)
				if err != nil {
					return fmt.Errorf("failed to check compatibility: %w", err)
				}

				if !report.Compatible {
					fmt.Printf("❌ Migration to %s is not compatible:\n", targetVersion)
					for _, issue := range report.Issues {
						fmt.Printf("  • %s\n", issue)
					}
					fmt.Println("\nUse --force to override compatibility checks")
					return fmt.Errorf("compatibility check failed")
				}

				if len(report.Warnings) > 0 {
					yellow := color.New(color.FgYellow).SprintFunc()
					fmt.Printf("%s Warnings:\n", yellow("⚠"))
					for _, warning := range report.Warnings {
						fmt.Printf("  • %s\n", warning)
					}
					fmt.Println()
				}

				if len(report.RequiredActions) > 0 {
					fmt.Println("Required actions before migration:")
					for _, action := range report.RequiredActions {
						fmt.Printf("  • %s\n", action)
					}
					fmt.Printf("\nContinue with migration? [y/N]: ")
					var response string
					if _, err := fmt.Scanln(&response); err != nil {
						// If scan fails, default to 'no' for safety
						fmt.Println("Migration cancelled")
						return nil
					}
					if strings.ToLower(response) != "y" {
						fmt.Println("Migration cancelled")
						return nil
					}
				}
			}

			// Perform migration
			if err := vm.MigrateTo(targetVersion); err != nil {
				return fmt.Errorf("migration failed: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			fmt.Printf("%s Successfully migrated to version %s\n", green("✓"), targetVersion)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force migration ignoring compatibility checks")

	return cmd
}

// newConfigVersionRollbackCmd rolls back to a previous version
func newConfigVersionRollbackCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "rollback [version]",
		Short: "Rollback to a previous configuration version",
		Long: `Rollback configuration to a previous version using backups.

This restores configuration from the backup created when the target version was active.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("target version required")
			}

			targetVersion := args[0]
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			vm := version.NewVersionManager(baseDir)

			// Confirm rollback
			if !force {
				fmt.Printf("Rollback to version %s?\n", targetVersion)
				fmt.Printf("This will restore configuration from backup.\n")
				fmt.Printf("Continue? [y/N]: ")
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					// If scan fails, default to 'no' for safety
					fmt.Println("Rollback cancelled")
					return nil
				}
				if strings.ToLower(response) != "y" {
					fmt.Println("Rollback cancelled")
					return nil
				}
			}

			// Perform rollback
			if err := vm.RollbackToVersion(targetVersion); err != nil {
				return fmt.Errorf("rollback failed: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			fmt.Printf("%s Successfully rolled back to version %s\n", green("✓"), targetVersion)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force rollback without confirmation")

	return cmd
}

// newConfigVersionCompatibilityCmd checks version compatibility
func newConfigVersionCompatibilityCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "compatibility [target-version]",
		Short: "Check compatibility with a target version",
		Long: `Check if current configuration is compatible with a target version.

Shows potential issues, warnings, and required actions for migration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("target version required")
			}

			targetVersion := args[0]
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			vm := version.NewVersionManager(baseDir)

			report, err := vm.CheckCompatibility(targetVersion)
			if err != nil {
				return fmt.Errorf("failed to check compatibility: %w", err)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(report)
				if err != nil {
					return fmt.Errorf("failed to marshal YAML: %w", err)
				}
				fmt.Println(string(data))
			default:
				green := color.New(color.FgGreen).SprintFunc()
				red := color.New(color.FgRed).SprintFunc()
				yellow := color.New(color.FgYellow).SprintFunc()

				fmt.Printf("Compatibility Report: %s → %s\n\n", report.CurrentVersion, report.TargetVersion)

				if report.Compatible {
					fmt.Printf("%s Compatible\n", green("✓"))
				} else {
					fmt.Printf("%s Not Compatible\n", red("✗"))
				}

				if len(report.Issues) > 0 {
					fmt.Printf("\n%s Issues:\n", red("●"))
					for _, issue := range report.Issues {
						fmt.Printf("  • %s\n", issue)
					}
				}

				if len(report.Warnings) > 0 {
					fmt.Printf("\n%s Warnings:\n", yellow("●"))
					for _, warning := range report.Warnings {
						fmt.Printf("  • %s\n", warning)
					}
				}

				if len(report.RequiredActions) > 0 {
					fmt.Printf("\n%s Required Actions:\n", yellow("●"))
					for _, action := range report.RequiredActions {
						fmt.Printf("  • %s\n", action)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")

	return cmd
}

// newConfigVersionUpdateCmd creates a new version
func newConfigVersionUpdateCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		description string
		changes     []string
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Create a new configuration version",
		Long: `Create a new version of the current configuration.

This creates a backup and updates the version history with the current state.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			vm := version.NewVersionManager(baseDir)

			if description == "" {
				description = "Manual version update"
			}

			newVersion, err := vm.UpdateVersion(description, changes)
			if err != nil {
				return fmt.Errorf("failed to create new version: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			fmt.Printf("%s Created version %s\n", green("✓"), newVersion.Version)
			fmt.Printf("  Description: %s\n", newVersion.Description)
			fmt.Printf("  Backup ID: %s\n", newVersion.BackupID)
			if len(newVersion.Changes) > 0 {
				fmt.Println("  Changes:")
				for _, change := range newVersion.Changes {
					fmt.Printf("    • %s\n", change)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "Version description")
	cmd.Flags().StringSliceVarP(&changes, "changes", "c", []string{}, "List of changes")

	return cmd
}

// newConfigUndoCmd creates the undo subcommand
func newConfigUndoCmd(settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "undo",
		Short: "Undo recent configuration changes",
		Long: `Rollback recent configuration changes using backup and version history.

The undo system tracks all configuration operations and allows you to safely
rollback changes using the backup and version control systems.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add undo subcommands
	cmd.AddCommand(newConfigUndoListCmd(settings))
	cmd.AddCommand(newConfigUndoLastCmd(settings))
	cmd.AddCommand(newConfigUndoOperationCmd(settings))
	cmd.AddCommand(newConfigUndoStatusCmd(settings))

	return cmd
}

// newConfigUndoListCmd lists undoable operations
func newConfigUndoListCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var (
		format string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent undoable operations",
		Long: `List recent operations that can be undone.

Shows operation details, timestamps, and potential risks of undoing each operation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			undoManager := undo.NewUndoManager(baseDir)

			if limit == 0 {
				limit = 10 // Default limit
			}

			operations, err := undoManager.GetUndoableOperations(limit)
			if err != nil {
				return fmt.Errorf("failed to get undoable operations: %w", err)
			}

			if len(operations) == 0 {
				fmt.Println("No operations available to undo")
				return nil
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(operations, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(operations)
				if err != nil {
					return fmt.Errorf("failed to marshal YAML: %w", err)
				}
				fmt.Println(string(data))
			default:
				green := color.New(color.FgGreen).SprintFunc()
				yellow := color.New(color.FgYellow).SprintFunc()
				red := color.New(color.FgRed).SprintFunc()
				gray := color.New(color.FgHiBlack).SprintFunc()

				fmt.Printf("Recent undoable operations (showing %d):\n\n", len(operations))
				for i, op := range operations {
					fmt.Printf("%s %s", green("●"), op.Operation)
					if !op.CanUndo {
						fmt.Printf(" %s", red("(cannot undo)"))
					}
					fmt.Println()

					fmt.Printf("    ID: %s\n", gray(op.ID))
					fmt.Printf("    Time: %s\n", op.Timestamp.Format("2006-01-02 15:04:05"))
					fmt.Printf("    Description: %s\n", op.Description)
					if op.Target != "" {
						fmt.Printf("    Target: %s\n", op.Target)
					}

					if len(op.UndoRisks) > 0 {
						fmt.Printf("    %s Risks: %s\n", yellow("⚠"), strings.Join(op.UndoRisks, ", "))
					}

					if i < len(operations)-1 {
						fmt.Println()
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")
	cmd.Flags().IntVarP(&limit, "limit", "l", 10, "Number of operations to show")

	return cmd
}

// newConfigUndoLastCmd undoes the most recent operation
func newConfigUndoLastCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "last",
		Short: "Undo the most recent operation",
		Long: `Undo the most recent configuration operation.

This is a quick way to rollback the last change you made.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			undoManager := undo.NewUndoManager(baseDir)

			// Check if there are operations to undo
			canUndo, err := undoManager.CanUndo()
			if err != nil {
				return fmt.Errorf("failed to check undo availability: %w", err)
			}

			if !canUndo {
				fmt.Println("No operations available to undo")
				return nil
			}

			// Get the last operation for confirmation
			operations, err := undoManager.GetUndoableOperations(1)
			if err != nil {
				return fmt.Errorf("failed to get operations: %w", err)
			}

			if len(operations) == 0 {
				fmt.Println("No operations available to undo")
				return nil
			}

			lastOp := operations[0]

			// Confirm unless forced
			if !force {
				fmt.Printf("Undo operation: %s\n", lastOp.Description)
				fmt.Printf("Target: %s\n", lastOp.Target)
				fmt.Printf("Time: %s\n", lastOp.Timestamp.Format("2006-01-02 15:04:05"))

				if len(lastOp.UndoRisks) > 0 {
					yellow := color.New(color.FgYellow).SprintFunc()
					fmt.Printf("\n%s Risks:\n", yellow("⚠"))
					for _, risk := range lastOp.UndoRisks {
						fmt.Printf("  • %s\n", risk)
					}
				}

				fmt.Printf("\nContinue? [y/N]: ")
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					response = "n" // Default to no on input error
				}
				if strings.ToLower(response) != "y" {
					fmt.Println("Undo cancelled")
					return nil
				}
			}

			// Perform the undo
			result, err := undoManager.UndoLast(force)
			if err != nil {
				return fmt.Errorf("failed to undo operation: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Printf("%s %s\n", green("✓"), result.Message)
			fmt.Printf("Restored from: %s\n", result.RestoredFrom)

			if result.NewBackupID != "" {
				fmt.Printf("Pre-undo backup: %s\n", result.NewBackupID)
			}

			if len(result.Warnings) > 0 {
				fmt.Printf("\n%s Warnings:\n", yellow("⚠"))
				for _, warning := range result.Warnings {
					fmt.Printf("  • %s\n", warning)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation and ignore risks")

	return cmd
}

// newConfigUndoOperationCmd undoes a specific operation
func newConfigUndoOperationCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "operation [operation-id]",
		Short: "Undo a specific operation",
		Long: `Undo a specific operation by its ID.

Use 'devex config undo list' to see available operation IDs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("operation ID required")
			}

			operationID := args[0]
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			undoManager := undo.NewUndoManager(baseDir)

			// Get operation details
			operation, err := undoManager.GetOperationDetails(operationID)
			if err != nil {
				return fmt.Errorf("failed to get operation details: %w", err)
			}

			// Confirm unless forced
			if !force {
				fmt.Printf("Undo operation: %s\n", operation.Description)
				fmt.Printf("Target: %s\n", operation.Target)
				fmt.Printf("Time: %s\n", operation.Timestamp.Format("2006-01-02 15:04:05"))

				if len(operation.UndoRisks) > 0 {
					yellow := color.New(color.FgYellow).SprintFunc()
					fmt.Printf("\n%s Risks:\n", yellow("⚠"))
					for _, risk := range operation.UndoRisks {
						fmt.Printf("  • %s\n", risk)
					}
				}

				fmt.Printf("\nContinue? [y/N]: ")
				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					response = "n" // Default to no on input error
				}
				if strings.ToLower(response) != "y" {
					fmt.Println("Undo cancelled")
					return nil
				}
			}

			// Perform the undo
			result, err := undoManager.UndoOperation(operationID, force)
			if err != nil {
				return fmt.Errorf("failed to undo operation: %w", err)
			}

			green := color.New(color.FgGreen).SprintFunc()
			yellow := color.New(color.FgYellow).SprintFunc()

			fmt.Printf("%s %s\n", green("✓"), result.Message)
			fmt.Printf("Restored from: %s\n", result.RestoredFrom)

			if result.NewBackupID != "" {
				fmt.Printf("Pre-undo backup: %s\n", result.NewBackupID)
			}

			if len(result.Warnings) > 0 {
				fmt.Printf("\n%s Warnings:\n", yellow("⚠"))
				for _, warning := range result.Warnings {
					fmt.Printf("  • %s\n", warning)
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation and ignore risks")

	return cmd
}

// newConfigUndoStatusCmd shows undo system status
func newConfigUndoStatusCmd(settings config.CrossPlatformSettings) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show undo system status",
		Long: `Display the current status of the undo system.

Shows available operations, recent activity, and system health.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			baseDir := filepath.Join(os.Getenv("HOME"), ".devex")
			undoManager := undo.NewUndoManager(baseDir)

			summary, err := undoManager.GetUndoSummary()
			if err != nil {
				return fmt.Errorf("failed to get undo summary: %w", err)
			}

			switch format {
			case "json":
				data, err := json.MarshalIndent(summary, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal JSON: %w", err)
				}
				fmt.Println(string(data))
			case "yaml":
				data, err := yaml.Marshal(summary)
				if err != nil {
					return fmt.Errorf("failed to marshal YAML: %w", err)
				}
				fmt.Println(string(data))
			default:
				green := color.New(color.FgGreen).SprintFunc()
				blue := color.New(color.FgBlue).SprintFunc()
				gray := color.New(color.FgHiBlack).SprintFunc()

				fmt.Printf("%s Undo System Status\n\n", blue("●"))

				if summary.CanUndo {
					fmt.Printf("Status: %s\n", green("Ready"))
				} else {
					fmt.Printf("Status: %s\n", gray("No operations to undo"))
				}

				fmt.Printf("Total operations: %d\n", summary.TotalOperations)
				fmt.Printf("Undoable operations: %d\n", summary.UndoableOperations)

				if summary.LastOperation != nil {
					fmt.Printf("Last operation: %s\n", *summary.LastOperation)
				}

				if summary.LastUndo != nil {
					fmt.Printf("Last undo: %s\n", *summary.LastUndo)
				}

				if len(summary.RecentOperations) > 0 {
					fmt.Println("\nRecent operations:")
					for _, op := range summary.RecentOperations {
						fmt.Printf("  • %s\n", op)
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format (table, json, yaml)")

	return cmd
}

// runConfigBackupWithProgress runs config backup with TUI progress tracking
func runConfigBackupWithProgress(settings config.CrossPlatformSettings, description string, tags []string, compress bool) error {
	// Create progress runner
	runner := tui.NewProgressRunner(context.Background(), settings)
	defer runner.Quit()

	// Start config backup operation with progress
	return runner.RunConfigOperation("backup", description, tags, compress)
}

// runConfigExportWithProgress runs config export with TUI progress tracking
func runConfigExportWithProgress(settings config.CrossPlatformSettings, format, output string, include, exclude []string, bundle, compress bool) error {
	// Create progress runner
	runner := tui.NewProgressRunner(context.Background(), settings)
	defer runner.Quit()

	// Start config export operation with progress
	return runner.RunConfigOperation("export", format, output, include, exclude, bundle, compress)
}
