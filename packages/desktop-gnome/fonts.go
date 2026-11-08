package main

import (
	"context"
	"fmt"
	"regexp"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// systemScope is install-fonts' declared privilege: package installs change the system.
var systemScope = sdk.OperationPrivilege{Scope: sdk.ScopeSystem}

// userScope keeps per-user cache refreshes with the invoking account, switching
// back to SUDO_USER when the plugin itself runs elevated.
var userScope = sdk.OperationPrivilege{Scope: sdk.ScopeUser}

// FontManager handles GNOME font operations
type FontManager struct{}

// NewFontManager creates a new font manager instance
func NewFontManager() *FontManager {
	return &FontManager{}
}

// InstallFonts installs development fonts for GNOME
func (fm *FontManager) InstallFonts(ctx context.Context, args []string) error {
	if err := fm.executeInstall(ctx, args); err != nil {
		return fmt.Errorf("failed to install fonts: %w", err)
	}
	fmt.Println("✓ Fonts installed successfully!")
	return nil
}

// ConfigureFonts sets GNOME font preferences
func (fm *FontManager) ConfigureFonts(ctx context.Context, args []string) error {
	// Validate font name arguments
	for _, arg := range args {
		if err := validateFontName(arg); err != nil {
			return fmt.Errorf("invalid font name %q: %w", arg, err)
		}
	}
	fmt.Println("Configuring GNOME font settings...")

	fontSettings := fm.getDefaultFontSettings()

	// Allow custom monospace font as first argument
	if len(args) > 0 {
		monospaceFont := args[0]
		for i, setting := range fontSettings {
			if setting.key == "monospace-font-name" {
				fontSettings[i].value = monospaceFont
				break
			}
		}
	}

	// Apply font settings
	var batch settingsBatch
	for _, setting := range fontSettings {
		if err := batch.record(setGSettingWithContext(ctx, setting.schema, setting.key, setting.value)); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", setting.desc, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", setting.desc, setting.value)
		}
	}
	if err := batch.err("gnome font configuration"); err != nil {
		return err
	}

	fmt.Println("\nFont configuration complete!")
	fmt.Println("You may need to restart applications for changes to take effect.")
	return nil
}

// fontSetting represents a GNOME font configuration setting
type fontSetting struct {
	schema string
	key    string
	value  string
	desc   string
}

// getDefaultFontSettings returns the default GNOME font configuration
func (fm *FontManager) getDefaultFontSettings() []fontSetting {
	return []fontSetting{
		{
			schema: "org.gnome.desktop.interface",
			key:    "font-name",
			value:  "Ubuntu 11",
			desc:   "System font",
		},
		{
			schema: "org.gnome.desktop.interface",
			key:    "document-font-name",
			value:  "Sans 11",
			desc:   "Document font",
		},
		{
			schema: "org.gnome.desktop.interface",
			key:    "monospace-font-name",
			value:  "JetBrains Mono 10",
			desc:   "Monospace font",
		},
		{
			schema: "org.gnome.desktop.wm.preferences",
			key:    "titlebar-font",
			value:  "Ubuntu Bold 11",
			desc:   "Window title font",
		},
		{
			schema: "org.gnome.desktop.interface",
			key:    "font-antialiasing",
			value:  "rgba",
			desc:   "Font antialiasing",
		},
		{
			schema: "org.gnome.desktop.interface",
			key:    "font-hinting",
			value:  "slight",
			desc:   "Font hinting",
		},
	}
}

// validateFontName validates font name arguments to prevent injection
func validateFontName(fontName string) error {
	// Allow letters, numbers, spaces, and common font characters
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9\s\-\.]+$`)
	if !validPattern.MatchString(fontName) {
		return fmt.Errorf("contains illegal characters")
	}
	if len(fontName) > 100 {
		return fmt.Errorf("font name too long")
	}
	return nil
}
