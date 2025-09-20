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

// PantheonPlugin implements Pantheon desktop environment configuration
type PantheonPlugin struct {
	*sdk.BasePlugin
}

// NewPantheonPlugin creates a new Pantheon plugin
func NewPantheonPlugin() *PantheonPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-pantheon",
		Version:     version,
		Description: "Pantheon desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "pantheon", "linux", "elementary"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure Pantheon desktop settings",
				Usage:       "Apply DevEx Pantheon desktop configuration including themes, dock, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set Pantheon desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-dock",
				Description: "Configure Pantheon dock (Plank)",
				Usage:       "Configure Pantheon dock appearance and behavior",
			},
			{
				Name:        "configure-wingpanel",
				Description: "Configure Pantheon top panel (Wingpanel)",
				Usage:       "Configure Pantheon top panel settings",
			},
			{
				Name:        "apply-theme",
				Description: "Apply Pantheon themes",
				Usage:       "Apply GTK, icon, and Elementary themes",
			},
			{
				Name:        "backup",
				Description: "Backup current Pantheon settings",
				Usage:       "Create a backup of current Pantheon configuration",
			},
			{
				Name:        "restore",
				Description: "Restore Pantheon settings from backup",
				Usage:       "Restore Pantheon configuration from a previous backup",
			},
		},
	}

	return &PantheonPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *PantheonPlugin) Execute(command string, args []string) error {
	// Check if Pantheon is available
	if !isPantheonAvailable() {
		return fmt.Errorf("pantheon desktop environment is not available on this system")
	}

	switch command {
	case "configure":
		return p.handleConfigure(args)
	case "set-background":
		return p.handleSetBackground(args)
	case "configure-dock":
		return p.handleConfigureDock(args)
	case "configure-wingpanel":
		return p.handleConfigureWingpanel(args)
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

// isPantheonAvailable checks if Pantheon is available
func isPantheonAvailable() bool {
	// Check if gsettings is available
	if !sdk.CommandExists("gsettings") {
		return false
	}

	// Check if we're in a Pantheon session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	sessionType := os.Getenv("XDG_SESSION_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "pantheon") ||
		strings.Contains(strings.ToLower(sessionType), "pantheon") ||
		strings.Contains(strings.ToLower(desktop), "elementary")
}

// handleConfigure applies comprehensive Pantheon configuration
func (p *PantheonPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring Pantheon desktop environment...")

	// Apply default configurations
	configs := []struct {
		schema string
		key    string
		value  string
	}{
		// Interface settings
		{"org.gnome.desktop.interface", "gtk-theme", "elementary"},
		{"org.gnome.desktop.interface", "icon-theme", "elementary"},
		{"org.gnome.desktop.interface", "cursor-theme", "elementary"},
		{"org.gnome.desktop.interface", "font-name", "Open Sans 9"},
		{"org.gnome.desktop.interface", "document-font-name", "Open Sans 10"},
		{"org.gnome.desktop.interface", "monospace-font-name", "Roboto Mono 10"},
		// Desktop settings
		{"org.gnome.desktop.background", "show-desktop-icons", "false"},
		{"org.gnome.desktop.background", "picture-options", "zoom"},
		// Dock (Plank) settings
		{"net.launchpad.plank.dock.settings", "theme", "Gtk+"},
		{"net.launchpad.plank.dock.settings", "icon-size", "48"},
		{"net.launchpad.plank.dock.settings", "hide-mode", "intelligent"},
		{"net.launchpad.plank.dock.settings", "position", "bottom"},
		{"net.launchpad.plank.dock.settings", "alignment", "center"},
		// Window management
		{"org.gnome.desktop.wm.preferences", "button-layout", "close:maximize"},
		{"org.gnome.desktop.wm.preferences", "titlebar-font", "Open Sans Bold 9"},
		{"org.gnome.desktop.wm.preferences", "focus-mode", "click"},
		// Files (elementary Files) settings
		{"io.elementary.files.preferences", "default-viewmode", "list"},
		{"io.elementary.files.preferences", "sidebar-width", "200"},
		// Sound settings
		{"org.gnome.desktop.sound", "event-sounds", "true"},
		// Privacy settings
		{"org.gnome.desktop.privacy", "remember-recent-files", "true"},
		{"org.gnome.desktop.privacy", "recent-files-max-age", "30"},
		// Power settings
		{"org.gnome.desktop.session", "idle-delay", "300"},
	}

	for _, config := range configs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.schema, config.key, config.value)
		}
	}

	fmt.Println("Pantheon configuration complete!")
	return nil
}

// handleSetBackground sets the desktop wallpaper
func (p *PantheonPlugin) handleSetBackground(args []string) error {
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

	// Set wallpaper using GNOME schema (Pantheon uses GNOME backend)
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

// handleConfigureDock configures the Pantheon dock (Plank)
func (p *PantheonPlugin) handleConfigureDock(args []string) error {
	fmt.Println("Configuring Pantheon dock (Plank)...")

	// Plank dock configurations
	plankConfigs := []struct {
		schema string
		key    string
		value  string
	}{
		{"net.launchpad.plank.dock.settings", "theme", "Gtk+"},
		{"net.launchpad.plank.dock.settings", "icon-size", "48"},
		{"net.launchpad.plank.dock.settings", "hide-mode", "intelligent"},
		{"net.launchpad.plank.dock.settings", "position", "bottom"},
		{"net.launchpad.plank.dock.settings", "alignment", "center"},
		{"net.launchpad.plank.dock.settings", "items-alignment", "center"},
		{"net.launchpad.plank.dock.settings", "show-dock-item", "false"},
		{"net.launchpad.plank.dock.settings", "lock-items", "false"},
		{"net.launchpad.plank.dock.settings", "pinned-only", "false"},
	}

	hasErrors := false
	for _, config := range plankConfigs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			hasErrors = true
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.key, config.value)
		}
	}

	if hasErrors {
		fmt.Println("Note: Some dock settings failed. Plank may not be installed or running.")
		fmt.Println("Install Plank with: sudo apt install plank")
	}

	fmt.Println("Dock configuration complete!")
	return nil
}

// handleConfigureWingpanel configures the Pantheon top panel
func (p *PantheonPlugin) handleConfigureWingpanel(args []string) error {
	fmt.Println("Configuring Pantheon top panel (Wingpanel)...")

	// Wingpanel configurations
	wingpanelConfigs := []struct {
		schema string
		key    string
		value  string
	}{
		{"io.elementary.desktop.wingpanel", "use-transparency", "true"},
		{"io.elementary.desktop.wingpanel.datetime", "format", "%A, %B %e %H:%M"},
		{"io.elementary.desktop.wingpanel.datetime", "clock-show-seconds", "false"},
		{"io.elementary.desktop.wingpanel.datetime", "clock-show-weekday", "true"},
	}

	hasErrors := false
	for _, config := range wingpanelConfigs {
		if err := setGSetting(config.schema, config.key, config.value); err != nil {
			hasErrors = true
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.key, config.value)
		}
	}

	if hasErrors {
		fmt.Println("Note: Some wingpanel settings failed. This is normal on non-elementary systems.")
	}

	fmt.Println("Wingpanel configuration complete!")
	return nil
}

// handleApplyTheme applies Pantheon themes
func (p *PantheonPlugin) handleApplyTheme(args []string) error {
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

	// Apply elementary-specific themes if available
	if strings.Contains(strings.ToLower(themeName), "elementary") {
		if err := setGSetting("org.gnome.desktop.interface", "cursor-theme", "elementary"); err != nil {
			fmt.Printf("Note: Failed to set cursor theme.\n")
		}
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	return nil
}

// handleBackup creates a backup of Pantheon settings
func (p *PantheonPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "pantheon")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("pantheon-settings-%s.conf", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Use dconf to dump Pantheon and related settings
	schemas := []string{
		"/org/gnome/desktop/",
		"/io/elementary/",
		"/net/launchpad/plank/",
	}

	var allOutput strings.Builder
	allOutput.WriteString(fmt.Sprintf("# Pantheon Settings Backup - %s\n", time.Now().Format(time.RFC3339)))

	for _, schema := range schemas {
		cmd := exec.Command("dconf", "dump", schema)
		output, err := cmd.Output()
		if err != nil {
			fmt.Printf("Warning: Failed to dump %s: %v\n", schema, err)
			continue
		}

		allOutput.WriteString(fmt.Sprintf("\n# Schema: %s\n", schema))
		allOutput.Write(output)
	}

	// Write to file
	if err := os.WriteFile(backupFile, []byte(allOutput.String()), 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// handleRestore restores Pantheon settings from backup
func (p *PantheonPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current Pantheon settings!")
	fmt.Print("Continue? [y/N]: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}
	if strings.ToLower(response) != "y" {
		fmt.Println("Restore cancelled.")
		return nil
	}

	fmt.Println("Note: Pantheon backup restoration requires manual configuration.")
	fmt.Println("Please refer to the backup file for settings to restore manually.")
	fmt.Printf("Backup file location: %s\n", backupFile)
	fmt.Println("You can use 'dconf load /schema/path/ < backup-file' to restore specific schemas.")

	return nil
}

// setGSetting sets a Pantheon setting using gsettings
func setGSetting(schema, key, value string) error {
	cmd := exec.Command("gsettings", "set", schema, key, value)
	return cmd.Run()
}

func main() {
	plugin := NewPantheonPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
