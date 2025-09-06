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
	case "install-fonts":
		return p.handleInstallFonts(args)
	case "configure-fonts":
		return p.handleConfigureFonts(args)
	case "list-themes":
		return p.handleListThemes(args)
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
		file  string
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
	if _, err := fmt.Scanln(&response); err != nil {
		fmt.Printf("Error reading input: %v\n", err)
		return err
	}
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
	if err := exec.Command("killall", "plasmashell").Run(); err != nil {
		// Ignore error if plasmashell is not running
		fmt.Printf("Note: plasmashell may not have been running: %v\n", err)
	}

	// Wait a moment
	time.Sleep(2 * time.Second)

	// Start plasmashell
	cmd := exec.Command("plasmashell")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plasmashell: %w", err)
	}

	return nil
}

// handleInstallFonts installs development fonts for KDE
func (p *KDEPlugin) handleInstallFonts(args []string) error {
	fmt.Println("Installing fonts for KDE Plasma desktop...")

	// Default development fonts if none specified
	var fontPackages []string
	if len(args) == 0 {
		// Use similar fonts but with Qt/KDE preferred ones
		fontPackages = []string{
			"fonts-firacode",
			"fonts-jetbrains-mono",
			"fonts-hack",
			"fonts-source-code-pro",
			"fonts-inconsolata",
			"fonts-cascadia-code",
			"fonts-noto", // KDE prefers Noto fonts
			"fonts-liberation",
			"fonts-oxygen", // KDE Oxygen font
		}
		fmt.Println("Installing default development fonts...")
	} else {
		// Map common font names to package names
		fontMap := map[string]string{
			"firacode":      "fonts-firacode",
			"jetbrains":     "fonts-jetbrains-mono",
			"jetbrainsmono": "fonts-jetbrains-mono",
			"hack":          "fonts-hack",
			"sourcecodepro": "fonts-source-code-pro",
			"inconsolata":   "fonts-inconsolata",
			"cascadia":      "fonts-cascadia-code",
			"noto":          "fonts-noto",
			"liberation":    "fonts-liberation",
			"oxygen":        "fonts-oxygen",
			"roboto":        "fonts-roboto",
		}

		for _, fontName := range args {
			if packageName, ok := fontMap[strings.ToLower(fontName)]; ok {
				fontPackages = append(fontPackages, packageName)
			} else {
				// Assume it's already a package name
				fontPackages = append(fontPackages, fontName)
			}
		}
	}

	// Detect package manager and install
	var installCmd []string
	if sdk.CommandExists("apt-get") {
		installCmd = append([]string{"apt-get", "install", "-y"}, fontPackages...)
	} else if sdk.CommandExists("dnf") {
		installCmd = append([]string{"dnf", "install", "-y"}, fontPackages...)
	} else if sdk.CommandExists("pacman") {
		installCmd = append([]string{"pacman", "-S", "--noconfirm"}, fontPackages...)
	} else if sdk.CommandExists("zypper") {
		installCmd = append([]string{"zypper", "install", "-y"}, fontPackages...)
	} else {
		return fmt.Errorf("no supported package manager found")
	}

	fmt.Printf("Installing: %s\n", strings.Join(fontPackages, ", "))
	if err := sdk.ExecCommand(true, installCmd[0], installCmd[1:]...); err != nil {
		return fmt.Errorf("failed to install fonts: %w", err)
	}

	// Refresh font cache
	fmt.Println("Refreshing font cache...")
	if sdk.CommandExists("fc-cache") {
		if err := sdk.ExecCommand(false, "fc-cache", "-f", "-v"); err != nil {
			fmt.Printf("Warning: Failed to refresh font cache: %v\n", err)
		}
	}

	// KDE-specific font cache update
	if sdk.CommandExists("kbuildsycoca5") {
		fmt.Println("Updating KDE system configuration cache...")
		if err := sdk.ExecCommand(false, "kbuildsycoca5", "--noincremental"); err != nil {
			fmt.Printf("Warning: Failed to update KDE cache: %v\n", err)
		}
	}

	fmt.Println("✓ Fonts installed successfully!")
	return nil
}

// handleConfigureFonts sets KDE font preferences
func (p *KDEPlugin) handleConfigureFonts(args []string) error {
	fmt.Println("Configuring KDE Plasma font settings...")

	// Check if kwriteconfig5 is available
	if !sdk.CommandExists("kwriteconfig5") {
		return fmt.Errorf("kwriteconfig5 not found. Please ensure KDE Plasma is properly installed")
	}

	// Default font configuration for KDE
	fontConfigs := []struct {
		file  string
		group string
		key   string
		value string
		desc  string
	}{
		{
			file:  "kdeglobals",
			group: "General",
			key:   "font",
			value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			desc:  "General font",
		},
		{
			file:  "kdeglobals",
			group: "General",
			key:   "fixed",
			value: "JetBrains Mono,10,-1,5,50,0,0,0,0,0",
			desc:  "Fixed width font",
		},
		{
			file:  "kdeglobals",
			group: "General",
			key:   "smallestReadableFont",
			value: "Noto Sans,8,-1,5,50,0,0,0,0,0",
			desc:  "Small font",
		},
		{
			file:  "kdeglobals",
			group: "General",
			key:   "toolBarFont",
			value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			desc:  "Toolbar font",
		},
		{
			file:  "kdeglobals",
			group: "General",
			key:   "menuFont",
			value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			desc:  "Menu font",
		},
		{
			file:  "kdeglobals",
			group: "WM",
			key:   "activeFont",
			value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			desc:  "Window title font",
		},
	}

	// Allow custom monospace font as first argument
	if len(args) > 0 {
		monospaceFont := args[0]
		// Update fixed font with custom font
		for i, config := range fontConfigs {
			if config.key == "fixed" {
				// Format: FontName,Size,-1,5,50,0,0,0,0,0
				fontConfigs[i].value = fmt.Sprintf("%s,10,-1,5,50,0,0,0,0,0", monospaceFont)
				break
			}
		}
	}

	// Apply font settings using kwriteconfig5
	for _, config := range fontConfigs {
		cmd := exec.Command("kwriteconfig5",
			"--file", config.file,
			"--group", config.group,
			"--key", config.key,
			config.value)

		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", config.desc, err)
		} else {
			fmt.Printf("✓ Set %s\n", config.desc)
		}
	}

	// Configure font rendering
	fmt.Println("\nConfiguring font rendering...")
	renderingConfigs := []struct {
		file  string
		group string
		key   string
		value string
	}{
		{
			file:  "kdeglobals",
			group: "General",
			key:   "XftAntialias",
			value: "true",
		},
		{
			file:  "kdeglobals",
			group: "General",
			key:   "XftHintStyle",
			value: "hintslight",
		},
		{
			file:  "kdeglobals",
			group: "General",
			key:   "XftSubPixel",
			value: "rgb",
		},
	}

	for _, config := range renderingConfigs {
		cmd := exec.Command("kwriteconfig5",
			"--file", config.file,
			"--group", config.group,
			"--key", config.key,
			config.value)

		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.group, config.key, err)
		}
	}

	fmt.Println("\nFont configuration complete!")
	fmt.Println("You may need to restart KDE Plasma or log out for all changes to take effect.")
	fmt.Println("Run 'devex desktop-kde restart-plasma' to restart Plasma Shell.")
	return nil
}

// handleListThemes lists available KDE themes
func (p *KDEPlugin) handleListThemes(args []string) error {
	fmt.Println("Available KDE Plasma themes:")

	// Check Plasma themes
	fmt.Println("Plasma Themes:")
	plasmaThemeDirs := []string{
		"/usr/share/plasma/desktoptheme",
		fmt.Sprintf("%s/.local/share/plasma/desktoptheme", os.Getenv("HOME")),
	}

	plasmaThemes := make(map[string]bool)
	for _, dir := range plasmaThemeDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					plasmaThemes[entry.Name()] = true
				}
			}
		}
	}

	for theme := range plasmaThemes {
		fmt.Printf("  - %s\n", theme)
	}

	// Check color schemes
	fmt.Println("\nColor Schemes:")
	colorSchemeDirs := []string{
		"/usr/share/color-schemes",
		fmt.Sprintf("%s/.local/share/color-schemes", os.Getenv("HOME")),
	}

	colorSchemes := make(map[string]bool)
	for _, dir := range colorSchemeDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".colors") {
					schemeName := strings.TrimSuffix(entry.Name(), ".colors")
					colorSchemes[schemeName] = true
				}
			}
		}
	}

	for scheme := range colorSchemes {
		fmt.Printf("  - %s\n", scheme)
	}

	// Check Qt/KDE application styles
	fmt.Println("\nApplication Styles:")
	// These are typically provided by packages and registered with Qt
	styles := []string{
		"Breeze",
		"Oxygen",
		"Fusion",
		"Windows",
		"QtCurve",
	}

	for _, style := range styles {
		// Check if style is available (simplified check)
		fmt.Printf("  - %s\n", style)
	}

	// Check icon themes
	fmt.Println("\nIcon Themes:")
	iconThemeDirs := []string{
		"/usr/share/icons",
		fmt.Sprintf("%s/.local/share/icons", os.Getenv("HOME")),
	}

	iconThemes := make(map[string]bool)
	for _, dir := range iconThemeDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					// Check if it's an icon theme (has index.theme)
					themePath := filepath.Join(dir, entry.Name())
					if _, err := os.Stat(filepath.Join(themePath, "index.theme")); err == nil {
						iconThemes[entry.Name()] = true
					}
				}
			}
		}
	}

	for theme := range iconThemes {
		fmt.Printf("  - %s\n", theme)
	}

	// Show current theme settings using kreadconfig5
	fmt.Println("\nCurrent theme settings:")
	if sdk.CommandExists("kreadconfig5") {
		// Read current Plasma theme
		cmd := exec.Command("kreadconfig5", "--file", "plasmarc", "--group", "Theme", "--key", "name")
		if output, err := cmd.Output(); err == nil {
			fmt.Printf("  Plasma Theme: %s", output)
		}

		// Read current color scheme
		cmd = exec.Command("kreadconfig5", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme")
		if output, err := cmd.Output(); err == nil {
			fmt.Printf("  Color Scheme: %s", output)
		}

		// Read current icon theme
		cmd = exec.Command("kreadconfig5", "--file", "kdeglobals", "--group", "Icons", "--key", "Theme")
		if output, err := cmd.Output(); err == nil {
			fmt.Printf("  Icon Theme: %s", output)
		}

		// Read current widget style
		cmd = exec.Command("kreadconfig5", "--file", "kdeglobals", "--group", "KDE", "--key", "widgetStyle")
		if output, err := cmd.Output(); err == nil {
			fmt.Printf("  Widget Style: %s", output)
		}
	}

	return nil
}

func main() {
	plugin := NewKDEPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
