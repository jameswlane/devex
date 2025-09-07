package main

// Build timestamp: 2025-09-06

import (
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// ShellPlugin implements the Shell configuration plugin
type ShellPlugin struct {
	*sdk.BasePlugin
}

// NewShellPlugin creates a new Shell plugin
func NewShellPlugin() *ShellPlugin {
	info := sdk.PluginInfo{
		Name:        "tool-shell",
		Version:     version,
		Description: "Shell configuration and management for bash, zsh, and fish shells",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"shell", "bash", "zsh", "fish", "configuration", "dotfiles"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "setup",
				Description: "Set up shell configuration",
				Usage:       "Initialize shell environment with sensible defaults and configurations",
			},
			{
				Name:        "switch",
				Description: "Switch between shells",
				Usage:       "Change default shell to bash, zsh, or fish with validation",
				Flags: map[string]string{
					"shell": "Target shell (bash, zsh, fish)",
				},
			},
			{
				Name:        "config",
				Description: "Configure shell settings",
				Usage:       "View and configure shell aliases, functions, and preferences",
			},
			{
				Name:        "backup",
				Description: "Backup shell configurations",
				Usage:       "Create timestamped backup of shell configuration files",
			},
		},
	}

	return &ShellPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

func main() {
	plugin := NewShellPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
