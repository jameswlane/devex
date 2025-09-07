package main

// Build timestamp: 2025-09-06

import (
	"fmt"
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewDebPlugin creates a new DEB plugin
func NewDebPlugin() *DebInstaller {
	info := sdk.PluginInfo{
		Name:        "package-manager-deb",
		Version:     version,
		Description: "Debian package (.deb) installer for local and remote packages",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "deb", "debian", "ubuntu", "dpkg"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install .deb packages from file or URL",
				Usage:       "Install one or more .deb packages with automatic dependency resolution",
				Flags: map[string]string{
					"force":         "Force installation even if dependencies are missing",
					"no-deps":       "Skip dependency installation",
					"download-only": "Download packages without installing",
					"target-dir":    "Directory to download packages to",
				},
			},
			{
				Name:        "remove",
				Description: "Remove installed packages",
				Usage:       "Remove packages installed via dpkg",
				Flags: map[string]string{
					"purge": "Remove packages and their configuration files",
				},
			},
			{
				Name:        "info",
				Description: "Show information about a .deb package",
				Usage:       "Display detailed information about a .deb file",
			},
			{
				Name:        "list-files",
				Description: "List files in a .deb package",
				Usage:       "Show all files that will be installed by a .deb package",
			},
			{
				Name:        "verify",
				Description: "Verify package integrity and dependencies",
				Usage:       "Check if a .deb package can be installed and list missing dependencies",
			},
			{
				Name:        "is-installed",
				Description: "Check if a package is installed",
				Usage:       "Returns exit code 0 if package is installed, 1 if not",
			},
			{
				Name:        "extract",
				Description: "Extract .deb package contents without installing",
				Usage:       "Extract package contents to a specified directory",
				Flags: map[string]string{
					"target-dir": "Directory to extract files to",
				},
			},
		},
	}

	return &DebInstaller{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "dpkg"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewDebPlugin()

	// Handle args with panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Plugin panic recovered: %v\n", r)
			os.Exit(1)
		}
	}()

	sdk.HandleArgs(plugin, os.Args[1:])
}
