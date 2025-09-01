package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// GitPlugin implements the Git configuration plugin
type GitPlugin struct {
	*sdk.BasePlugin
}

// NewGitPlugin creates a new Git plugin
func NewGitPlugin() *GitPlugin {
	info := sdk.PluginInfo{
		Name:        "tool-git",
		Version:     version,
		Description: "Git configuration and alias management",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"git", "vcs", "development"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "config",
				Description: "Configure Git settings",
				Usage:       "Set up Git user, email, and global configuration",
			},
			{
				Name:        "aliases",
				Description: "Install Git aliases",
				Usage:       "Install useful Git aliases from configuration",
			},
			{
				Name:        "status",
				Description: "Show Git configuration status",
				Usage:       "Display current Git configuration",
			},
		},
	}

	return &GitPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *GitPlugin) Execute(command string, args []string) error {
	// Ensure git is available
	if !sdk.CommandExists("git") {
		return fmt.Errorf("git is not installed on this system")
	}

	switch command {
	case "config":
		return p.handleConfig(args)
	case "aliases":
		return p.handleAliases(args)
	case "status":
		return p.handleStatus(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *GitPlugin) handleConfig(args []string) error {
	fmt.Println("Configuring Git...")
	
	// TODO: Implement Git configuration from moved gitconfig package
	return fmt.Errorf("git configuration not yet implemented in plugin")
}

func (p *GitPlugin) handleAliases(args []string) error {
	fmt.Println("Installing Git aliases...")
	
	// TODO: Implement Git alias installation
	return fmt.Errorf("git aliases not yet implemented in plugin")
}

func (p *GitPlugin) handleStatus(args []string) error {
	fmt.Println("Git Configuration Status:")
	
	// Show git version
	output, err := sdk.RunCommand("git", "--version")
	if err != nil {
		return fmt.Errorf("failed to get git version: %w", err)
	}
	fmt.Printf("Version: %s\n", strings.TrimSpace(output))
	
	// Show user configuration
	user, err := sdk.RunCommand("git", "config", "--global", "user.name")
	if err == nil {
		fmt.Printf("User Name: %s\n", strings.TrimSpace(user))
	}
	
	email, err := sdk.RunCommand("git", "config", "--global", "user.email")
	if err == nil {
		fmt.Printf("User Email: %s\n", strings.TrimSpace(email))
	}
	
	return nil
}

func main() {
	plugin := NewGitPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
