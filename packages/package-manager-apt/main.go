package main

// Build timestamp: 2025-09-20 19:40:00
// Enhanced with improved error handling and diagnostics

import (
	"fmt"
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewAPTPlugin creates a new APT plugin
func NewAPTPlugin() *APTInstaller {
	info := sdk.PluginInfo{
		Name:        "package-manager-apt",
		Version:     version,
		Description: "APT package manager support for Debian/Ubuntu systems",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "apt", "debian", "ubuntu", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using APT",
				Usage:       "Install one or more packages with automatic dependency resolution",
				Flags: map[string]string{
					"yes":        "Automatically answer yes to prompts",
					"update":     "Update package lists before installing",
					"fix-broken": "Attempt to fix broken dependencies",
				},
			},
			{
				Name:        "remove",
				Description: "Remove packages using APT",
				Usage:       "Remove one or more packages from the system",
				Flags: map[string]string{
					"purge":      "Remove packages and their configuration files",
					"autoremove": "Remove automatically installed dependencies that are no longer needed",
				},
			},
			{
				Name:        "update",
				Description: "Update package lists",
				Usage:       "Download package information from all configured sources",
			},
			{
				Name:        "upgrade",
				Description: "Upgrade installed packages",
				Usage:       "Install newer versions of all installed packages",
				Flags: map[string]string{
					"dist-upgrade": "Intelligently handle changing dependencies",
				},
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages or search for available packages",
				Flags: map[string]string{
					"installed":  "List only installed packages",
					"upgradable": "List only upgradable packages",
					"manual":     "List manually installed packages",
				},
			},
			{
				Name:        "info",
				Description: "Show package information",
				Usage:       "Display detailed information about a package",
			},
			{
				Name:        "is-installed",
				Description: "Check if a package is installed",
				Usage:       "Returns exit code 0 if package is installed, 1 if not",
			},
			{
				Name:        "add-repository",
				Description: "Add a new APT repository with GPG key",
				Usage:       "Add repository with automatic GPG key handling and validation",
				Flags: map[string]string{
					"key-url":         "URL to download the GPG key",
					"key-path":        "Local path to store the GPG key",
					"source-line":     "APT source line to add",
					"source-file":     "File to store the source line",
					"require-dearmor": "Convert ASCII-armored key to binary format",
				},
			},
			{
				Name:        "remove-repository",
				Description: "Remove an APT repository and its GPG key",
				Usage:       "Remove repository source file and associated GPG key",
				Flags: map[string]string{
					"source-file": "Source file to remove",
					"key-path":    "GPG key file to remove",
				},
			},
			{
				Name:        "validate-repository",
				Description: "Validate repository configuration and GPG keys",
				Usage:       "Check that repository sources and keys are properly configured",
			},
			{
				Name:        "health-check",
				Description: "Perform APT system health check",
				Usage:       "Check APT configuration, locks, and repository connectivity",
				Flags: map[string]string{
					"fix-issues": "Automatically attempt to fix detected issues",
					"verbose":    "Show detailed diagnostic information",
				},
			},
		},
	}

	return &APTInstaller{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "apt"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewAPTPlugin()

	// Handle args with potential panic recovery and diagnostic information
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "APT plugin panic recovered: %v\n", r)
			fmt.Fprintf(os.Stderr, "Plugin version: %s\n", version)
			os.Exit(1)
		}
	}()

	sdk.HandleArgs(plugin, os.Args[1:])
}
