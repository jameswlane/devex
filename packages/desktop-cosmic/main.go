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

// CosmicPlugin implements COSMIC desktop environment configuration
type CosmicPlugin struct {
	*sdk.BasePlugin
}

// NewCosmicPlugin creates a new COSMIC plugin
func NewCosmicPlugin() *CosmicPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-cosmic",
		Version:     version,
		Description: "COSMIC desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "cosmic", "linux", "system76", "rust"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure COSMIC desktop settings",
				Usage:       "Apply DevEx COSMIC desktop configuration including themes, panels, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set COSMIC desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-panel",
				Description: "Configure COSMIC panel",
				Usage:       "Configure COSMIC panel appearance and behavior",
			},
			{
				Name:        "configure-comp",
				Description: "Configure COSMIC compositor",
				Usage:       "Configure COSMIC compositor and window management",
			},
			{
				Name:        "apply-theme",
				Description: "Apply COSMIC themes",
				Usage:       "Apply COSMIC themes and appearance settings",
			},
			{
				Name:        "backup",
				Description: "Backup current COSMIC settings",
				Usage:       "Create a backup of current COSMIC configuration",
			},
			{
				Name:        "restore",
				Description: "Restore COSMIC settings from backup",
				Usage:       "Restore COSMIC configuration from a previous backup",
			},
		},
	}

	return &CosmicPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *CosmicPlugin) Execute(command string, args []string) error {
	// Check if COSMIC is available
	if !isCosmicAvailable() {
		return fmt.Errorf("COSMIC desktop environment is not available on this system")
	}

	switch command {
	case "configure":
		return p.handleConfigure(args)
	case "set-background":
		return p.handleSetBackground(args)
	case "configure-panel":
		return p.handleConfigurePanel(args)
	case "configure-comp":
		return p.handleConfigureComp(args)
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

// isCosmicAvailable checks if COSMIC is available
func isCosmicAvailable() bool {
	// Check if cosmic-settings is available
	if !sdk.CommandExists("cosmic-settings") && !sdk.CommandExists("cosmic-panel") {
		return false
	}

	// Check if we're in a COSMIC session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	sessionType := os.Getenv("XDG_SESSION_DESKTOP")
	compositor := os.Getenv("XDG_SESSION_TYPE")

	return strings.Contains(strings.ToLower(desktop), "cosmic") ||
		strings.Contains(strings.ToLower(sessionType), "cosmic") ||
		(compositor == "wayland" && sdk.CommandExists("cosmic-comp"))
}

// handleConfigure applies comprehensive COSMIC configuration
func (p *CosmicPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring COSMIC desktop environment...")

	// Create COSMIC config directories if they don't exist
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cosmic")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create COSMIC config directory: %w", err)
	}

	// Basic COSMIC configuration
	fmt.Println("✓ Created COSMIC configuration directory")

	// Configure panel
	if err := p.configureCosmicPanel(); err != nil {
		fmt.Printf("Warning: Failed to configure panel: %v\n", err)
	} else {
		fmt.Println("✓ Configured COSMIC panel")
	}

	// Configure desktop settings
	if err := p.configureCosmicDesktop(); err != nil {
		fmt.Printf("Warning: Failed to configure desktop: %v\n", err)
	} else {
		fmt.Println("✓ Configured COSMIC desktop")
	}

	// Configure compositor settings
	if err := p.configureCosmicCompositor(); err != nil {
		fmt.Printf("Warning: Failed to configure compositor: %v\n", err)
	} else {
		fmt.Println("✓ Configured COSMIC compositor")
	}

	fmt.Println("COSMIC configuration complete!")
	fmt.Println("Note: COSMIC is still in alpha. Some features may be experimental.")
	return nil
}

// configureCosmicPanel creates basic panel configuration
func (p *CosmicPlugin) configureCosmicPanel() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cosmic")
	panelConfig := filepath.Join(configDir, "com.system76.CosmicPanel.ron")

	// Basic COSMIC panel configuration in RON format
	panelSettings := `(
    output: "default",
    layer: Top,
    anchor: (
        top: true,
        bottom: false,
        left: true,
        right: true,
    ),
    size: (
        width: 0,
        height: 32,
    ),
    margin: (
        top: 0,
        bottom: 0,
        left: 0,
        right: 0,
    ),
    applets: [
        (
            id: "cosmic-applet-launcher",
            config: {},
        ),
        (
            id: "cosmic-applet-workspaces",
            config: {},
        ),
        (
            id: "cosmic-applet-time",
            config: {},
        ),
        (
            id: "cosmic-applet-system-tray",
            config: {},
        ),
        (
            id: "cosmic-applet-network",
            config: {},
        ),
        (
            id: "cosmic-applet-battery",
            config: {},
        ),
        (
            id: "cosmic-applet-audio",
            config: {},
        ),
    ],
)`

	return os.WriteFile(panelConfig, []byte(panelSettings), 0644)
}

// configureCosmicDesktop creates basic desktop configuration
func (p *CosmicPlugin) configureCosmicDesktop() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cosmic")
	desktopConfig := filepath.Join(configDir, "com.system76.CosmicBackground.ron")

	// Basic COSMIC desktop configuration
	desktopSettings := `(
    wallpaper: Some("/usr/share/backgrounds/cosmic/default.png"),
    mode: Zoom,
    color_background: (0.0, 0.0, 0.0, 1.0),
)`

	return os.WriteFile(desktopConfig, []byte(desktopSettings), 0644)
}

// configureCosmicCompositor creates basic compositor configuration
func (p *CosmicPlugin) configureCosmicCompositor() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cosmic")
	compConfig := filepath.Join(configDir, "com.system76.CosmicComp.ron")

	// Basic COSMIC compositor configuration
	compSettings := `(
    border_radius: 12,
    gaps: (
        inner: 8,
        outer: 8,
    ),
    active_hint: true,
    focus_follows_cursor: false,
    cursor_follows_focus: false,
    workspaces: [
        "Workspace 1",
        "Workspace 2", 
        "Workspace 3",
        "Workspace 4",
    ],
)`

	return os.WriteFile(compConfig, []byte(compSettings), 0644)
}

// handleSetBackground sets the desktop wallpaper
func (p *CosmicPlugin) handleSetBackground(args []string) error {
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

	// Update COSMIC background configuration
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cosmic")
	desktopConfig := filepath.Join(configDir, "com.system76.CosmicBackground.ron")

	// Create new background config
	backgroundSettings := fmt.Sprintf(`(
    wallpaper: Some("%s"),
    mode: Zoom,
    color_background: (0.0, 0.0, 0.0, 1.0),
)`, absPath)

	if err := os.WriteFile(desktopConfig, []byte(backgroundSettings), 0644); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	fmt.Println("Note: You may need to restart COSMIC for changes to take effect.")
	return nil
}

// handleConfigurePanel configures the COSMIC panel
func (p *CosmicPlugin) handleConfigurePanel(args []string) error {
	fmt.Println("Configuring COSMIC panel...")

	if err := p.configureCosmicPanel(); err != nil {
		return fmt.Errorf("failed to configure panel: %w", err)
	}

	fmt.Println("Panel configuration complete!")
	fmt.Println("Note: You may need to restart cosmic-panel for changes to take effect:")
	fmt.Println("  systemctl --user restart cosmic-panel")
	return nil
}

// handleConfigureComp configures the COSMIC compositor
func (p *CosmicPlugin) handleConfigureComp(args []string) error {
	fmt.Println("Configuring COSMIC compositor...")

	if err := p.configureCosmicCompositor(); err != nil {
		return fmt.Errorf("failed to configure compositor: %w", err)
	}

	fmt.Println("Compositor configuration complete!")
	fmt.Println("Note: You may need to restart the compositor for changes to take effect:")
	fmt.Println("  systemctl --user restart cosmic-comp")
	return nil
}

// handleApplyTheme applies COSMIC themes
func (p *CosmicPlugin) handleApplyTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name (light/dark)")
	}

	themeName := strings.ToLower(args[0])
	fmt.Printf("Applying theme: %s\n", themeName)

	// COSMIC theme configuration
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "cosmic")
	themeConfig := filepath.Join(configDir, "com.system76.CosmicTheme.ron")

	var themeSettings string
	if themeName == "dark" {
		themeSettings = `(
    theme_mode: Dark,
    accent_color: Some((0.2, 0.6, 1.0, 1.0)),
    window_hint: Some((0.1, 0.1, 0.1, 0.9)),
)`
	} else {
		themeSettings = `(
    theme_mode: Light,
    accent_color: Some((0.2, 0.6, 1.0, 1.0)),
    window_hint: Some((1.0, 1.0, 1.0, 0.9)),
)`
	}

	if err := os.WriteFile(themeConfig, []byte(themeSettings), 0644); err != nil {
		return fmt.Errorf("failed to apply theme: %w", err)
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	fmt.Println("Note: You may need to restart COSMIC session for theme changes to take effect.")
	return nil
}

// handleBackup creates a backup of COSMIC settings
func (p *CosmicPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "cosmic")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("cosmic-settings-%s.tar.gz", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Create tar.gz backup of COSMIC config directory
	cmd := exec.Command("tar", "-czf", backupFile, "-C", filepath.Join(os.Getenv("HOME"), ".config"), "cosmic")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// handleRestore restores COSMIC settings from backup
func (p *CosmicPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current COSMIC settings!")
	fmt.Print("Continue? [y/N]: ")

	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}
	if strings.ToLower(response) != "y" {
		fmt.Println("Restore cancelled.")
		return nil
	}

	// Extract backup to config directory
	cmd := exec.Command("tar", "-xzf", backupFile, "-C", filepath.Join(os.Getenv("HOME"), ".config"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore settings: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to restart COSMIC session for all changes to take effect.")
	return nil
}

func main() {
	plugin := NewCosmicPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
