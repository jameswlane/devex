package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// DesktopManager handles core GNOME desktop configuration
type DesktopManager struct{}

// NewDesktopManager creates a new desktop manager instance
func NewDesktopManager() *DesktopManager {
	return &DesktopManager{}
}

// Configure applies comprehensive GNOME configuration
func (dm *DesktopManager) Configure(ctx context.Context, args []string) error {
	fmt.Println("Configuring GNOME desktop environment...")

	configs := dm.getDefaultConfigurations()

	for _, config := range configs {
		if err := setGSettingWithContext(ctx, config.schema, config.key, config.value); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.schema, config.key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.schema, config.key, config.value)
		}
	}

	fmt.Println("GNOME configuration complete!")
	return nil
}

// SetBackground sets the desktop wallpaper
func (dm *DesktopManager) SetBackground(ctx context.Context, args []string) error {
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

	// Set wallpaper
	uri := fmt.Sprintf("file://%s", absPath)
	if err := setGSettingWithContext(ctx, "org.gnome.desktop.background", "picture-uri", uri); err != nil {
		return fmt.Errorf("failed to set wallpaper: %w", err)
	}

	// Also set for dark mode
	if err := setGSettingWithContext(ctx, "org.gnome.desktop.background", "picture-uri-dark", uri); err != nil {
		// Non-critical error
		fmt.Printf("Warning: Failed to set dark mode wallpaper: %v\n", err)
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// ConfigureDock configures the GNOME dock
func (dm *DesktopManager) ConfigureDock(ctx context.Context, args []string) error {
	fmt.Println("Configuring GNOME dock...")

	// Check if dash-to-dock extension is available
	dashToDockConfigs := dm.getDashToDockConfigurations()

	hasErrors := false
	for _, config := range dashToDockConfigs {
		if err := setGSettingWithContext(ctx, config.schema, config.key, config.value); err != nil {
			// Dash-to-dock might not be installed
			hasErrors = true
		} else {
			fmt.Printf("✓ Set %s to %s\n", config.key, config.value)
		}
	}

	if hasErrors {
		fmt.Println("Note: Some dock settings failed. Dash-to-dock extension may not be installed.")
		fmt.Println("Run 'devex desktop-gnome install-extensions' to install recommended extensions.")
	}

	fmt.Println("Dock configuration complete!")
	return nil
}

// gsettingConfig represents a GSetting configuration
type gsettingConfig struct {
	schema string
	key    string
	value  string
}

// getDefaultConfigurations returns the default GNOME configurations
func (dm *DesktopManager) getDefaultConfigurations() []gsettingConfig {
	return []gsettingConfig{
		// Window buttons
		{"org.gnome.desktop.wm.preferences", "button-layout", "close,minimize,maximize:appmenu"},
		// Enable hot corners
		{"org.gnome.desktop.interface", "enable-hot-corners", "true"},
		// Show weekday in clock
		{"org.gnome.desktop.interface", "clock-show-weekday", "true"},
		// Enable natural scrolling
		{"org.gnome.desktop.peripherals.touchpad", "natural-scroll", "true"},
		// Tap to click
		{"org.gnome.desktop.peripherals.touchpad", "tap-to-click", "true"},
		// Show battery percentage
		{"org.gnome.desktop.interface", "show-battery-percentage", "true"},
		// Enable locate pointer
		{"org.gnome.desktop.interface", "locate-pointer", "true"},
		// Set theme variant to prefer dark
		{"org.gnome.desktop.interface", "color-scheme", "prefer-dark"},
	}
}

// getDashToDockConfigurations returns Dash-to-Dock specific configurations
func (dm *DesktopManager) getDashToDockConfigurations() []gsettingConfig {
	return []gsettingConfig{
		{"org.gnome.shell.extensions.dash-to-dock", "dock-position", "BOTTOM"},
		{"org.gnome.shell.extensions.dash-to-dock", "dash-max-icon-size", "48"},
		{"org.gnome.shell.extensions.dash-to-dock", "click-action", "minimize"},
		{"org.gnome.shell.extensions.dash-to-dock", "show-trash", "true"},
		{"org.gnome.shell.extensions.dash-to-dock", "show-mounts", "true"},
		{"org.gnome.shell.extensions.dash-to-dock", "show-apps-at-top", "true"},
		{"org.gnome.shell.extensions.dash-to-dock", "animate", "true"},
		{"org.gnome.shell.extensions.dash-to-dock", "autohide", "true"},
		{"org.gnome.shell.extensions.dash-to-dock", "intellihide", "false"},
	}
}
