package main

// Build timestamp: 2025-09-06

import (
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewMisePlugin creates a new Mise plugin
func NewMisePlugin() *MisePlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-mise",
		Version:     version,
		Description: "Mise development tool version manager with multi-language support",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "mise", "tools", "development", "version-manager"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install development tools using Mise",
				Usage:       "Install language versions with 'language@version' format (e.g., node@18, python@3.11)",
				Flags: map[string]string{
					"global": "Set as global version (default)",
					"local":  "Set as local version for current directory",
				},
			},
			{
				Name:        "remove",
				Description: "Remove development tools using Mise",
				Usage:       "Remove language versions from system",
			},
			{
				Name:        "update",
				Description: "Update Mise plugins and tool versions",
				Usage:       "Update Mise plugins and installed tools",
			},
			{
				Name:        "search",
				Description: "Search for available tools",
				Usage:       "Search for available development tools and versions",
			},
			{
				Name:        "list",
				Description: "List installed tools",
				Usage:       "List installed development tools and their versions",
				Flags: map[string]string{
					"all":      "Show all available versions for each tool",
					"current":  "Show only currently active versions",
					"outdated": "Show outdated tools that can be updated",
				},
			},
			{
				Name:        "ensure-installed",
				Description: "Ensure Mise is installed on the system",
				Usage:       "Install Mise using the official installation script",
			},
			{
				Name:        "is-installed",
				Description: "Check if a tool is installed",
				Usage:       "Returns exit code 0 if tool is installed, 1 if not",
			},
		},
	}

	return &MisePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "mise"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewMisePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
