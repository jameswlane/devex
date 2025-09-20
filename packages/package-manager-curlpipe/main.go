package main

// Build timestamp: 2025-09-06

import (
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewCurlpipePlugin creates a new Curl Pipe plugin
func NewCurlpipePlugin() *CurlpipePlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-curlpipe",
		Version:     version,
		Description: "Direct download and installation via curl with security validation",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "curl", "download", "script", "installation"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Execute installation scripts from trusted URLs",
				Usage:       "Execute installation scripts with security validation",
				Flags: map[string]string{
					"dry-run": "Show what would be executed without running it",
					"force":   "Skip domain validation (use with caution)",
				},
			},
			{
				Name:        "remove",
				Description: "Remove tracked installations (limited support)",
				Usage:       "Remove installations tracked by this plugin",
			},
			{
				Name:        "validate-url",
				Description: "Validate if URL is from a trusted domain",
				Usage:       "Check if a URL is safe for curl pipe installation",
			},
			{
				Name:        "list-trusted",
				Description: "List trusted domains",
				Usage:       "Show list of domains trusted for curl pipe installations",
			},
			{
				Name:        "preview",
				Description: "Preview installation script",
				Usage:       "Download and preview script before execution",
			},
		},
	}

	return &CurlpipePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "curl"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewCurlpipePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
