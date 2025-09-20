package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// FontManager handles KDE font operations
type FontManager struct{}

// NewFontManager creates a new font manager instance
func NewFontManager() *FontManager {
	return &FontManager{}
}

// InstallFonts installs development fonts for KDE
func (fm *FontManager) InstallFonts(ctx context.Context, args []string) error {
	// Validate input arguments
	for _, arg := range args {
		if err := validateFontName(arg); err != nil {
			return fmt.Errorf("invalid font argument %q: %w", arg, err)
		}
	}
	fmt.Println("Installing fonts for KDE Plasma desktop...")

	// Default development fonts if none specified
	var fontPackages []string
	if len(args) == 0 {
		fontPackages = fm.getDefaultKDEFonts()
		fmt.Println("Installing default development fonts...")
	} else {
		fontPackages = fm.mapFontNames(args)
	}

	if err := fm.installPackages(ctx, fontPackages); err != nil {
		return fmt.Errorf("failed to install fonts: %w", err)
	}

	if err := fm.refreshFontCache(ctx); err != nil {
		fmt.Printf("Warning: Failed to refresh font cache: %v\n", err)
	}

	if err := fm.updateKDECache(ctx); err != nil {
		fmt.Printf("Warning: Failed to update KDE cache: %v\n", err)
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
	for _, config := range fontConfigs {
		if err := fm.setKDEConfig(ctx, config); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", config.Desc, err)
		} else {
			fmt.Printf("✓ Set %s\n", config.Desc)
		}
	}

	// Configure font rendering
	if err := fm.configureFontRendering(ctx); err != nil {
		fmt.Printf("Warning: Failed to configure font rendering: %v\n", err)
	}

	fmt.Println("\nFont configuration complete!")
	fmt.Println("You may need to restart KDE Plasma or log out for all changes to take effect.")
	fmt.Println("Run 'devex desktop-kde restart-plasma' to restart Plasma Shell.")
	return nil
}

// getDefaultKDEFonts returns the default font packages for KDE
func (fm *FontManager) getDefaultKDEFonts() []string {
	return []string{
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
}

// mapFontNames converts common font names to package names
func (fm *FontManager) mapFontNames(args []string) []string {
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

	var fontPackages []string
	for _, fontName := range args {
		if packageName, ok := fontMap[strings.ToLower(fontName)]; ok {
			fontPackages = append(fontPackages, packageName)
		} else {
			// Assume it's already a package name
			fontPackages = append(fontPackages, fontName)
		}
	}

	return fontPackages
}

// installPackages installs font packages using appropriate package manager
func (fm *FontManager) installPackages(ctx context.Context, fontPackages []string) error {
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
	return executeCommand(ctx, installCmd[0], installCmd[1:]...)
}

// refreshFontCache refreshes the system font cache
func (fm *FontManager) refreshFontCache(ctx context.Context) error {
	fmt.Println("Refreshing font cache...")
	if sdk.CommandExists("fc-cache") {
		return executeCommand(ctx, "fc-cache", "-f", "-v")
	}
	return nil
}

// updateKDECache updates KDE system configuration cache
func (fm *FontManager) updateKDECache(ctx context.Context) error {
	if sdk.CommandExists("kbuildsycoca5") {
		fmt.Println("Updating KDE system configuration cache...")
		return executeCommand(ctx, "kbuildsycoca5", "--noincremental")
	}
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

// configureFontRendering configures KDE font rendering settings
func (fm *FontManager) configureFontRendering(ctx context.Context) error {
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
		if err := fm.setKDEConfig(ctx, config); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", config.Desc, err)
		}
	}

	return nil
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
