package main

// Build timestamp: 2025-09-06

import (
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewAppimagePlugin creates a new AppImage plugin
func NewAppimagePlugin() *AppimagePlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-appimage",
		Version:     version,
		Description: "AppImage package manager for Linux with desktop integration",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "appimage", "linux", "portable", "gui"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install AppImage applications",
				Usage:       "Download and install AppImage with format: '<download_url> <binary_name>'",
				Flags: map[string]string{
					"gui": "Install to ~/Applications for GUI apps (default)",
					"cli": "Install to ~/.local/bin for CLI tools",
				},
			},
			{
				Name:        "remove",
				Description: "Remove AppImage applications",
				Usage:       "Remove installed AppImage binaries and desktop entries",
			},
			{
				Name:        "list",
				Description: "List installed AppImages",
				Usage:       "List AppImages installed in ~/Applications and ~/.local/bin",
			},
			{
				Name:        "is-installed",
				Description: "Check if an AppImage is installed",
				Usage:       "Returns exit code 0 if AppImage is installed, 1 if not",
			},
			{
				Name:        "validate-url",
				Description: "Validate AppImage download URL",
				Usage:       "Check if URL is accessible and points to valid AppImage",
			},
		},
	}

	return &AppimagePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "AppImage"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewAppimagePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
