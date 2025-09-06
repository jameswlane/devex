package main

// Build timestamp: 2025-09-06

import (
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// PipPlugin implements the Pip package manager
type PipPlugin struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// NewPipPlugin creates a new Pip plugin
func NewPipPlugin() *PipPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-pip",
		Version:     version,
		Description: "Python package installer with virtual environment support",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "pip", "python", "packages", "virtual-environment"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install Python packages using Pip",
				Usage:       "Install packages with virtual environment detection",
				Flags: map[string]string{
					"user":         "Install to user directory",
					"system":       "Force system-wide installation",
					"requirements": "Install from requirements.txt file",
					"upgrade":      "Upgrade packages to latest versions",
				},
			},
			{
				Name:        "remove",
				Description: "Remove Python packages using Pip",
				Usage:       "Remove packages with dependency cleanup",
				Flags: map[string]string{
					"yes": "Automatically confirm removal",
				},
			},
			{
				Name:        "update",
				Description: "Update installed packages",
				Usage:       "Update pip and installed packages to latest versions",
				Flags: map[string]string{
					"all": "Update all installed packages",
				},
			},
			{
				Name:        "search",
				Description: "Search for Python packages",
				Usage:       "Search PyPI for packages (note: search may be limited)",
			},
			{
				Name:        "list",
				Description: "List installed packages",
				Usage:       "List installed Python packages",
				Flags: map[string]string{
					"outdated": "Show only outdated packages",
					"format":   "Output format (columns, freeze, json)",
				},
			},
			{
				Name:        "is-installed",
				Description: "Check if a package is installed",
				Usage:       "Returns exit code 0 if package is installed, 1 if not",
			},
			{
				Name:        "create-venv",
				Description: "Create a virtual environment",
				Usage:       "Create a Python virtual environment in current directory",
				Flags: map[string]string{
					"name": "Virtual environment name (default: venv)",
				},
			},
			{
				Name:        "freeze",
				Description: "Generate requirements.txt",
				Usage:       "Generate requirements.txt from installed packages",
			},
		},
	}

	return &PipPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "pip"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewPipPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
