package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// KDEPlugin implements KDE Plasma desktop environment configuration
type KDEPlugin struct {
	*sdk.BasePlugin
	desktop *DesktopManager
	widgets *WidgetManager
	fonts   *FontManager
	themes  *ThemeManager
	backup  *BackupManager
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
				Name:        "list-widgets",
				Description: "List installed widgets",
				Usage:       "List all installed KDE Plasma widgets",
			},
			{
				Name:        "remove-widget",
				Description: "Remove a widget",
				Usage:       "Remove a specific KDE Plasma widget",
			},
			{
				Name:        "configure-widget",
				Description: "Configure widget settings",
				Usage:       "Get help on configuring KDE Plasma widgets",
			},
			{
				Name:        "apply-theme",
				Description: "Apply KDE themes",
				Usage:       "Apply Qt, KDE, and Plasma themes",
			},
			{
				Name:        "install-fonts",
				Description: "Install and configure fonts",
				Usage:       "Install development fonts and configure KDE font settings",
			},
			{
				Name:        "configure-fonts",
				Description: "Configure font settings",
				Usage:       "Set system and monospace fonts for KDE Plasma",
			},
			{
				Name:        "list-themes",
				Description: "List available themes",
				Usage:       "List installed Plasma, Qt, and color schemes",
			},
			{
				Name:        "restart-plasma",
				Description: "Restart Plasma Shell",
				Usage:       "Restart KDE Plasma Shell to apply changes",
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
			{
				Name:        "list-backups",
				Description: "List available backups",
				Usage:       "List all available KDE configuration backups",
			},
		},
	}

	return &KDEPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
		desktop:    NewDesktopManager(),
		widgets:    NewWidgetManager(),
		fonts:      NewFontManager(),
		themes:     NewThemeManager(),
		backup:     NewBackupManager(),
	}
}

// Execute handles command execution
func (p *KDEPlugin) Execute(command string, args []string) error {
	// Check if KDE is available
	if !isKDEAvailable() {
		return fmt.Errorf("KDE Plasma desktop environment is not available on this system")
	}

	ctx := context.Background()

	switch command {
	case "configure":
		return p.desktop.Configure(ctx, args)
	case "set-background":
		return p.desktop.SetBackground(ctx, args)
	case "configure-panel":
		return p.desktop.ConfigurePanel(ctx, args)
	case "restart-plasma":
		return p.desktop.RestartPlasma(ctx, args)
	case "install-widgets":
		return p.widgets.InstallWidgets(ctx, args)
	case "list-widgets":
		return p.widgets.ListWidgets(ctx, args)
	case "remove-widget":
		return p.widgets.RemoveWidget(ctx, args)
	case "configure-widget":
		return p.widgets.ConfigureWidget(ctx, args)
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

// isKDEAvailable checks if KDE is available
func isKDEAvailable() bool {
	// Check if KDE tools are available
	if !sdk.CommandExists("kwriteconfig5") {
		return false
	}

	// Check if we're in a KDE session
	desktop := os.Getenv("XDG_CURRENT_DESKTOP")
	session := os.Getenv("DESKTOP_SESSION")

	return strings.Contains(strings.ToLower(desktop), "kde") ||
		strings.Contains(strings.ToLower(session), "kde") ||
		strings.Contains(strings.ToLower(session), "plasma")
}

func main() {
	plugin := NewKDEPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
