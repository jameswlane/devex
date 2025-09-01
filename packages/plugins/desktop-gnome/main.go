package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
	"gopkg.in/yaml.v2"
)

var version = "dev" // Set by goreleaser

// GNOMEPlugin implements GNOME desktop environment configuration
type GNOMEPlugin struct {
	*sdk.BasePlugin
}

// NewGNOMEPlugin creates a new GNOME plugin
func NewGNOMEPlugin() *GNOMEPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-gnome",
		Version:     version,
		Description: "GNOME desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "gnome", "linux", "gtk"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure GNOME desktop settings",
				Usage:       "Apply DevEx GNOME desktop configuration including themes, extensions, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set GNOME desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-dock",
				Description: "Configure GNOME dock/dash",
				Usage:       "Configure GNOME dock appearance and behavior",
			},
			{
				Name:        "install-extensions",
				Description: "Install GNOME extensions",
				Usage:       "Install and configure GNOME Shell extensions",
			},
			{
				Name:        "apply-theme",
				Description: "Apply GTK and Shell themes",
				Usage:       "Apply GTK, icon, and GNOME Shell themes",
			},
			{
				Name:        "backup",
				Description: "Backup current GNOME settings",
				Usage:       "Create a backup of current GNOME configuration",
			},
			{
				Name:        "restore",
				Description: "Restore GNOME settings from backup",
				Usage:       "Restore GNOME configuration from a previous backup",
			},
		},
	}

	return &GNOMEPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *GNOMEPlugin) Execute(command string, args []string) error {
	// Check if GNOME is available
	if !isGNOMEAvailable() {
		return fmt.Errorf("GNOME desktop environment is not available on this system")
	}

	switch command {
	case "configure":
		return p.handleConfigure(args)
	case "set-background":
		return p.handleSetBackground(args)
	case "configure-dock":
		return p.handleConfigureDock(args)
	case "install-extensions":
		return p.handleInstallExtensions(args)
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

// isGNOMEAvailable checks if GNOME is available
func isGNOMEAvailable() bool {
	// Check if gsettings is available
	if !sdk.CommandExists("gsettings") {
		return false
	}

	// Check if we're in a GNOME session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "gnome")
}

// handleConfigure applies comprehensive GNOME configuration
func (p *GNOMEPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring GNOME desktop environment...")

	// Apply default configurations
	configs := []struct {
		schema string
		key    string
		value  string
	}{
		// Window buttons
		{"org.gnome.desktop.wm.preferences", "button-layout", "close,minimize,maximize:appmenu"},
		// Enable hot corners
		{"org.gnome.desktop.interface", "enable-hot-corners", "true"},
		// Show weekday in clock
		{"org.gnome.desktop.interface", "clock-show-weekday", "true"},
		// Enable natural scrolling
		{"org.gnome.desktop.peripherals.touchpad", "natural-scroll", "true"},
		// Tap to click
		{"org.gnome.desktop.peripherals.touchpad", "tap-to-click", "true"},
	}

	for _, config := range configs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.schema, config.key, config.value)
		}
	}

	fmt.Println("GNOME configuration complete!")
	return nil
}

// handleSetBackground sets the desktop wallpaper
func (p *GNOMEPlugin) handleSetBackground(args []string) error {
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

	// Set wallpaper
	uri := fmt.Sprintf("file://%s", absPath)
	if err := setGSetting("org.gnome.desktop.background", "picture-uri", uri); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	// Also set for dark mode
	if err := setGSetting("org.gnome.desktop.background", "picture-uri-dark", uri); err != nil {
		// Non-critical error
		fmt.Printf("Warning: Failed to set dark mode wallpaper: %v\n", err)
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// handleConfigureDock configures the GNOME dock
func (p *GNOMEPlugin) handleConfigureDock(args []string) error {
	fmt.Println("Configuring GNOME dock...")

	// Check if dash-to-dock extension is available
	dashToDockConfigs := []struct {
		schema string
		key    string
		value  string
	}{
		{"org.gnome.shell.extensions.dash-to-dock", "dock-position", "BOTTOM"},
		{"org.gnome.shell.extensions.dash-to-dock", "dash-max-icon-size", "48"},
		{"org.gnome.shell.extensions.dash-to-dock", "click-action", "minimize"},
		{"org.gnome.shell.extensions.dash-to-dock", "show-trash", "true"},
		{"org.gnome.shell.extensions.dash-to-dock", "show-mounts", "true"},
	}

	hasErrors := false
	for _, config := range dashToDockConfigs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			// Dash-to-dock might not be installed
			hasErrors = true
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.key, config.value)
		}
	}

	if hasErrors {
		fmt.Println("Note: Some dock settings failed. Dash-to-dock extension may not be installed.")
		fmt.Println("Run 'devex desktop-gnome install-extensions' to install recommended extensions.")
	}

	fmt.Println("Dock configuration complete!")
	return nil
}

// handleInstallExtensions installs GNOME extensions
func (p *GNOMEPlugin) handleInstallExtensions(args []string) error {
	fmt.Println("Installing GNOME extensions...")

	// Check if gnome-extensions tool is available
	if !sdk.CommandExists("gnome-extensions") {
		return fmt.Errorf("gnome-extensions tool not found. Please install gnome-shell-extensions package")
	}

	// Default recommended extensions
	extensions := []struct {
		uuid        string
		name        string
		description string
	}{
		{
			uuid:        "dash-to-dock@micxgx.gmail.com",
			name:        "Dash to Dock",
			description: "Transform the dash into a dock",
		},
		{
			uuid:        "appindicatorsupport@rgcjonas.gmail.com",
			name:        "AppIndicator Support",
			description: "Support for tray icons",
		},
		{
			uuid:        "blur-my-shell@aunetx",
			name:        "Blur my Shell",
			description: "Blur effect for GNOME Shell",
		},
	}

	fmt.Println("\nRecommended extensions:")
	for i, ext := range extensions {
		fmt.Printf("%d. %s - %s\n", i+1, ext.name, ext.description)
	}

	fmt.Println("\nNote: Extensions should be installed from https://extensions.gnome.org/")
	fmt.Println("Visit the website and install the GNOME Shell integration browser extension.")

	return nil
}

// handleApplyTheme applies GTK and Shell themes
func (p *GNOMEPlugin) handleApplyTheme(args []string) error {
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

	// Apply shell theme (requires user-theme extension)
	if err := setGSetting("org.gnome.shell.extensions.user-theme", "name", themeName); err != nil {
		fmt.Printf("Note: Failed to set shell theme. User Theme extension may not be installed.\n")
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	return nil
}

// handleBackup creates a backup of GNOME settings
func (p *GNOMEPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "gnome")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("gnome-settings-%s.conf", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Use dconf to dump settings
	cmd := exec.Command("dconf", "dump", "/")
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

// handleRestore restores GNOME settings from backup
func (p *GNOMEPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current GNOME settings!")
	fmt.Print("Continue? [y/N]: ")

	var response string
	fmt.Scanln(&response)
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
	cmd := exec.Command("dconf", "load", "/")
	cmd.Stdin = strings.NewReader(string(data))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore dconf settings: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to log out and back in for all changes to take effect.")
	return nil
}

// setGSetting sets a GNOME setting using gsettings
func setGSetting(schema, key, value string) error {
	cmd := exec.Command("gsettings", "set", schema, key, value)
	return cmd.Run()
}

// GnomeExtension represents a GNOME extension configuration
type GnomeExtension struct {
	ID          string       `yaml:"id"`
	SchemaFiles []SchemaFile `yaml:"schema_files"`
}

// SchemaFile represents a schema file to copy
type SchemaFile struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

func main() {
	plugin := NewGNOMEPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
