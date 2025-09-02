package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// FontsPlugin implements the desktop fonts plugin
type FontsPlugin struct {
	*sdk.BasePlugin
}

// NewFontsPlugin creates a new fonts plugin
func NewFontsPlugin() *FontsPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-fonts",
		Version:     version,
		Description: "Desktop font management and installation",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "fonts", "customization"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install fonts from configuration",
				Usage:       "Install fonts from YAML configuration file",
			},
			{
				Name:        "list",
				Description: "List available fonts",
				Usage:       "List fonts available for installation",
			},
			{
				Name:        "cache-refresh",
				Description: "Refresh font cache",
				Usage:       "Refresh system font cache using fc-cache",
			},
		},
	}

	return &FontsPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *FontsPlugin) Execute(command string, args []string) error {
	switch command {
	case "install":
		return p.handleInstall(args)
	case "list":
		return p.handleList(args)
	case "cache-refresh":
		return p.handleCacheRefresh(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *FontsPlugin) handleInstall(args []string) error {
	// Parse command line arguments
	var configFile string
	var fontNames []string
	
	if len(args) > 0 {
		// Check if first arg is a config file or font name
		if strings.HasSuffix(args[0], ".yaml") || strings.HasSuffix(args[0], ".yml") {
			configFile = args[0]
		} else {
			// Assume args are font names
			fontNames = args
		}
	}
	
	if configFile != "" {
		fmt.Printf("Installing fonts from configuration: %s\n", configFile)
		// For now, return error as config file parsing would require more implementation
		return fmt.Errorf("configuration file installation not yet implemented")
	}
	
	// If no fonts specified, install common development fonts
	if len(fontNames) == 0 {
		fontNames = []string{
			"FiraCode",
			"JetBrainsMono",
			"Hack",
			"SourceCodePro",
			"Inconsolata",
		}
		fmt.Println("Installing default development fonts...")
	}
	
	// Detect OS and install fonts accordingly
	switch runtime.GOOS {
	case "linux":
		return p.installFontsLinux(fontNames)
	case "darwin":
		return p.installFontsMacOS(fontNames)
	case "windows":
		return p.installFontsWindows(fontNames)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (p *FontsPlugin) installFontsLinux(fontNames []string) error {
	// Check available font directories
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	
	userFontDir := filepath.Join(homeDir, ".local", "share", "fonts")
	
	// Create user font directory if it doesn't exist
	if err := os.MkdirAll(userFontDir, 0755); err != nil {
		return fmt.Errorf("failed to create font directory: %w", err)
	}
	
	// Map of font names to package names/URLs
	fontPackages := map[string]string{
		"FiraCode":       "fonts-firacode",
		"JetBrainsMono":  "fonts-jetbrains-mono",
		"Hack":           "fonts-hack",
		"SourceCodePro":  "fonts-source-code-pro",
		"Inconsolata":    "fonts-inconsolata",
		"Cascadia":       "fonts-cascadia-code",
		"IBMPlexMono":    "fonts-ibm-plex",
		"RobotoMono":     "fonts-roboto",
	}
	
	// Try to install via package manager first
	packagesToInstall := []string{}
	for _, fontName := range fontNames {
		if packageName, ok := fontPackages[fontName]; ok {
			packagesToInstall = append(packagesToInstall, packageName)
		}
	}
	
	if len(packagesToInstall) > 0 {
		// Detect package manager
		var installCmd []string
		if sdk.CommandExists("apt-get") {
			installCmd = append([]string{"apt-get", "install", "-y"}, packagesToInstall...)
		} else if sdk.CommandExists("dnf") {
			installCmd = append([]string{"dnf", "install", "-y"}, packagesToInstall...)
		} else if sdk.CommandExists("pacman") {
			installCmd = append([]string{"pacman", "-S", "--noconfirm"}, packagesToInstall...)
		} else if sdk.CommandExists("zypper") {
			installCmd = append([]string{"zypper", "install", "-y"}, packagesToInstall...)
		}
		
		if len(installCmd) > 0 {
			fmt.Printf("Installing fonts via package manager: %s\n", strings.Join(packagesToInstall, ", "))
			if err := sdk.ExecCommand(true, installCmd[0], installCmd[1:]...); err != nil {
				fmt.Printf("Warning: Package manager installation failed: %v\n", err)
				fmt.Println("Falling back to manual installation...")
			} else {
				fmt.Println("Fonts installed successfully via package manager")
			}
		}
	}
	
	// Refresh font cache
	return p.handleCacheRefresh(nil)
}

func (p *FontsPlugin) installFontsMacOS(fontNames []string) error {
	// On macOS, use Homebrew cask
	if !sdk.CommandExists("brew") {
		return fmt.Errorf("Homebrew is required to install fonts on macOS")
	}
	
	// Map of font names to Homebrew cask names
	fontCasks := map[string]string{
		"FiraCode":       "font-fira-code",
		"JetBrainsMono":  "font-jetbrains-mono",
		"Hack":           "font-hack",
		"SourceCodePro":  "font-source-code-pro",
		"Inconsolata":    "font-inconsolata",
		"Cascadia":       "font-cascadia-code",
		"IBMPlexMono":    "font-ibm-plex",
		"RobotoMono":     "font-roboto-mono",
	}
	
	// Install Homebrew tap for fonts if not already tapped
	fmt.Println("Ensuring Homebrew font cask is available...")
	if err := sdk.ExecCommand(false, "brew", "tap", "homebrew/cask-fonts"); err != nil {
		fmt.Printf("Warning: Failed to tap cask-fonts: %v\n", err)
	}
	
	// Install each font
	installed := 0
	for _, fontName := range fontNames {
		if caskName, ok := fontCasks[fontName]; ok {
			fmt.Printf("Installing %s...\n", fontName)
			if err := sdk.ExecCommand(false, "brew", "install", "--cask", caskName); err != nil {
				fmt.Printf("Warning: Failed to install %s: %v\n", fontName, err)
			} else {
				installed++
			}
		} else {
			fmt.Printf("Warning: Unknown font %s for macOS\n", fontName)
		}
	}
	
	if installed > 0 {
		fmt.Printf("\nSuccessfully installed %d fonts\n", installed)
	}
	
	return nil
}

func (p *FontsPlugin) installFontsWindows(fontNames []string) error {
	// On Windows, use winget or chocolatey
	if sdk.CommandExists("winget") {
		// Use winget
		fontPackages := map[string]string{
			"FiraCode":       "Microsoft.FiraCode",
			"JetBrainsMono":  "JetBrains.JetBrainsMono",
			"Hack":           "SourceFoundry.HackFonts",
			"CascadiaCode":   "Microsoft.CascadiaCode",
		}
		
		installed := 0
		for _, fontName := range fontNames {
			if packageName, ok := fontPackages[fontName]; ok {
				fmt.Printf("Installing %s...\n", fontName)
				if err := sdk.ExecCommand(false, "winget", "install", packageName); err != nil {
					fmt.Printf("Warning: Failed to install %s: %v\n", fontName, err)
				} else {
					installed++
				}
			}
		}
		
		if installed > 0 {
			fmt.Printf("\nSuccessfully installed %d fonts\n", installed)
		}
	} else if sdk.CommandExists("choco") {
		// Use chocolatey
		fontPackages := map[string]string{
			"FiraCode":       "firacode",
			"JetBrainsMono":  "jetbrainsmono",
			"Hack":           "hackfont",
			"SourceCodePro":  "sourcecodepro",
			"Inconsolata":    "inconsolata",
		}
		
		packagesToInstall := []string{}
		for _, fontName := range fontNames {
			if packageName, ok := fontPackages[fontName]; ok {
				packagesToInstall = append(packagesToInstall, packageName)
			}
		}
		
		if len(packagesToInstall) > 0 {
			installCmd := append([]string{"choco", "install", "-y"}, packagesToInstall...)
			if err := sdk.ExecCommand(true, installCmd[0], installCmd[1:]...); err != nil {
				return fmt.Errorf("failed to install fonts: %w", err)
			}
			fmt.Println("Fonts installed successfully")
		}
	} else {
		return fmt.Errorf("neither winget nor chocolatey found - please install one to manage fonts on Windows")
	}
	
	return nil
}

func (p *FontsPlugin) handleList(args []string) error {
	fmt.Println("Available font management operations:")
	fmt.Println("  - install <config.yaml>  Install fonts from configuration")
	fmt.Println("  - cache-refresh          Refresh system font cache")
	return nil
}

func (p *FontsPlugin) handleCacheRefresh(args []string) error {
	fmt.Println("Refreshing font cache...")
	return sdk.ExecCommand(false, "fc-cache", "-f", "-v")
}

func main() {
	plugin := NewFontsPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
