package main

// Build timestamp: 2025-09-06

import (
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewGitPlugin creates a new Git plugin
func NewGitPlugin() *GitPlugin {
	info := sdk.PluginInfo{
		Name:        "tool-git",
		Version:     version,
		Description: "Git configuration and alias management for development workflows",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"git", "vcs", "development", "configuration", "aliases"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "config",
				Description: "Configure Git settings",
				Usage:       "Set up Git user, email, and global configuration with sensible defaults",
				Flags: map[string]string{
					"name":  "Set Git user name",
					"email": "Set Git user email",
				},
			},
			{
				Name:        "aliases",
				Description: "Install Git aliases",
				Usage:       "Install useful Git aliases to improve workflow efficiency",
			},
			{
				Name:        "status",
				Description: "Show Git configuration status",
				Usage:       "Display current Git configuration and version information",
			},
		},
	}

	return &GitPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

func main() {
	plugin := NewGitPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
