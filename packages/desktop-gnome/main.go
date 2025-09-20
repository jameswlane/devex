package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// GNOMEPlugin implements GNOME desktop environment configuration
type GNOMEPlugin struct {
	*sdk.BasePlugin
	desktop    *DesktopManager
	extensions *ExtensionManager
	fonts      *FontManager
	themes     *ThemeManager
	backup     *BackupManager
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
				Name:        "install-fonts",
				Description: "Install and configure fonts",
				Usage:       "Install development fonts and configure GNOME font settings",
			},
			{
				Name:        "configure-fonts",
				Description: "Configure font settings",
				Usage:       "Set system, document, and monospace fonts for GNOME",
			},
			{
				Name:        "list-themes",
				Description: "List available themes",
				Usage:       "List installed GTK, Shell, and icon themes",
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
			{
				Name:        "list-backups",
				Description: "List available backups",
				Usage:       "List all available GNOME configuration backups",
			},
		},
	}

	return &GNOMEPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
		desktop:    NewDesktopManager(),
		extensions: NewExtensionManager(),
		fonts:      NewFontManager(),
		themes:     NewThemeManager(),
		backup:     NewBackupManager(),
	}
}

// Execute handles command execution
func (p *GNOMEPlugin) Execute(command string, args []string) error {
	// Check if GNOME is available
	if !isGNOMEAvailable() {
		return fmt.Errorf("GNOME desktop environment is not available on this system")
	}

	ctx := context.Background()

	switch command {
	case "configure":
		return p.desktop.Configure(ctx, args)
	case "set-background":
		return p.desktop.SetBackground(ctx, args)
	case "configure-dock":
		return p.desktop.ConfigureDock(ctx, args)
	case "install-extensions":
		return p.extensions.InstallExtensions(ctx, args)
	case "apply-theme":
		return p.themes.ApplyTheme(ctx, args)
	case "install-fonts":
		return p.fonts.InstallFonts(ctx, args)
	case "configure-fonts":
		return p.fonts.ConfigureFonts(ctx, args)
	case "list-themes":
		return p.themes.ListThemes(ctx, args)
	case "backup":
		return p.backup.CreateBackup(ctx, args)
	case "restore":
		return p.backup.RestoreBackup(ctx, args)
	case "list-backups":
		return p.backup.ListBackups(ctx, args)
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

// setGSettingWithContext sets a GNOME setting using gsettings with context support
func setGSettingWithContext(ctx context.Context, schema, key, value string) error {
	cmd := exec.CommandContext(ctx, "gsettings", "set", schema, key, value)
	return cmd.Run()
}

func main() {
	plugin := NewGNOMEPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
