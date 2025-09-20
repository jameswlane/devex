package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// ThemeManager handles KDE theme operations
type ThemeManager struct{}

// NewThemeManager creates a new theme manager instance
func NewThemeManager() *ThemeManager {
	return &ThemeManager{}
}

// ListThemes lists available KDE themes
func (tm *ThemeManager) ListThemes(ctx context.Context, args []string) error {
	fmt.Println("Available KDE Plasma themes:")

	if err := tm.listPlasmaThemes(); err != nil {
		fmt.Printf("Warning: Failed to list Plasma themes: %v\n", err)
	}

	if err := tm.listColorSchemes(); err != nil {
		fmt.Printf("Warning: Failed to list color schemes: %v\n", err)
	}

	if err := tm.listApplicationStyles(); err != nil {
		fmt.Printf("Warning: Failed to list application styles: %v\n", err)
	}

	if err := tm.listIconThemes(); err != nil {
		fmt.Printf("Warning: Failed to list icon themes: %v\n", err)
	}

	if err := tm.showCurrentThemes(); err != nil {
		fmt.Printf("Warning: Failed to show current themes: %v\n", err)
	}

	return nil
}

// ApplyTheme applies KDE themes
func (tm *ThemeManager) ApplyTheme(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying theme: %s\n", themeName)

	// Apply Plasma theme
	if err := tm.applyPlasmaTheme(themeName); err != nil {
		fmt.Printf("Warning: Failed to set Plasma theme: %v\n", err)
	}

	// Apply color scheme if it exists
	if err := tm.applyColorScheme(themeName); err != nil {
		fmt.Printf("Note: '%s' may not be a color scheme\n", themeName)
	}

	// Apply widget style
	if err := tm.applyWidgetStyle(themeName); err != nil {
		fmt.Printf("Note: '%s' may not be a widget style\n", themeName)
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	fmt.Println("You may need to restart applications for all changes to take effect.")
	return nil
}

// listPlasmaThemes lists available Plasma themes
func (tm *ThemeManager) listPlasmaThemes() error {
	fmt.Println("\nPlasma Themes:")
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

	return nil
}

// listColorSchemes lists available color schemes
func (tm *ThemeManager) listColorSchemes() error {
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

	return nil
}

// listApplicationStyles lists available Qt/KDE application styles
func (tm *ThemeManager) listApplicationStyles() error {
	fmt.Println("\nApplication Styles:")
	// These are typically provided by packages and registered with Qt
	styles := tm.getKnownStyles()

	for _, style := range styles {
		fmt.Printf("  - %s\n", style)
	}

	return nil
}

// listIconThemes lists available icon themes
func (tm *ThemeManager) listIconThemes() error {
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
					if tm.isIconTheme(themePath) {
						iconThemes[entry.Name()] = true
					}
				}
			}
		}
	}

	for theme := range iconThemes {
		fmt.Printf("  - %s\n", theme)
	}

	return nil
}

// showCurrentThemes displays current theme settings
func (tm *ThemeManager) showCurrentThemes() error {
	fmt.Println("\nCurrent theme settings:")

	if !sdk.CommandExists("kreadconfig5") {
		fmt.Println("  kreadconfig5 not available - cannot read current settings")
		return nil
	}

	// Read current Plasma theme
	if output, err := tm.readKDESetting("plasmarc", "Theme", "name"); err == nil {
		fmt.Printf("  Plasma Theme: %s", output)
	}

	// Read current color scheme
	if output, err := tm.readKDESetting("kdeglobals", "General", "ColorScheme"); err == nil {
		fmt.Printf("  Color Scheme: %s", output)
	}

	// Read current icon theme
	if output, err := tm.readKDESetting("kdeglobals", "Icons", "Theme"); err == nil {
		fmt.Printf("  Icon Theme: %s", output)
	}

	// Read current widget style
	if output, err := tm.readKDESetting("kdeglobals", "KDE", "widgetStyle"); err == nil {
		fmt.Printf("  Widget Style: %s", output)
	}

	return nil
}

// applyPlasmaTheme applies a Plasma desktop theme
func (tm *ThemeManager) applyPlasmaTheme(themeName string) error {
	if !sdk.CommandExists("plasma-apply-desktoptheme") {
		// Fallback to kwriteconfig5
		cmd := exec.Command("kwriteconfig5", "--file", "plasmarc", "--group", "Theme", "--key", "name", themeName)
		return cmd.Run()
	}

	cmd := exec.Command("plasma-apply-desktoptheme", themeName)
	return cmd.Run()
}

// applyColorScheme applies a color scheme
func (tm *ThemeManager) applyColorScheme(schemeName string) error {
	if !sdk.CommandExists("plasma-apply-colorscheme") {
		// Fallback to kwriteconfig5
		cmd := exec.Command("kwriteconfig5", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme", schemeName)
		return cmd.Run()
	}

	cmd := exec.Command("plasma-apply-colorscheme", schemeName)
	return cmd.Run()
}

// applyWidgetStyle applies a widget style
func (tm *ThemeManager) applyWidgetStyle(styleName string) error {
	cmd := exec.Command("kwriteconfig5", "--file", "kdeglobals", "--group", "KDE", "--key", "widgetStyle", styleName)
	return cmd.Run()
}

// isIconTheme checks if a directory contains an icon theme
func (tm *ThemeManager) isIconTheme(themePath string) bool {
	_, err := os.Stat(filepath.Join(themePath, "index.theme"))
	return err == nil
}

// readKDESetting reads a KDE configuration setting
func (tm *ThemeManager) readKDESetting(file, group, key string) ([]byte, error) {
	cmd := exec.Command("kreadconfig5", "--file", file, "--group", group, "--key", key)
	return cmd.Output()
}

// getKnownStyles returns a list of commonly available Qt/KDE styles
func (tm *ThemeManager) getKnownStyles() []string {
	return []string{
		"Breeze",
		"Oxygen",
		"Fusion",
		"Windows",
		"QtCurve",
		"Adwaita",
		"Adwaita-Dark",
	}
}
