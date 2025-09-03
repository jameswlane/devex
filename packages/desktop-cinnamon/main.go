package main

// Build timestamp: 2025-09-03 17:41:19

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

// CinnamonPlugin implements Cinnamon desktop environment configuration
type CinnamonPlugin struct {
	*sdk.BasePlugin
}

// NewCinnamonPlugin creates a new Cinnamon plugin
func NewCinnamonPlugin() *CinnamonPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-cinnamon",
		Version:     version,
		Description: "Cinnamon desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "cinnamon", "linux", "mint"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure Cinnamon desktop settings",
				Usage:       "Apply DevEx Cinnamon desktop configuration including themes, applets, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set Cinnamon desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-panel",
				Description: "Configure Cinnamon panel",
				Usage:       "Configure Cinnamon panel appearance and behavior",
			},
			{
				Name:        "install-applets",
				Description: "Install Cinnamon applets",
				Usage:       "Install and configure Cinnamon desktop applets",
			},
			{
				Name:        "apply-theme",
				Description: "Apply Cinnamon themes",
				Usage:       "Apply GTK, icon, and Cinnamon themes",
			},
			{
				Name:        "backup",
				Description: "Backup current Cinnamon settings",
				Usage:       "Create a backup of current Cinnamon configuration",
			},
			{
				Name:        "restore",
				Description: "Restore Cinnamon settings from backup",
				Usage:       "Restore Cinnamon configuration from a previous backup",
			},
		},
	}

	return &CinnamonPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *CinnamonPlugin) Execute(command string, args []string) error {
	// Check if Cinnamon is available
	if !isCinnamonAvailable() {
		return fmt.Errorf("cinnamon desktop environment is not available on this system")
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

// isCinnamonAvailable checks if Cinnamon is available
func isCinnamonAvailable() bool {
	// Check if gsettings is available
	if !sdk.CommandExists("gsettings") {
		return false
	}

	// Check if we're in a Cinnamon session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	sessionType := os.Getenv("XDG_SESSION_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "cinnamon") ||
		strings.Contains(strings.ToLower(sessionType), "cinnamon")
}

// handleConfigure applies comprehensive Cinnamon configuration
func (p *CinnamonPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring Cinnamon desktop environment...")

	// Apply default configurations
	configs := []struct {
		schema string
		key    string
		value  string
	}{
		// Panel settings
		{"org.cinnamon", "panel-height", "32"},
		{"org.cinnamon", "panel-zone-text-sizes", "{'left': 0, 'center': 0, 'right': 0}"},
		// Desktop settings
		{"org.cinnamon.desktop.interface", "clock-show-seconds", "false"},
		{"org.cinnamon.desktop.interface", "clock-show-date", "true"},
		// Window management
		{"org.cinnamon.desktop.wm.preferences", "button-layout", "close,minimize,maximize:menu"},
		{"org.cinnamon.desktop.wm.preferences", "titlebar-font", "Sans Bold 10"},
		// Theme settings
		{"org.cinnamon.theme", "name", "cinnamon"},
		{"org.cinnamon.desktop.interface", "gtk-theme", "Mint-Y"},
		{"org.cinnamon.desktop.interface", "icon-theme", "Mint-Y"},
		// Sound settings
		{"org.cinnamon.desktop.sound", "event-sounds", "false"},
	}

	for _, config := range configs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.schema, config.key, config.value)
		}
	}

	fmt.Println("Cinnamon configuration complete!")
	return nil
}

// handleSetBackground sets the desktop wallpaper
func (p *CinnamonPlugin) handleSetBackground(args []string) error {
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

	// Set wallpaper using Cinnamon schema
	uri := fmt.Sprintf("file://%s", absPath)
	if err := setGSetting("org.cinnamon.desktop.background", "picture-uri", uri); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	// Set picture options
	if err := setGSetting("org.cinnamon.desktop.background", "picture-options", "zoom"); err != nil {
		fmt.Printf("Warning: Failed to set wallpaper options: %v\n", err)
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// handleConfigurePanel configures the Cinnamon panel
func (p *CinnamonPlugin) handleConfigurePanel(args []string) error {
	fmt.Println("Configuring Cinnamon panel...")

	// Panel configurations
	panelConfigs := []struct {
		schema string
		key    string
		value  string
	}{
		{"org.cinnamon", "panel-height", "32"},
		{"org.cinnamon", "panel-edit-mode", "false"},
		{"org.cinnamon", "panel-autohide", "false"},
		{"org.cinnamon", "panel-zone-icon-sizes", "{'left': 24, 'center': 24, 'right': 24}"},
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

// handleInstallApplets provides information about installing Cinnamon applets
func (p *CinnamonPlugin) handleInstallApplets(args []string) error {
	fmt.Println("Installing Cinnamon applets...")

	// Check if cinnamon is available
	if !sdk.CommandExists("cinnamon") {
		return fmt.Errorf("cinnamon command not found")
	}

	// Default recommended applets
	applets := []struct {
		name        string
		description string
	}{
		{
			name:        "weather@mockturtl",
			description: "Weather applet with forecasts",
		},
		{
			name:        "system-monitor@pixunil",
			description: "System resource monitor",
		},
		{
			name:        "calendar@cinnamon.org",
			description: "Calendar with agenda view",
		},
		{
			name:        "download-and-upload-speed@cardsurf",
			description: "Network speed monitor",
		},
	}

	fmt.Println("\nRecommended applets:")
	for i, applet := range applets {
		fmt.Printf("%d. %s - %s\n", i+1, applet.name, applet.description)
	}

	fmt.Println("\nNote: Applets can be installed through:")
	fmt.Println("1. System Settings > Applets > Download tab")
	fmt.Println("2. Cinnamon Spices website: https://cinnamon-spices.linuxmint.com/applets")
	fmt.Println("3. Right-click panel > Applets > Download")

	return nil
}

// handleApplyTheme applies Cinnamon themes
func (p *CinnamonPlugin) handleApplyTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying theme: %s\n", themeName)

	// Apply GTK theme
	if err := setGSetting("org.cinnamon.desktop.interface", "gtk-theme", themeName); err != nil {
		return fmt.Errorf("failed to set GTK theme: %w", err)
	}

	// Apply icon theme (if it's an icon theme)
	if strings.Contains(strings.ToLower(themeName), "icon") {
		if err := setGSetting("org.cinnamon.desktop.interface", "icon-theme", themeName); err != nil {
			fmt.Printf("Warning: Failed to set icon theme: %v\n", err)
		}
	}

	// Apply Cinnamon theme
	if err := setGSetting("org.cinnamon.theme", "name", themeName); err != nil {
		fmt.Printf("Note: Failed to set Cinnamon theme. Theme may not be installed.\n")
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	return nil
}

// handleBackup creates a backup of Cinnamon settings
func (p *CinnamonPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "cinnamon")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("cinnamon-settings-%s.conf", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Use dconf to dump Cinnamon settings
	cmd := exec.Command("dconf", "dump", "/org/cinnamon/")
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

// handleRestore restores Cinnamon settings from backup
func (p *CinnamonPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current Cinnamon settings!")
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
	cmd := exec.Command("dconf", "load", "/org/cinnamon/")
	cmd.Stdin = strings.NewReader(string(data))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore dconf settings: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to restart Cinnamon for all changes to take effect.")
	return nil
}

// setGSetting sets a Cinnamon setting using gsettings
func setGSetting(schema, key, value string) error {
	cmd := exec.Command("gsettings", "set", schema, key, value)
	return cmd.Run()
}

func main() {
	plugin := NewCinnamonPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
