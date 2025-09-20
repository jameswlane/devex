package main

// Build timestamp: 2025-09-03 17:52:00

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// BudgiePlugin implements Budgie desktop environment configuration
type BudgiePlugin struct {
	*sdk.BasePlugin
}

// NewBudgiePlugin creates a new Budgie plugin
func NewBudgiePlugin() *BudgiePlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-budgie",
		Version:     version,
		Description: "Budgie desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "budgie", "linux", "solus"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure Budgie desktop settings",
				Usage:       "Apply DevEx Budgie desktop configuration including themes, panel, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set Budgie desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-panel",
				Description: "Configure Budgie panel",
				Usage:       "Configure Budgie panel appearance and behavior",
			},
			{
				Name:        "install-applets",
				Description: "Install Budgie applets",
				Usage:       "Install and configure Budgie desktop applets",
			},
			{
				Name:        "apply-theme",
				Description: "Apply Budgie themes",
				Usage:       "Apply GTK, icon, and Budgie themes",
			},
			{
				Name:        "backup",
				Description: "Backup current Budgie settings",
				Usage:       "Create a backup of current Budgie configuration",
			},
			{
				Name:        "restore",
				Description: "Restore Budgie settings from backup",
				Usage:       "Restore Budgie configuration from a previous backup",
			},
		},
	}

	return &BudgiePlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *BudgiePlugin) Execute(command string, args []string) error {
	// Check if Budgie is available
	if !isBudgieAvailable() {
		return fmt.Errorf("budgie desktop environment is not available on this system")
	}

	switch command {
	case "configure":
		return p.handleConfigure(args)
	case "set-background":
		return p.handleSetBackground(args)
	case "configure-panel":
		return p.handleConfigurePanel(args)
	case "install-applets":
		return p.handleInstallApplets(args)
	case "apply-theme":
		return p.handleApplyTheme(args)
	case "backup":
		return p.handleBackup(args)
	case "restore":
		return p.handleRestore(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// isBudgieAvailable checks if Budgie is available
func isBudgieAvailable() bool {
	// Check if gsettings is available
	if !sdk.CommandExists("gsettings") {
		return false
	}

	// Check if we're in a Budgie session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	sessionType := os.Getenv("XDG_SESSION_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "budgie") ||
		strings.Contains(strings.ToLower(sessionType), "budgie")
}

// handleConfigure applies comprehensive Budgie configuration
func (p *BudgiePlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring Budgie desktop environment...")

	// Apply default configurations
	configs := []struct {
		schema string
		key    string
		value  string
	}{
		// Panel settings
		{"com.solus-project.budgie-panel", "dark-theme", "true"},
		{"com.solus-project.budgie-panel", "builtin-theme", "true"},
		// Desktop settings
		{"org.gnome.desktop.interface", "clock-show-seconds", "false"},
		{"org.gnome.desktop.interface", "clock-show-date", "true"},
		{"org.gnome.desktop.interface", "show-battery-percentage", "true"},
		// Window management
		{"org.gnome.desktop.wm.preferences", "button-layout", "appmenu:minimize,maximize,close"},
		{"org.gnome.desktop.wm.preferences", "titlebar-uses-system-font", "true"},
		// Theme settings
		{"org.gnome.desktop.interface", "gtk-theme", "Arc-Dark"},
		{"org.gnome.desktop.interface", "icon-theme", "Arc"},
		{"org.gnome.desktop.interface", "cursor-theme", "Adwaita"},
		// Sound settings
		{"org.gnome.desktop.sound", "event-sounds", "false"},
		// Raven settings (Budgie sidebar)
		{"com.solus-project.budgie-raven", "show-power-strip", "true"},
		{"com.solus-project.budgie-raven", "show-calendar-widget", "true"},
		{"com.solus-project.budgie-raven", "show-sound-output-widget", "true"},
	}

	for _, config := range configs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.schema, config.key, config.value)
		}
	}

	fmt.Println("Budgie configuration complete!")
	return nil
}

// handleSetBackground sets the desktop wallpaper
func (p *BudgiePlugin) handleSetBackground(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a path to the wallpaper image")
	}

	wallpaperPath := args[0]

	// Check if file exists
	if _, err := os.Stat(wallpaperPath); err != nil {
		return fmt.Errorf("wallpaper file not found: %s", wallpaperPath)
	}

	// Get absolute path
	absPath, err := filepath.Abs(wallpaperPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Set wallpaper using GNOME schema (Budgie uses GNOME backend)
	uri := fmt.Sprintf("file://%s", absPath)
	if err := setGSetting("org.gnome.desktop.background", "picture-uri", uri); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	// Also set for dark mode
	if err := setGSetting("org.gnome.desktop.background", "picture-uri-dark", uri); err != nil {
		fmt.Printf("Warning: Failed to set dark mode wallpaper: %v\n", err)
	}

	// Set picture options
	if err := setGSetting("org.gnome.desktop.background", "picture-options", "zoom"); err != nil {
		fmt.Printf("Warning: Failed to set wallpaper options: %v\n", err)
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// handleConfigurePanel configures the Budgie panel
func (p *BudgiePlugin) handleConfigurePanel(args []string) error {
	fmt.Println("Configuring Budgie panel...")

	// Panel configurations
	panelConfigs := []struct {
		schema string
		key    string
		value  string
	}{
		{"com.solus-project.budgie-panel", "dark-theme", "true"},
		{"com.solus-project.budgie-panel", "builtin-theme", "true"},
		{"com.solus-project.budgie-panel", "migration-level", "1"},
	}

	for _, config := range panelConfigs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.key, config.value)
		}
	}

	fmt.Println("Panel configuration complete!")
	return nil
}

// handleInstallApplets provides information about installing Budgie applets
func (p *BudgiePlugin) handleInstallApplets(args []string) error {
	fmt.Println("Installing Budgie applets...")

	// Check if budgie-panel is available
	if !sdk.CommandExists("budgie-panel") {
		return fmt.Errorf("budgie-panel command not found")
	}

	// Default recommended applets
	applets := []struct {
		name        string
		description string
	}{
		{
			name:        "WeatherShow",
			description: "Weather information applet",
		},
		{
			name:        "Workspace Switcher",
			description: "Switch between workspaces",
		},
		{
			name:        "Night Light",
			description: "Blue light filter control",
		},
		{
			name:        "Applications Menu",
			description: "Application launcher menu",
		},
		{
			name:        "System Tray",
			description: "System tray for applications",
		},
	}

	fmt.Println("\nRecommended applets:")
	for i, applet := range applets {
		fmt.Printf("%d. %s - %s\n", i+1, applet.name, applet.description)
	}

	fmt.Println("\nNote: Applets can be managed through:")
	fmt.Println("1. Right-click panel > Panel settings > Applets")
	fmt.Println("2. Budgie Desktop Settings")
	fmt.Println("3. Install additional applets from package manager")

	return nil
}

// handleApplyTheme applies Budgie themes
func (p *BudgiePlugin) handleApplyTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying theme: %s\n", themeName)

	// Apply GTK theme
	if err := setGSetting("org.gnome.desktop.interface", "gtk-theme", themeName); err != nil {
		return fmt.Errorf("failed to set GTK theme: %w", err)
	}

	// Apply icon theme (if it's an icon theme)
	if strings.Contains(strings.ToLower(themeName), "icon") {
		if err := setGSetting("org.gnome.desktop.interface", "icon-theme", themeName); err != nil {
			fmt.Printf("Warning: Failed to set icon theme: %v\n", err)
		}
	}

	// Apply Budgie panel theme
	if strings.Contains(strings.ToLower(themeName), "dark") {
		if err := setGSetting("com.solus-project.budgie-panel", "dark-theme", "true"); err != nil {
			fmt.Printf("Note: Failed to set Budgie panel theme.\n")
		}
	} else {
		if err := setGSetting("com.solus-project.budgie-panel", "dark-theme", "false"); err != nil {
			fmt.Printf("Note: Failed to set Budgie panel theme.\n")
		}
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	return nil
}

// handleBackup creates a backup of Budgie settings
func (p *BudgiePlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "budgie")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("budgie-settings-%s.conf", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Use dconf to dump Budgie settings
	cmd := exec.Command("dconf", "dump", "/com/solus-project/budgie-panel/")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to dump dconf settings: %w", err)
	}

	// Write to file
	if err := os.WriteFile(backupFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// handleRestore restores Budgie settings from backup
func (p *BudgiePlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current Budgie settings!")
	fmt.Print("Continue? [y/N]: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}
	if strings.ToLower(response) != "y" {
		fmt.Println("Restore cancelled.")
		return nil
	}

	// Read backup file
	data, err := os.ReadFile(backupFile)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Use dconf to load settings
	cmd := exec.Command("dconf", "load", "/com/solus-project/budgie-panel/")
	cmd.Stdin = strings.NewReader(string(data))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore dconf settings: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to restart Budgie for all changes to take effect.")
	return nil
}

// setGSetting sets a Budgie setting using gsettings
func setGSetting(schema, key, value string) error {
	cmd := exec.Command("gsettings", "set", schema, key, value)
	return cmd.Run()
}

func main() {
	plugin := NewBudgiePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
