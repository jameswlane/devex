package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
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
		Description: "Shell configuration and management",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"shell", "bash", "zsh", "fish"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "setup",
				Description: "Set up shell configuration",
				Usage:       "Initialize shell environment with dotfiles and configurations",
			},
			{
				Name:        "switch",
				Description: "Switch between shells",
				Usage:       "Change default shell to bash, zsh, or fish",
			},
			{
				Name:        "config",
				Description: "Configure shell settings",
				Usage:       "Configure aliases, functions, and shell preferences",
			},
			{
				Name:        "backup",
				Description: "Backup shell configurations",
				Usage:       "Create backup of current shell configuration files",
			},
		},
	}

	return &ShellPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *ShellPlugin) Execute(command string, args []string) error {
	switch command {
	case "setup":
		return p.handleSetup(args)
	case "switch":
		return p.handleSwitch(args)
	case "config":
		return p.handleConfig(args)
	case "backup":
		return p.handleBackup(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *ShellPlugin) handleSetup(args []string) error {
	fmt.Println("Setting up shell configuration...")
	
	// Detect current shell
	currentShell := p.detectCurrentShell()
	fmt.Printf("Current shell: %s\n", currentShell)
	
	// TODO: Implement shell configuration setup
	return fmt.Errorf("shell setup not yet implemented in plugin")
}

func (p *ShellPlugin) handleSwitch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("shell switch requires a target shell (bash, zsh, fish)")
	}
	
	targetShell := args[0]
	fmt.Printf("Switching to %s shell...\n", targetShell)
	
	// TODO: Implement shell switching functionality
	return fmt.Errorf("shell switching not yet implemented in plugin")
}

func (p *ShellPlugin) handleConfig(args []string) error {
	fmt.Println("Configuring shell settings...")
	
	// TODO: Implement shell configuration management
	return fmt.Errorf("shell configuration not yet implemented in plugin")
}

func (p *ShellPlugin) handleBackup(args []string) error {
	fmt.Println("Backing up shell configuration...")
	
	// TODO: Implement shell configuration backup
	return fmt.Errorf("shell backup not yet implemented in plugin")
}

func (p *ShellPlugin) detectCurrentShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}
	
	// Extract shell name from path
	parts := strings.Split(shell, "/")
	return parts[len(parts)-1]
}

func main() {
	plugin := NewShellPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
