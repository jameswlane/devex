package main

// Build timestamp: 2025-09-06

import (
	"fmt"
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewFlatpakPlugin creates a new Flatpak plugin
func NewFlatpakPlugin() *FlatpakInstaller {
	info := sdk.PluginInfo{
		Name:        "package-manager-flatpak",
		Version:     version,
		Description: "Flatpak universal package manager with system installation support",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "flatpak", "universal", "linux", "sandboxed"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install Flatpak applications",
				Usage:       "Install applications from configured remotes",
				Flags: map[string]string{
					"user":   "Install for current user only (default)",
					"system": "Install system-wide",
					"remote": "Specify remote to install from",
					"yes":    "Automatically answer yes to prompts",
				},
			},
			{
				Name:        "remove",
				Description: "Remove Flatpak applications",
				Usage:       "Remove installed applications",
				Flags: map[string]string{
					"unused": "Remove unused runtimes after removal",
				},
			},
			{
				Name:        "update",
				Description: "Update installed applications and runtimes",
				Usage:       "Update all installed applications and runtimes",
			},
			{
				Name:        "search",
				Description: "Search for applications",
				Usage:       "Search for applications across all configured remotes",
			},
			{
				Name:        "list",
				Description: "List installed applications",
				Usage:       "Show installed applications and runtimes",
				Flags: map[string]string{
					"app":     "List only applications",
					"runtime": "List only runtimes",
				},
			},
			{
				Name:        "remote-add",
				Description: "Add a new remote repository",
				Usage:       "Add remote repository for applications",
				Flags: map[string]string{
					"if-not-exists": "Don't fail if remote already exists",
					"gpg-import":    "Import GPG key for remote",
				},
			},
			{
				Name:        "remote-remove",
				Description: "Remove a remote repository",
				Usage:       "Remove remote repository",
			},
			{
				Name:        "remote-list",
				Description: "List configured remotes",
				Usage:       "Show all configured remote repositories",
			},
			{
				Name:        "is-installed",
				Description: "Check if application is installed",
				Usage:       "Returns exit code 0 if installed, 1 if not",
			},
			{
				Name:        "info",
				Description: "Show application information",
				Usage:       "Display detailed information about an application",
			},
			{
				Name:        "ensure-installed",
				Description: "Install Flatpak system-wide if not present",
				Usage:       "Install Flatpak package manager on systems that don't have it",
			},
			{
				Name:        "add-flathub",
				Description: "Add Flathub repository",
				Usage:       "Add the main Flathub repository for applications",
				Flags: map[string]string{
					"user":   "Add for current user only",
					"system": "Add system-wide",
				},
			},
		},
	}

	return &FlatpakInstaller{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "flatpak"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewFlatpakPlugin()

	// Handle args with panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Plugin panic recovered: %v\n", r)
			os.Exit(1)
		}
	}()

	sdk.HandleArgs(plugin, os.Args[1:])
}
