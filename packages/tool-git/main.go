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
			{Name: "ssh", Description: "Preview SSH key generation or display an existing public key", Usage: "ssh --mode generate|existing --path /absolute/key [--apply]"},
			{
				Name:        "config",
				Description: "Configure Git settings",
				Usage:       "Set Git user and email; optionally apply missing recommended global settings with --defaults",
				Flags: map[string]string{
					"name":     "Set Git user name",
					"email":    "Set Git user email",
					"defaults": "Apply recommended global settings without overwriting existing values",
				},
			},
			{
				Name:        "aliases",
				Description: "Install Git aliases",
				Usage:       "Install missing recommended Git aliases only with --defaults",
				Flags: map[string]string{
					"defaults": "Install recommended aliases without overwriting existing values",
				},
			},
			{
				Name:        "status",
				Description: "Show Git configuration status",
				Usage:       "Display current Git configuration and version information",
			},
			{
				Name:        "setup",
				Description: "Configure Git via setup protocol",
				Usage:       "Configure Git user information during setup; --defaults also applies missing recommended settings",
				Flags: map[string]string{
					"defaults": "Apply recommended global settings without overwriting existing values",
				},
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
