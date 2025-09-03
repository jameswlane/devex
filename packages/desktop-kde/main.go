package main

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

// KDEPlugin implements KDE Plasma desktop environment configuration
type KDEPlugin struct {
	*sdk.BasePlugin
}

// NewKDEPlugin creates a new KDE plugin
func NewKDEPlugin() *KDEPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-kde",
		Version:     version,
		Description: "KDE Plasma desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "kde", "plasma", "linux", "qt"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure KDE Plasma desktop settings",
				Usage:       "Apply DevEx KDE Plasma desktop configuration including themes, panels, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set KDE Plasma desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-panel",
				Description: "Configure KDE Plasma panel",
				Usage:       "Configure KDE Plasma panel appearance and behavior",
			},
			{
				Name:        "install-widgets",
				Description: "Install KDE Plasma widgets",
				Usage:       "Install and configure KDE Plasma desktop widgets",
			},
			{
				Name:        "apply-theme",
				Description: "Apply KDE themes",
				Usage:       "Apply Qt, KDE, and Plasma themes",
			},
			{
				Name:        "backup",
				Description: "Backup current KDE settings",
				Usage:       "Create a backup of current KDE configuration",
			},
			{
				Name:        "restore",
				Description: "Restore KDE settings from backup",
				Usage:       "Restore KDE configuration from a previous backup",
			},
		},
	}

	return &KDEPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *KDEPlugin) Execute(command string, args []string) error {
	// Check if KDE is available
	if !isKDEAvailable() {
		return fmt.Errorf("KDE Plasma desktop environment is not available on this system")
	}

	switch command {
	case "configure":
		return p.handleConfigure(args)
	case "set-background":
		return p.handleSetBackground(args)
	case "configure-panel":
		return p.handleConfigurePanel(args)
	case "install-widgets":
		return p.handleInstallWidgets(args)
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

// isKDEAvailable checks if KDE is available
func isKDEAvailable() bool {
	// Check if kwriteconfig5 is available
	if !sdk.CommandExists("kwriteconfig5") {
		return false
	}

	// Check if we're in a KDE session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "kde")
}

// handleConfigure applies comprehensive KDE configuration
func (p *KDEPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring KDE Plasma desktop environment...")

	// Apply default configurations using kwriteconfig5
	configs := []struct {
		file string
		group string
		key   string
		value string
	}{
		// Taskbar settings
		{"plasma-org.kde.plasma.desktop-appletsrc", "Containments][1][General", "showToolTips", "true"},
		// Window decorations
		{"kwinrc", "org.kde.kdecoration2", "theme", "Breeze"},
		// Enable compositing
		{"kwinrc", "Compositing", "Enabled", "true"},
		// Desktop effects
		{"kwinrc", "Plugins", "blurEnabled", "true"},
		// Panel auto-hide
		{"plasma-org.kde.plasma.desktop-appletsrc", "Containments][1][General", "visibility", "0"},
	}

	for _, config := range configs {
		if err := setKDESetting(config.file, config.group, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s.%s: %v\n", config.file, config.group, config.key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.group, config.key, config.value)
		}
	}

	// Restart plasmashell to apply changes
	fmt.Println("Restarting Plasma Shell to apply changes...")
	if err := restartPlasmaShell(); err != nil {
		fmt.Printf("Warning: Failed to restart Plasma Shell: %v\n", err)
	}

	fmt.Println("KDE Plasma configuration complete!")
	return nil
}

// handleSetBackground sets the desktop wallpaper
func (p *KDEPlugin) handleSetBackground(args []string) error {
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

	// Set wallpaper using qdbus
	if err := setKDEWallpaper(absPath); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// handleConfigurePanel configures the KDE panel
func (p *KDEPlugin) handleConfigurePanel(args []string) error {
	fmt.Println("Configuring KDE Plasma panel...")

	// Panel configurations
	panelConfigs := []struct {
		file  string
		group string
		key   string
		value string
	}{
		{"plasma-org.kde.plasma.desktop-appletsrc", "Containments][1][General", "formfactor", "2"},
		{"plasma-org.kde.plasma.desktop-appletsrc", "Containments][1][General", "immutability", "1"},
		{"plasma-org.kde.plasma.desktop-appletsrc", "Containments][1][General", "location", "4"},
		{"plasma-org.kde.plasma.desktop-appletsrc", "Containments][1][General", "plugin", "org.kde.panel"},
	}

	for _, config := range panelConfigs {
		if err := setKDESetting(config.file, config.group, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set panel setting %s: %v\n", config.key, err)
		} else {
			fmt.Printf("✓ Set panel %s to %s\n", config.key, config.value)
		}
	}

	fmt.Println("Panel configuration complete!")
	return nil
}

// handleInstallWidgets provides information about KDE widgets
func (p *KDEPlugin) handleInstallWidgets(args []string) error {
	fmt.Println("Installing KDE Plasma widgets...")

	// Recommended widgets
	widgets := []struct {
		name        string
		description string
		command     string
	}{
		{
			name:        "System Monitor",
			description: "CPU, memory, and network monitoring",
			command:     "org.kde.plasma.systemmonitor",
		},
		{
			name:        "Weather Widget",
			description: "Weather information display",
			command:     "org.kde.plasma.weather",
		},
		{
			name:        "Digital Clock",
			description: "Enhanced clock with date",
			command:     "org.kde.plasma.digitalclock",
		},
	}

	fmt.Println("\nRecommended widgets:")
	for i, widget := range widgets {
		fmt.Printf("%d. %s - %s\n", i+1, widget.name, widget.description)
	}

	fmt.Println("\nNote: Widgets can be added through:")
	fmt.Println("1. Right-click on desktop -> Add Widgets")
	fmt.Println("2. System Settings -> Workspace -> Desktop Behavior -> Desktop Effects")
	fmt.Println("3. Or install from KDE Store: https://store.kde.org/")

	return nil
}

// handleApplyTheme applies KDE themes
func (p *KDEPlugin) handleApplyTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying KDE theme: %s\n", themeName)

	// Apply various theme components
	themeConfigs := []struct {
		file  string
		group string
		key   string
		value string
	}{
		{"kdeglobals", "General", "ColorScheme", themeName},
		{"kdeglobals", "Icons", "Theme", themeName + "Icons"},
		{"kwinrc", "org.kde.kdecoration2", "theme", themeName},
		{"plasmarc", "Theme", "name", themeName},
	}

	for _, config := range themeConfigs {
		if err := setKDESetting(config.file, config.group, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set theme component %s: %v\n", config.key, err)
		} else {
			fmt.Printf("✓ Applied %s theme to %s\n", themeName, config.key)
		}
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	fmt.Println("You may need to log out and back in for all changes to take effect.")
	return nil
}

// handleBackup creates a backup of KDE settings
func (p *KDEPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "kde")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("kde-settings-%s.tar.gz", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Backup KDE configuration directory
	configDir := filepath.Join(os.Getenv("HOME"), ".config")
	cmd := exec.Command("tar", "-czf", backupFile, "-C", configDir, ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// handleRestore restores KDE settings from backup
func (p *KDEPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current KDE settings!")
	fmt.Print("Continue? [y/N]: ")

	var response string
	fmt.Scanln(&response)
	if strings.ToLower(response) != "y" {
		fmt.Println("Restore cancelled.")
		return nil
	}

	// Extract backup to config directory
	configDir := filepath.Join(os.Getenv("HOME"), ".config")
	cmd := exec.Command("tar", "-xzf", backupFile, "-C", configDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to log out and back in for all changes to take effect.")
	return nil
}

// setKDESetting sets a KDE setting using kwriteconfig5
func setKDESetting(file, group, key, value string) error {
	cmd := exec.Command("kwriteconfig5", "--file", file, "--group", group, "--key", key, value)
	return cmd.Run()
}

// setKDEWallpaper sets the wallpaper using qdbus
func setKDEWallpaper(wallpaperPath string) error {
	// Get the current desktop number
	cmd := exec.Command("qdbus", "org.kde.plasmashell", "/PlasmaShell", "org.kde.PlasmaShell.evaluateScript",
		fmt.Sprintf(`var allDesktops = desktops();
		for (i=0;i<allDesktops.length;i++) {
			d = allDesktops[i];
			d.wallpaperPlugin = "org.kde.image";
			d.currentConfigGroup = Array("Wallpaper", "org.kde.image", "General");
			d.writeConfig("Image", "file://%s");
		}`, wallpaperPath))
	return cmd.Run()
}

// restartPlasmaShell restarts the Plasma Shell
func restartPlasmaShell() error {
	// Kill plasmashell
	exec.Command("killall", "plasmashell").Run()
	
	// Wait a moment
	time.Sleep(2 * time.Second)
	
	// Start plasmashell
	cmd := exec.Command("plasmashell")
	cmd.Start()
	
	return nil
}

func main() {
	plugin := NewKDEPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
