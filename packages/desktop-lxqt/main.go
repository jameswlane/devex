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

// LXQtPlugin implements LXQt desktop environment configuration
type LXQtPlugin struct {
	*sdk.BasePlugin
}

// NewLXQtPlugin creates a new LXQt plugin
func NewLXQtPlugin() *LXQtPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-lxqt",
		Version:     version,
		Description: "LXQt desktop environment configuration for DevEx",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "lxqt", "linux", "qt"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure LXQt desktop settings",
				Usage:       "Apply DevEx LXQt desktop configuration including themes, panels, and settings",
			},
			{
				Name:        "set-background",
				Description: "Set desktop wallpaper",
				Usage:       "Set LXQt desktop wallpaper from a file path or URL",
			},
			{
				Name:        "configure-panel",
				Description: "Configure LXQt panel",
				Usage:       "Configure LXQt panel appearance and behavior",
			},
			{
				Name:        "configure-openbox",
				Description: "Configure Openbox window manager",
				Usage:       "Configure Openbox window manager settings",
			},
			{
				Name:        "apply-theme",
				Description: "Apply LXQt themes",
				Usage:       "Apply Qt, icon, and Openbox themes",
			},
			{
				Name:        "backup",
				Description: "Backup current LXQt settings",
				Usage:       "Create a backup of current LXQt configuration",
			},
			{
				Name:        "restore",
				Description: "Restore LXQt settings from backup",
				Usage:       "Restore LXQt configuration from a previous backup",
			},
		},
	}

	return &LXQtPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *LXQtPlugin) Execute(command string, args []string) error {
	// Check if LXQt is available
	if !isLXQtAvailable() {
		return fmt.Errorf("LXQt desktop environment is not available on this system")
	}

	switch command {
	case "configure":
		return p.handleConfigure(args)
	case "set-background":
		return p.handleSetBackground(args)
	case "configure-panel":
		return p.handleConfigurePanel(args)
	case "configure-openbox":
		return p.handleConfigureOpenbox(args)
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

// isLXQtAvailable checks if LXQt is available
func isLXQtAvailable() bool {
	// Check if lxqt-config is available
	if !sdk.CommandExists("lxqt-config") && !sdk.CommandExists("pcmanfm-qt") {
		return false
	}

	// Check if we're in an LXQt session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	sessionType := os.Getenv("XDG_SESSION_DESKTOP")
	return strings.Contains(strings.ToLower(desktop), "lxqt") ||
		strings.Contains(strings.ToLower(sessionType), "lxqt")
}

// handleConfigure applies comprehensive LXQt configuration
func (p *LXQtPlugin) handleConfigure(args []string) error {
	fmt.Println("Configuring LXQt desktop environment...")

	// Create config directories if they don't exist
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "lxqt")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Basic LXQt configuration
	fmt.Println("✓ Created LXQt configuration directory")

	// Configure panel
	if err := p.configureLXQtPanel(); err != nil {
		fmt.Printf("Warning: Failed to configure panel: %v\n", err)
	} else {
		fmt.Println("✓ Configured LXQt panel")
	}

	// Configure desktop settings
	if err := p.configureLXQtDesktop(); err != nil {
		fmt.Printf("Warning: Failed to configure desktop: %v\n", err)
	} else {
		fmt.Println("✓ Configured LXQt desktop")
	}

	// Configure session settings
	if err := p.configureLXQtSession(); err != nil {
		fmt.Printf("Warning: Failed to configure session: %v\n", err)
	} else {
		fmt.Println("✓ Configured LXQt session")
	}

	fmt.Println("LXQt configuration complete!")
	return nil
}

// configureLXQtPanel creates basic panel configuration
func (p *LXQtPlugin) configureLXQtPanel() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "lxqt")
	panelConfig := filepath.Join(configDir, "panel.conf")

	panelSettings := `[General]
iconTheme=
panels=panel1

[panel1]
alignment=-1
animation-duration=0
background-color=@Variant(\0\0\0\x43\0\xff\xff\0\0\0\0\0\0\0\0)
background-image=
desktop=0
font-color=@Variant(\0\0\0\x43\0\xff\xff\0\0\0\0\0\0\0\0)
hidable=false
hide-on-overlap=false
iconSize=22
lineCount=1
lockPanel=false
opacity=100
panelSize=32
plugins=mainmenu, taskbar, tray, clock
position=Bottom
reserve-space=true
show-delay=0
visible-margin=true
width=100
width-percent=true

[mainmenu]
type=mainmenu

[taskbar]
type=taskbar

[tray]
type=tray

[clock]
type=clock
`

	return os.WriteFile(panelConfig, []byte(panelSettings), 0644)
}

// configureLXQtDesktop creates basic desktop configuration
func (p *LXQtPlugin) configureLXQtDesktop() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "lxqt")
	desktopConfig := filepath.Join(configDir, "desktop.conf")

	desktopSettings := `[General]
desktopFont=@Variant(\0\0\0@\0\0\0\x12\0S\0\x61\0n\0s\0 \0S\0\x65\0r\0i\0\x66@\x1c\0\0\0\0\0\0\xff\xff\xff\xff\x5\x1\0\x32\x10)
wallpaper=/usr/share/pixmaps/lxqt-logo.png
wallpaperMode=stretch

[Network]
ProxyType=0
`

	return os.WriteFile(desktopConfig, []byte(desktopSettings), 0644)
}

// configureLXQtSession creates basic session configuration
func (p *LXQtPlugin) configureLXQtSession() error {
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "lxqt")
	sessionConfig := filepath.Join(configDir, "session.conf")

	sessionSettings := `[General]
__userfile__=true

[Environment]
GTK_CSD=0
QT_STYLE_OVERRIDE=

[Keyboard]
delay=500
interval=30

[Mouse]
accel_factor=20
accel_threshold=10
left_handed=false

[Qt]
doubleClickInterval=400
wheelScrollLines=3
style=
`

	return os.WriteFile(sessionConfig, []byte(sessionSettings), 0644)
}

// handleSetBackground sets the desktop wallpaper
func (p *LXQtPlugin) handleSetBackground(args []string) error {
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

	// Set wallpaper using pcmanfm-qt if available
	if sdk.CommandExists("pcmanfm-qt") {
		cmd := exec.Command("pcmanfm-qt", "--set-wallpaper", absPath)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set wallpaper: %w", err)
		}
	} else {
		// Update desktop.conf manually
		configDir := filepath.Join(os.Getenv("HOME"), ".config", "lxqt")
		desktopConfig := filepath.Join(configDir, "desktop.conf")

		// Read current config
		content := fmt.Sprintf(`[General]
wallpaper=%s
wallpaperMode=stretch
`, absPath)

		if err := os.WriteFile(desktopConfig, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to update desktop config: %w", err)
		}
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// handleConfigurePanel configures the LXQt panel
func (p *LXQtPlugin) handleConfigurePanel(args []string) error {
	fmt.Println("Configuring LXQt panel...")

	if err := p.configureLXQtPanel(); err != nil {
		return fmt.Errorf("failed to configure panel: %w", err)
	}

	fmt.Println("Panel configuration complete!")
	fmt.Println("Note: You may need to restart the LXQt panel for changes to take effect:")
	fmt.Println("  killall lxqt-panel && lxqt-panel &")
	return nil
}

// handleConfigureOpenbox configures the Openbox window manager
func (p *LXQtPlugin) handleConfigureOpenbox(args []string) error {
	fmt.Println("Configuring Openbox window manager...")

	// Create openbox config directory
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "openbox")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create openbox config directory: %w", err)
	}

	// Basic openbox configuration
	rcXML := filepath.Join(configDir, "lxqt-rc.xml")
	rcConfig := `<?xml version="1.0" encoding="UTF-8"?>
<openbox_config xmlns="http://openbox.org/3.4/rc" xmlns:xi="http://www.w3.org/2001/XInclude">
  <resistance>
    <strength>10</strength>
    <screen_edge_strength>20</screen_edge_strength>
  </resistance>
  <focus>
    <focusNew>yes</focusNew>
    <followMouse>no</followMouse>
    <focusLast>yes</focusLast>
    <underMouse>no</underMouse>
    <focusDelay>200</focusDelay>
    <raiseOnFocus>no</raiseOnFocus>
  </focus>
  <placement>
    <policy>Smart</policy>
    <center>yes</center>
    <monitor>Primary</monitor>
    <primaryMonitor>1</primaryMonitor>
  </placement>
  <theme>
    <name>Clearlooks</name>
    <titleLayout>NLIMC</titleLayout>
    <keepBorder>yes</keepBorder>
    <animateIconify>yes</animateIconify>
    <font place="ActiveWindow">
      <name>sans</name>
      <size>8</size>
      <weight>bold</weight>
      <slant>normal</slant>
    </font>
    <font place="InactiveWindow">
      <name>sans</name>
      <size>8</size>
      <weight>bold</weight>
      <slant>normal</slant>
    </font>
  </theme>
  <desktops>
    <number>4</number>
    <firstdesk>1</firstdesk>
    <names>
      <name>Desktop 1</name>
      <name>Desktop 2</name>
      <name>Desktop 3</name>
      <name>Desktop 4</name>
    </names>
    <popupTime>875</popupTime>
  </desktops>
</openbox_config>`

	if err := os.WriteFile(rcXML, []byte(rcConfig), 0644); err != nil {
		return fmt.Errorf("failed to write openbox config: %w", err)
	}

	fmt.Println("✓ Configured Openbox window manager")
	fmt.Println("Note: You may need to restart Openbox for changes to take effect:")
	fmt.Println("  openbox --reconfigure")
	return nil
}

// handleApplyTheme applies LXQt themes
func (p *LXQtPlugin) handleApplyTheme(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying theme: %s\n", themeName)

	// Update LXQt theme configuration
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "lxqt")
	sessionConfig := filepath.Join(configDir, "session.conf")

	sessionSettings := fmt.Sprintf(`[Qt]
style=%s

[General]
theme=%s
`, themeName, themeName)

	if err := os.WriteFile(sessionConfig, []byte(sessionSettings), 0644); err != nil {
		return fmt.Errorf("failed to apply theme: %w", err)
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	fmt.Println("Note: You may need to restart LXQt session for theme changes to take full effect.")
	return nil
}

// handleBackup creates a backup of LXQt settings
func (p *LXQtPlugin) handleBackup(args []string) error {
	backupDir := filepath.Join(os.Getenv("HOME"), ".devex", "backups", "lxqt")
	if len(args) > 0 {
		backupDir = args[0]
	}

	// Create backup directory
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	timestamp := strings.ReplaceAll(strings.ReplaceAll(strings.Split(time.Now().Format(time.RFC3339), "T")[0], ":", "-"), " ", "_")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("lxqt-settings-%s.tar.gz", timestamp))

	fmt.Printf("Creating backup at: %s\n", backupFile)

	// Create tar.gz backup of LXQt config directory
	cmd := exec.Command("tar", "-czf", backupFile, "-C", filepath.Join(os.Getenv("HOME"), ".config"), "lxqt", "openbox")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	fmt.Printf("✓ Backup created successfully: %s\n", backupFile)
	return nil
}

// handleRestore restores LXQt settings from backup
func (p *LXQtPlugin) handleRestore(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide path to backup file")
	}

	backupFile := args[0]

	// Check if file exists
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Printf("Restoring from backup: %s\n", backupFile)
	fmt.Println("WARNING: This will overwrite your current LXQt settings!")
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

	// Extract backup to config directory
	cmd := exec.Command("tar", "-xzf", backupFile, "-C", filepath.Join(os.Getenv("HOME"), ".config"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to restore settings: %w", err)
	}

	fmt.Println("✓ Settings restored successfully!")
	fmt.Println("You may need to restart LXQt session for all changes to take effect.")
	return nil
}

func main() {
	plugin := NewLXQtPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
