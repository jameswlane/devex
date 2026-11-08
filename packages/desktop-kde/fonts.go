package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// systemScope is install-fonts' declared privilege: package installs change the system.
var systemScope = sdk.OperationPrivilege{Scope: sdk.ScopeSystem}

// userScope keeps per-user cache refreshes with the invoking account, switching
// back to SUDO_USER when the plugin itself runs elevated.
var userScope = sdk.OperationPrivilege{Scope: sdk.ScopeUser}

// FontManager handles KDE font operations
type FontManager struct{}

// NewFontManager creates a new font manager instance
func NewFontManager() *FontManager {
	return &FontManager{}
}

// InstallFonts installs development fonts for KDE
func (fm *FontManager) InstallFonts(ctx context.Context, args []string) error {
	if err := fm.executeInstall(ctx, args); err != nil {
		return fmt.Errorf("failed to install fonts: %w", err)
	}
	fmt.Println("✓ Fonts installed successfully!")
	return nil
}

// ConfigureFonts sets KDE font preferences
func (fm *FontManager) ConfigureFonts(ctx context.Context, args []string) error {
	// Validate font name arguments
	for _, arg := range args {
		if err := validateFontName(arg); err != nil {
			return fmt.Errorf("invalid font name %q: %w", arg, err)
		}
	}
	fmt.Println("Configuring KDE Plasma font settings...")

	// Check if kwriteconfig5 is available
	if !sdk.CommandExists("kwriteconfig5") {
		return fmt.Errorf("kwriteconfig5 not found. Please ensure KDE Plasma is properly installed")
	}

	fontConfigs := fm.getDefaultKDEFontSettings()

	// Allow custom monospace font as first argument
	if len(args) > 0 {
		monospaceFont := args[0]
		// Additional validation for font value format
		if err := validateFontName(monospaceFont); err != nil {
			return fmt.Errorf("invalid monospace font name %q: %w", monospaceFont, err)
		}
		for i, config := range fontConfigs {
			if config.Key == "fixed" {
				// Format: FontName,Size,-1,5,50,0,0,0,0,0
				fontConfigs[i].Value = fmt.Sprintf("%s,10,-1,5,50,0,0,0,0,0", monospaceFont)
				break
			}
		}
	}

	// Apply font settings using kwriteconfig5
	var batch settingsBatch
	for _, config := range fontConfigs {
		if err := batch.record(fm.setKDEConfig(ctx, config)); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", config.Desc, err)
		} else {
			fmt.Printf("✓ Set %s\n", config.Desc)
		}
	}

	// Configure font rendering
	fm.configureFontRendering(ctx, &batch)
	if err := batch.err("kde font configuration"); err != nil {
		return err
	}

	fmt.Println("\nFont configuration complete!")
	fmt.Println("You may need to restart KDE Plasma or log out for all changes to take effect.")
	fmt.Println("Run 'devex desktop-kde restart-plasma' to restart Plasma Shell.")
	return nil
}

// getDefaultKDEFontSettings returns the default KDE font configuration
func (fm *FontManager) getDefaultKDEFontSettings() []KDEConfig {
	return []KDEConfig{
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "font",
			Value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			Desc:  "General font",
		},
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "fixed",
			Value: "JetBrains Mono,10,-1,5,50,0,0,0,0,0",
			Desc:  "Fixed width font",
		},
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "smallestReadableFont",
			Value: "Noto Sans,8,-1,5,50,0,0,0,0,0",
			Desc:  "Small font",
		},
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "toolBarFont",
			Value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			Desc:  "Toolbar font",
		},
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "menuFont",
			Value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			Desc:  "Menu font",
		},
		{
			File:  "kdeglobals",
			Group: "WM",
			Key:   "activeFont",
			Value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
			Desc:  "Window title font",
		},
	}
}

// setKDEConfig sets a KDE configuration using kwriteconfig5
func (fm *FontManager) setKDEConfig(ctx context.Context, config KDEConfig) error {
	return executeCommand(ctx, "kwriteconfig5",
		"--file", config.File,
		"--group", config.Group,
		"--key", config.Key,
		config.Value)
}

// configureFontRendering configures KDE font rendering settings, recording
// each write in the font configuration's batch.
func (fm *FontManager) configureFontRendering(ctx context.Context, batch *settingsBatch) {
	fmt.Println("\nConfiguring font rendering...")
	renderingConfigs := []KDEConfig{
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "XftAntialias",
			Value: "true",
			Desc:  "Font antialiasing",
		},
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "XftHintStyle",
			Value: "hintslight",
			Desc:  "Font hinting style",
		},
		{
			File:  "kdeglobals",
			Group: "General",
			Key:   "XftSubPixel",
			Value: "rgb",
			Desc:  "Subpixel rendering",
		},
	}

	for _, config := range renderingConfigs {
		if err := batch.record(fm.setKDEConfig(ctx, config)); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", config.Desc, err)
		}
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

// executeCommand executes system commands with context support and security
func executeCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}
