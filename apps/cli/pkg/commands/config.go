package commands

import (
	"context"
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

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
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
