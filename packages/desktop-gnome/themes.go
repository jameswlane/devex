package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ThemeManager handles GNOME theme operations
type ThemeManager struct{}

// NewThemeManager creates a new theme manager instance
func NewThemeManager() *ThemeManager {
	return &ThemeManager{}
}

// ListThemes lists available GNOME themes
func (tm *ThemeManager) ListThemes(ctx context.Context, args []string) error {
	fmt.Println("Available GNOME themes:")

	if err := tm.listGTKThemes(); err != nil {
		fmt.Printf("Warning: Failed to list GTK themes: %v\n", err)
	}

	if err := tm.listShellThemes(); err != nil {
		fmt.Printf("Warning: Failed to list Shell themes: %v\n", err)
	}

	if err := tm.listIconThemes(); err != nil {
		fmt.Printf("Warning: Failed to list icon themes: %v\n", err)
	}

	if err := tm.showCurrentThemes(); err != nil {
		fmt.Printf("Warning: Failed to show current themes: %v\n", err)
	}

	return nil
}

// ApplyTheme applies GTK and Shell themes
func (tm *ThemeManager) ApplyTheme(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a theme name")
	}

	themeName := args[0]
	fmt.Printf("Applying theme: %s\n", themeName)

	// Apply GTK theme
	if err := setGSettingWithContext(ctx, "org.gnome.desktop.interface", "gtk-theme", themeName); err != nil {
		return fmt.Errorf("failed to set GTK theme: %w", err)
	}

	// Apply icon theme (if it's an icon theme)
	if strings.Contains(strings.ToLower(themeName), "icon") {
		if err := setGSettingWithContext(ctx, "org.gnome.desktop.interface", "icon-theme", themeName); err != nil {
			fmt.Printf("Warning: Failed to set icon theme: %v\n", err)
		}
	}

	// Apply shell theme (requires user-theme extension)
	if err := setGSettingWithContext(ctx, "org.gnome.shell.extensions.user-theme", "name", themeName); err != nil {
		fmt.Printf("Note: Failed to set shell theme. User Theme extension may not be installed.\n")
	}

	fmt.Printf("✓ Theme '%s' applied successfully!\n", themeName)
	return nil
}

// listGTKThemes lists available GTK themes
func (tm *ThemeManager) listGTKThemes() error {
	fmt.Println("\nGTK Themes:")
	gtkThemeDirs := []string{
		"/usr/share/themes",
		fmt.Sprintf("%s/.themes", os.Getenv("HOME")),
		fmt.Sprintf("%s/.local/share/themes", os.Getenv("HOME")),
	}

	gtkThemes := make(map[string]bool)
	for _, dir := range gtkThemeDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					// Check if it's a GTK theme (has gtk-3.0 or gtk-4.0 folder)
					themePath := filepath.Join(dir, entry.Name())
					if tm.isGTKTheme(themePath) {
						gtkThemes[entry.Name()] = true
					}
				}
			}
		}
	}

	for theme := range gtkThemes {
		fmt.Printf("  - %s\n", theme)
	}

	return nil
}

// listShellThemes lists available GNOME Shell themes
func (tm *ThemeManager) listShellThemes() error {
	fmt.Println("\nShell Themes:")
	shellThemeDirs := []string{
		"/usr/share/gnome-shell/theme",
		fmt.Sprintf("%s/.themes", os.Getenv("HOME")),
		fmt.Sprintf("%s/.local/share/themes", os.Getenv("HOME")),
	}

	shellThemes := make(map[string]bool)
	for _, dir := range shellThemeDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					// Check if it's a Shell theme (has gnome-shell folder)
					themePath := filepath.Join(dir, entry.Name())
					if tm.isShellTheme(themePath) {
						shellThemes[entry.Name()] = true
					}
				}
			}
		}
	}

	for theme := range shellThemes {
		fmt.Printf("  - %s\n", theme)
	}

	return nil
}

// listIconThemes lists available icon themes
func (tm *ThemeManager) listIconThemes() error {
	fmt.Println("\nIcon Themes:")
	iconThemeDirs := []string{
		"/usr/share/icons",
		fmt.Sprintf("%s/.icons", os.Getenv("HOME")),
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

	if output, err := tm.getCurrentGSetting("org.gnome.desktop.interface", "gtk-theme"); err == nil {
		fmt.Printf("  GTK Theme: %s", output)
	}

	if output, err := tm.getCurrentGSetting("org.gnome.desktop.interface", "icon-theme"); err == nil {
		fmt.Printf("  Icon Theme: %s", output)
	}

	if output, err := tm.getCurrentGSetting("org.gnome.shell.extensions.user-theme", "name"); err == nil {
		fmt.Printf("  Shell Theme: %s", output)
	}

	return nil
}

// isGTKTheme checks if a directory contains a GTK theme
func (tm *ThemeManager) isGTKTheme(themePath string) bool {
	if _, err := os.Stat(filepath.Join(themePath, "gtk-3.0")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(themePath, "gtk-4.0")); err == nil {
		return true
	}
	return false
}

// isShellTheme checks if a directory contains a GNOME Shell theme
func (tm *ThemeManager) isShellTheme(themePath string) bool {
	_, err := os.Stat(filepath.Join(themePath, "gnome-shell"))
	return err == nil
}

// isIconTheme checks if a directory contains an icon theme
func (tm *ThemeManager) isIconTheme(themePath string) bool {
	_, err := os.Stat(filepath.Join(themePath, "index.theme"))
	return err == nil
}

// getCurrentGSetting gets a current gsetting value
func (tm *ThemeManager) getCurrentGSetting(schema, key string) ([]byte, error) {
	cmd := exec.Command("gsettings", "get", schema, key)
	return cmd.Output()
}
