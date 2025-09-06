package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// FontManager handles GNOME font operations
type FontManager struct{}

// NewFontManager creates a new font manager instance
func NewFontManager() *FontManager {
	return &FontManager{}
}

// InstallFonts installs development fonts for GNOME
func (fm *FontManager) InstallFonts(ctx context.Context, args []string) error {
	// Validate input arguments
	for _, arg := range args {
		if err := validateFontName(arg); err != nil {
			return fmt.Errorf("invalid font argument %q: %w", arg, err)
		}
	}
	fmt.Println("Installing fonts for GNOME desktop...")

	// Default development fonts if none specified
	var fontPackages []string
	if len(args) == 0 {
		fontPackages = []string{
			"fonts-firacode",
			"fonts-jetbrains-mono",
			"fonts-hack",
			"fonts-source-code-pro",
			"fonts-inconsolata",
			"fonts-cascadia-code",
			"fonts-ubuntu",
			"fonts-liberation",
		}
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
	for _, setting := range fontSettings {
		if err := setGSettingWithContext(ctx, setting.schema, setting.key, setting.value); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", setting.desc, err)
		} else {
			fmt.Printf("✓ Set %s to %s\n", setting.desc, setting.value)
		}
	}

	fmt.Println("\nFont configuration complete!")
	fmt.Println("You may need to restart applications for changes to take effect.")
	return nil
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
		"ubuntu":        "fonts-ubuntu",
		"liberation":    "fonts-liberation",
		"noto":          "fonts-noto",
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

// executeCommand executes system commands with context support and security
func executeCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}
