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

// MATEPlugin implements MATE desktop environment configuration
type MATEPlugin struct {
	*sdk.BasePlugin
}

// NewMATEPlugin creates a new MATE plugin
func NewMATEPlugin() *MATEPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-mate",
		Version:     version,
		Description: "MATE desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "mate", "linux", "gtk"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure MATE desktop settings",
				Usage:       "Apply DevEx MATE desktop configuration including themes, panels, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set MATE desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-panel",
				Description: "Configure MATE panel",
				Usage:       "Configure MATE panel appearance and behavior",
			},
			{
				Name:        "install-applets",
				Description: "Install MATE applets",
				Usage:       "Install and configure MATE desktop applets",
			},
			{
				Name:        "apply-theme",
				Description: "Apply MATE themes",
				Usage:       "Apply GTK, icon, and Marco themes",
			},
			{
				Name:        "backup",
				Description: "Backup current MATE settings",
				Usage:       "Create a backup of current MATE configuration",
			},
			{
				Name:        "restore",
				Description: "Restore MATE settings from backup",
				Usage:       "Restore MATE configuration from a previous backup",
			},
		},
	}

	return &MATEPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *MATEPlugin) Execute(command string, args []string) error {
	// Check if MATE is available
	if !isMATEAvailable() {
		return fmt.Errorf("MATE desktop environment is not available on this system")
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

// isMATEAvailable checks if MATE is available
func isMATEAvailable() bool {
	// Check if gsettings is available
	if !sdk.CommandExists("gsettings") {
		return false
	}

	// Check if we're in a MATE session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	sessionType := os.Getenv("XDG_SESSION_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "mate") ||
		strings.Contains(strings.ToLower(sessionType), "mate")
}

// handleConfigure applies comprehensive MATE configuration
func (p *MATEPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring MATE desktop environment...")

	// Apply default configurations
	configs := []struct {
		schema string
		key    string
		value  string
	}{
		// Panel settings
		{"org.mate.panel", "default-layout", "default"},
		{"org.mate.panel.general", "tooltips-enabled", "true"},
		{"org.mate.panel.general", "enable-animations", "true"},
		// Desktop settings
		{"org.mate.interface", "gtk-theme", "TraditionalOk"},
		{"org.mate.interface", "icon-theme", "mate"},
		{"org.mate.interface", "font-name", "Sans 10"},
		// Window management
		{"org.mate.Marco.general", "theme", "TraditionalOk"},
		{"org.mate.Marco.general", "button-layout", "menu:minimize,maximize,close"},
		{"org.mate.Marco.general", "focus-mode", "click"},
		{"org.mate.Marco.general", "auto-raise", "false"},
		// File manager settings
		{"org.mate.caja.preferences", "default-folder-viewer", "list-view"},
		{"org.mate.caja.preferences", "show-hidden-files", "false"},
		// Sound settings
		{"org.mate.sound", "event-sounds", "false"},
		// Session settings
		{"org.mate.session", "logout-prompt", "true"},
		{"org.mate.session", "auto-save-session", "false"},
		// Background settings
		{"org.mate.background", "picture-options", "zoom"},
		{"org.mate.background", "color-shading-type", "solid"},
		{"org.mate.background", "primary-color", "#58589191"},
	}

	for _, config := range configs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.schema, config.key, config.value)
		}
	}

	fmt.Println("MATE configuration complete!")
	return nil
}

// handleSetBackground sets the desktop wallpaper
func (p *MATEPlugin) handleSetBackground(args []string) error {
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

	// Set wallpaper using MATE schema
	if err := setGSetting("org.mate.background", "picture-filename", absPath); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	// Set picture options
	if err := setGSetting("org.mate.background", "picture-options", "zoom"); err != nil {
		fmt.Printf("Warning: Failed to set wallpaper options: %v\n", err)
	}

	// Enable wallpaper
	if err := setGSetting("org.mate.background", "draw-background", "true"); err != nil {
		fmt.Printf("Warning: Failed to enable wallpaper: %v\n", err)
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// handleConfigurePanel configures the MATE panel
func (p *MATEPlugin) handleConfigurePanel(args []string) error {
	fmt.Println("Configuring MATE panel...")

	// Panel configurations
	panelConfigs := []struct {
		schema string
		key    string
		value  string
	}{
		{"org.mate.panel.general", "tooltips-enabled", "true"},
		{"org.mate.panel.general", "enable-animations", "true"},
		{"org.mate.panel.general", "show-program-list", "false"},
		{"org.mate.panel.general", "confirm-panel-removal", "true"},
	}

	for _, config := range panelConfigs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.key, config.value)
		}
	}

	// Configure default panel layout
	if err := setGSetting("org.mate.panel", "default-layout", "default"); err != nil {
		fmt.Printf("Warning: Failed to set default panel layout: %v\n", err)
	} else {
		fmt.Println("✓ Set default panel layout")
	}

	fmt.Println("Panel configuration complete!")
	return nil
}

// handleInstallApplets provides information about installing MATE applets
func (p *MATEPlugin) handleInstallApplets(args []string) error {
	fmt.Println("Installing MATE applets...")

	// Check if mate-panel is available
	if !sdk.CommandExists("mate-panel") {
		return fmt.Errorf("mate-panel command not found")
	}

	// Default recommended applets
	applets := []struct {
		name        string
		description string
	}{
		{
			name:        "Clock",
			description: "Date and time display",
		},
		{
			name:        "Window List",
			description: "Show running applications",
		},
		{
			name:        "Notification Area",
			description: "System tray for applications",
		},
		{
			name:        "Volume Control",
			description: "Audio volume control",
		},
		{
			name:        "Network Manager",
			description: "Network connection manager",
		},
		{
			name:        "Weather Report",
			description: "Weather information display",
		},
	}

	fmt.Println("\nRecommended applets:")
	for i, applet := range applets {
		fmt.Printf("%d. %s - %s\n", i+1, applet.name, applet.description)
	}

	fmt.Println("\nNote: Applets can be managed through:")
	fmt.Println("1. Right-click panel > Add to Panel")
	fmt.Println("2. MATE Control Center > Panels")
	fmt.Println("3. mate-panel-preferences command")

	return nil
}

// handleApplyTheme applies MATE themes
func (p *MATEPlugin) handleApplyTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying theme: %s\n", themeName)

	// Apply GTK theme
	if err := setGSetting("org.mate.interface", "gtk-theme", themeName); err != nil {
		return fmt.Errorf("failed to set GTK theme: %w", err)
	}

	// Apply icon theme (if it's an icon theme)
	if strings.Contains(strings.ToLower(themeName), "icon") {
		if err := setGSetting("org.mate.interface", "icon-theme", themeName); err != nil {
			fmt.Printf("Warning: Failed to set icon theme: %v\n", err)
		}
	}

	// Apply window manager theme (Marco)
	if err := setGSetting("org.mate.Marco.general", "theme", themeName); err != nil {
		fmt.Printf("Note: Failed to set Marco theme. Theme may not be installed.\n")
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	return nil
}

// handleBackup creates a backup of MATE settings
func (p *MATEPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "mate")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("mate-settings-%s.conf", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Use dconf to dump MATE settings
	cmd := exec.Command("dconf", "dump", "/org/mate/")
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

// handleRestore restores MATE settings from backup
func (p *MATEPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current MATE settings!")
	fmt.Print("Continue? [y/N]: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return err
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
	cmd := exec.Command("dconf", "load", "/org/mate/")
	cmd.Stdin = strings.NewReader(string(data))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore dconf settings: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to restart MATE for all changes to take effect.")
	return nil
}

// setGSetting sets a MATE setting using gsettings
func setGSetting(schema, key, value string) error {
	cmd := exec.Command("gsettings", "set", schema, key, value)
	return cmd.Run()
}

func main() {
	plugin := NewMATEPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
