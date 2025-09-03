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

// XFCEPlugin implements XFCE desktop environment configuration
type XFCEPlugin struct {
	*sdk.BasePlugin
}

// NewXFCEPlugin creates a new XFCE plugin
func NewXFCEPlugin() *XFCEPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-xfce",
		Version:     version,
		Description: "XFCE desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "xfce", "linux", "gtk"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure XFCE desktop settings",
				Usage:       "Apply DevEx XFCE desktop configuration including themes, panels, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set XFCE desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-panel",
				Description: "Configure XFCE panel",
				Usage:       "Configure XFCE panel appearance and behavior",
			},
			{
				Name:        "configure-wm",
				Description: "Configure window manager",
				Usage:       "Configure XFWM4 window manager settings",
			},
			{
				Name:        "apply-theme",
				Description: "Apply XFCE themes",
				Usage:       "Apply GTK, icon, and window manager themes",
			},
			{
				Name:        "backup",
				Description: "Backup current XFCE settings",
				Usage:       "Create a backup of current XFCE configuration",
			},
			{
				Name:        "restore",
				Description: "Restore XFCE settings from backup",
				Usage:       "Restore XFCE configuration from a previous backup",
			},
		},
	}

	return &XFCEPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *XFCEPlugin) Execute(command string, args []string) error {
	// Check if XFCE is available
	if !isXFCEAvailable() {
		return fmt.Errorf("XFCE desktop environment is not available on this system")
	}

	switch command {
	case "configure":
		return p.handleConfigure(args)
	case "set-background":
		return p.handleSetBackground(args)
	case "configure-panel":
		return p.handleConfigurePanel(args)
	case "configure-wm":
		return p.handleConfigureWM(args)
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

// isXFCEAvailable checks if XFCE is available
func isXFCEAvailable() bool {
	// Check if xfconf-query is available
	if !sdk.CommandExists("xfconf-query") {
		return false
	}

	// Check if we're in an XFCE session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	sessionType := os.Getenv("XDG_SESSION_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "xfce") ||
		strings.Contains(strings.ToLower(sessionType), "xfce")
}

// handleConfigure applies comprehensive XFCE configuration
func (p *XFCEPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring XFCE desktop environment...")

	// Apply default configurations
	configs := []struct {
		channel  string
		property string
		value    string
		dataType string
	}{
		// Desktop settings
		{"xfce4-desktop", "/backdrop/screen0/monitorVGA-1/workspace0/image-style", "5", "int"}, // Zoom
		{"xfce4-desktop", "/backdrop/screen0/monitorVGA-1/workspace0/color-style", "0", "int"}, // Solid color
		// Panel settings
		{"xfce4-panel", "/panels/panel-1/size", "32", "int"},
		{"xfce4-panel", "/panels/panel-1/length", "100", "int"},
		{"xfce4-panel", "/panels/panel-1/autohide-behavior", "0", "int"}, // Never
		// Window manager settings
		{"xfwm4", "/general/theme", "Default", "string"},
		{"xfwm4", "/general/button_layout", "O|SHMC", "string"},
		{"xfwm4", "/general/click_to_focus", "true", "bool"},
		{"xfwm4", "/general/focus_delay", "250", "int"},
		// Appearance settings
		{"xsettings", "/Net/ThemeName", "Adwaita", "string"},
		{"xsettings", "/Net/IconThemeName", "Adwaita", "string"},
		{"xsettings", "/Gtk/FontName", "Sans 10", "string"},
		// Keyboard settings
		{"keyboard-layout", "/Default/XkbDisable", "false", "bool"},
		{"keyboard-layout", "/Default/XkbLayout", "us", "string"},
	}

	for _, config := range configs {
		if err := setXFCESetting(config.channel, config.property, config.value, config.dataType); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.channel, config.property, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.channel, config.property, config.value)
		}
	}

	fmt.Println("XFCE configuration complete!")
	return nil
}

// handleSetBackground sets the desktop wallpaper
func (p *XFCEPlugin) handleSetBackground(args []string) error {
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

	// Set wallpaper for all workspaces and monitors (basic approach)
	properties := []string{
		"/backdrop/screen0/monitor0/workspace0/last-image",
		"/backdrop/screen0/monitorVGA-1/workspace0/last-image",
		"/backdrop/screen0/monitorHDMI-1/workspace0/last-image",
		"/backdrop/screen0/monitorDP-1/workspace0/last-image",
	}

	success := false
	for _, prop := range properties {
		if err := setXFCESetting("xfce4-desktop", prop, absPath, "string"); err == nil {
			success = true
			fmt.Printf("✓ Set wallpaper property: %s\n", prop)
		}
	}

	if !success {
		return fmt.Errorf("failed to set wallpaper on any monitor")
	}

	// Set image style to zoom
	_ = setXFCESetting("xfce4-desktop", "/backdrop/screen0/monitor0/workspace0/image-style", "5", "int")

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// handleConfigurePanel configures the XFCE panel
func (p *XFCEPlugin) handleConfigurePanel(args []string) error {
	fmt.Println("Configuring XFCE panel...")

	// Panel configurations
	panelConfigs := []struct {
		channel  string
		property string
		value    string
		dataType string
	}{
		{"xfce4-panel", "/panels/panel-1/position", "p=8;x=0;y=0", "string"}, // Bottom center
		{"xfce4-panel", "/panels/panel-1/size", "32", "int"},
		{"xfce4-panel", "/panels/panel-1/length", "100", "int"},
		{"xfce4-panel", "/panels/panel-1/length-adjust", "true", "bool"},
		{"xfce4-panel", "/panels/panel-1/span-monitors", "false", "bool"},
		{"xfce4-panel", "/panels/panel-1/autohide-behavior", "0", "int"}, // Never hide
		{"xfce4-panel", "/panels/panel-1/background-style", "0", "int"},  // None (system style)
	}

	for _, config := range panelConfigs {
		if err := setXFCESetting(config.channel, config.property, config.value, config.dataType); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.channel, config.property, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.property, config.value)
		}
	}

	fmt.Println("Panel configuration complete!")
	return nil
}

// handleConfigureWM configures the XFCE window manager
func (p *XFCEPlugin) handleConfigureWM(args []string) error {
	fmt.Println("Configuring XFCE window manager...")

	// Window manager configurations
	wmConfigs := []struct {
		channel  string
		property string
		value    string
		dataType string
	}{
		{"xfwm4", "/general/theme", "Default", "string"},
		{"xfwm4", "/general/button_layout", "O|SHMC", "string"},
		{"xfwm4", "/general/click_to_focus", "true", "bool"},
		{"xfwm4", "/general/focus_delay", "250", "int"},
		{"xfwm4", "/general/raise_delay", "250", "int"},
		{"xfwm4", "/general/double_click_time", "400", "int"},
		{"xfwm4", "/general/double_click_distance", "5", "int"},
		{"xfwm4", "/general/wrap_windows", "true", "bool"},
		{"xfwm4", "/general/wrap_workspaces", "false", "bool"},
		{"xfwm4", "/general/zoom_desktop", "true", "bool"},
	}

	for _, config := range wmConfigs {
		if err := setXFCESetting(config.channel, config.property, config.value, config.dataType); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.channel, config.property, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.property, config.value)
		}
	}

	fmt.Println("Window manager configuration complete!")
	return nil
}

// handleApplyTheme applies XFCE themes
func (p *XFCEPlugin) handleApplyTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying theme: %s\n", themeName)

	// Apply GTK theme
	if err := setXFCESetting("xsettings", "/Net/ThemeName", themeName, "string"); err != nil {
		return fmt.Errorf("failed to set GTK theme: %w", err)
	}

	// Apply icon theme (if it's an icon theme)
	if strings.Contains(strings.ToLower(themeName), "icon") {
		if err := setXFCESetting("xsettings", "/Net/IconThemeName", themeName, "string"); err != nil {
			fmt.Printf("Warning: Failed to set icon theme: %v\n", err)
		}
	}

	// Apply window manager theme
	if err := setXFCESetting("xfwm4", "/general/theme", themeName, "string"); err != nil {
		fmt.Printf("Note: Failed to set window manager theme. Theme may not be installed.\n")
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	return nil
}

// handleBackup creates a backup of XFCE settings
func (p *XFCEPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "xfce")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("xfce-settings-%s.xml", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Use xfconf-query to list all channels and dump their settings
	cmd := exec.Command("xfconf-query", "-l")
	channels, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list xfconf channels: %w", err)
	}

	backupContent := fmt.Sprintf("<!-- XFCE Settings Backup - %s -->\n<xfce-config>\n", time.Now().Format(time.RFC3339))

	// Iterate through each channel
	for _, channel := range strings.Split(strings.TrimSpace(string(channels)), "\n") {
		if channel == "" {
			continue
		}

		// Get all properties for this channel
		cmd := exec.Command("xfconf-query", "-c", channel, "-l")
		props, err := cmd.Output()
		if err != nil {
			continue
		}

		backupContent += fmt.Sprintf("  <channel name=\"%s\">\n", channel)

		for _, prop := range strings.Split(strings.TrimSpace(string(props)), "\n") {
			if prop == "" {
				continue
			}

			// Get the value for this property
			cmd := exec.Command("xfconf-query", "-c", channel, "-p", prop)
			value, err := cmd.Output()
			if err != nil {
				continue
			}

			backupContent += fmt.Sprintf("    <property name=\"%s\" value=\"%s\"/>\n", prop, strings.TrimSpace(string(value)))
		}

		backupContent += "  </channel>\n"
	}

	backupContent += "</xfce-config>\n"

	// Write to file
	if err := os.WriteFile(backupFile, []byte(backupContent), 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// handleRestore restores XFCE settings from backup
func (p *XFCEPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current XFCE settings!")
	fmt.Print("Continue? [y/N]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Restore cancelled.")
		return nil
	}

	fmt.Println("Note: XFCE backup restoration requires manual configuration.")
	fmt.Println("Please refer to the backup file for settings to restore manually.")
	fmt.Printf("Backup file location: %s\n", backupFile)

	return nil
}

// setXFCESetting sets an XFCE setting using xfconf-query
func setXFCESetting(channel, property, value, dataType string) error {
	var cmd *exec.Cmd
	switch dataType {
	case "string":
		cmd = exec.Command("xfconf-query", "-c", channel, "-p", property, "-s", value)
	case "int":
		cmd = exec.Command("xfconf-query", "-c", channel, "-p", property, "-t", "int", "-s", value)
	case "bool":
		cmd = exec.Command("xfconf-query", "-c", channel, "-p", property, "-t", "bool", "-s", value)
	default:
		cmd = exec.Command("xfconf-query", "-c", channel, "-p", property, "-s", value)
	}
	return cmd.Run()
}

func main() {
	plugin := NewXFCEPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
