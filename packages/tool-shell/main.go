package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine shell RC file
	var rcFile string
	switch currentShell {
	case "bash":
		rcFile = filepath.Join(homeDir, ".bashrc")
	case "zsh":
		rcFile = filepath.Join(homeDir, ".zshrc")
	case "fish":
		rcFile = filepath.Join(homeDir, ".config", "fish", "config.fish")
	default:
		return fmt.Errorf("unsupported shell: %s", currentShell)
	}

	// Check if RC file exists, create if not
	if _, err := os.Stat(rcFile); os.IsNotExist(err) {
		// Create parent directory for fish
		if currentShell == "fish" {
			fishDir := filepath.Dir(rcFile)
			if err := os.MkdirAll(fishDir, 0755); err != nil {
				return fmt.Errorf("failed to create fish config directory: %w", err)
			}
		}

		// Create the RC file
		if err := os.WriteFile(rcFile, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create RC file: %w", err)
		}
		fmt.Printf("Created %s\n", rcFile)
	}

	// Add basic shell configuration
	configs := p.getShellConfigs(currentShell)

	// Read existing content
	content, err := os.ReadFile(rcFile)
	if err != nil {
		return fmt.Errorf("failed to read RC file: %w", err)
	}

	// Add configurations that don't already exist
	updated := false
	existingContent := string(content)
	newContent := existingContent

	// Add DevEx section marker if not present
	devexMarker := "# DevEx Shell Configuration"
	if !strings.Contains(existingContent, devexMarker) {
		newContent += "\n" + devexMarker + "\n"
		updated = true
	}

	// Add each configuration if not present
	for _, config := range configs {
		if !strings.Contains(existingContent, config) {
			newContent += config + "\n"
			updated = true
		}
	}

	// Write back if updated
	if updated {
		if err := os.WriteFile(rcFile, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to update RC file: %w", err)
		}
		fmt.Printf("Updated %s with DevEx configurations\n", rcFile)
	} else {
		fmt.Println("Shell configuration already up to date")
	}

	return nil
}

func (p *ShellPlugin) handleSwitch(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("shell switch requires a target shell (bash, zsh, fish)")
	}

	targetShell := args[0]
	fmt.Printf("Switching to %s shell...\n", targetShell)

	// Validate target shell
	validShells := []string{"bash", "zsh", "fish"}
	isValid := false
	for _, shell := range validShells {
		if targetShell == shell {
			isValid = true
			break
		}
	}

	if !isValid {
		return fmt.Errorf("invalid shell: %s. Valid options are: bash, zsh, fish", targetShell)
	}

	// Check if target shell is installed
	shellPath, err := sdk.ExecCommandOutput("which", targetShell)
	if err != nil {
		return fmt.Errorf("shell %s is not installed", targetShell)
	}
	shellPath = strings.TrimSpace(shellPath)

	// Change default shell
	fmt.Printf("Changing default shell to %s...\n", shellPath)
	if err := sdk.ExecCommand(true, "chsh", "-s", shellPath); err != nil {
		return fmt.Errorf("failed to change shell: %w", err)
	}

	fmt.Printf("Successfully switched to %s shell\n", targetShell)
	fmt.Println("Please log out and log back in for the change to take effect")

	return nil
}

func (p *ShellPlugin) handleConfig(args []string) error {
	fmt.Println("Configuring shell settings...")

	currentShell := p.detectCurrentShell()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Show current shell configuration
	fmt.Printf("Current shell: %s\n", currentShell)

	var rcFile string
	switch currentShell {
	case "bash":
		rcFile = filepath.Join(homeDir, ".bashrc")
	case "zsh":
		rcFile = filepath.Join(homeDir, ".zshrc")
	case "fish":
		rcFile = filepath.Join(homeDir, ".config", "fish", "config.fish")
	default:
		return fmt.Errorf("unsupported shell: %s", currentShell)
	}

	fmt.Printf("Configuration file: %s\n", rcFile)

	// Check if configuration exists
	if _, err := os.Stat(rcFile); os.IsNotExist(err) {
		fmt.Println("Configuration file does not exist. Run 'setup' to create it.")
		return nil
	}

	// Show DevEx configurations
	content, err := os.ReadFile(rcFile)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	if strings.Contains(string(content), "# DevEx Shell Configuration") {
		fmt.Println("\nDevEx configurations are present in your shell configuration.")
	} else {
		fmt.Println("\nNo DevEx configurations found. Run 'setup' to add them.")
	}

	return nil
}

func (p *ShellPlugin) handleBackup(args []string) error {
	fmt.Println("Backing up shell configuration...")

	currentShell := p.detectCurrentShell()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine files to backup
	var filesToBackup []string
	switch currentShell {
	case "bash":
		filesToBackup = []string{
			filepath.Join(homeDir, ".bashrc"),
			filepath.Join(homeDir, ".bash_profile"),
			filepath.Join(homeDir, ".profile"),
		}
	case "zsh":
		filesToBackup = []string{
			filepath.Join(homeDir, ".zshrc"),
			filepath.Join(homeDir, ".zprofile"),
			filepath.Join(homeDir, ".zshenv"),
		}
	case "fish":
		filesToBackup = []string{
			filepath.Join(homeDir, ".config", "fish", "config.fish"),
			filepath.Join(homeDir, ".config", "fish", "functions"),
		}
	default:
		return fmt.Errorf("unsupported shell: %s", currentShell)
	}

	// Create backup directory
	backupDir := filepath.Join(homeDir, ".devex", "backups", "shell")
	timestamp := fmt.Sprintf("%d", os.Getpid())
	backupPath := filepath.Join(backupDir, timestamp)

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Backup each file
	backedUp := 0
	for _, file := range filesToBackup {
		if _, err := os.Stat(file); err == nil {
			// File exists, backup it
			content, err := os.ReadFile(file)
			if err != nil {
				fmt.Printf("Warning: failed to read %s: %v\n", file, err)
				continue
			}

			backupFile := filepath.Join(backupPath, filepath.Base(file))
			if err := os.WriteFile(backupFile, content, 0644); err != nil {
				fmt.Printf("Warning: failed to backup %s: %v\n", file, err)
				continue
			}

			fmt.Printf("Backed up %s\n", file)
			backedUp++
		}
	}

	if backedUp > 0 {
		fmt.Printf("\nBackup completed: %d files backed up to %s\n", backedUp, backupPath)
	} else {
		fmt.Println("\nNo files to backup")
	}

	return nil
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

// getShellConfigs returns shell-specific configurations
func (p *ShellPlugin) getShellConfigs(shell string) []string {
	switch shell {
	case "bash":
		return []string{
			"# Enable color prompt",
			"export CLICOLOR=1",
			"# Better history settings",
			"export HISTSIZE=10000",
			"export HISTFILESIZE=20000",
			"export HISTCONTROL=ignoreboth:erasedups",
			"# Useful aliases",
			"alias ll='ls -la'",
			"alias la='ls -A'",
			"alias l='ls -CF'",
			"alias ..='cd ..'",
			"alias ...='cd ../..'",
		}
	case "zsh":
		return []string{
			"# Enable colors",
			"autoload -U colors && colors",
			"# Better history settings",
			"HISTSIZE=10000",
			"SAVEHIST=20000",
			"setopt HIST_IGNORE_DUPS",
			"setopt HIST_IGNORE_SPACE",
			"setopt SHARE_HISTORY",
			"# Useful aliases",
			"alias ll='ls -la'",
			"alias la='ls -A'",
			"alias l='ls -CF'",
			"alias ..='cd ..'",
			"alias ...='cd ../..'",
		}
	case "fish":
		return []string{
			"# Fish color settings",
			"set -g fish_color_command green",
			"set -g fish_color_error red",
			"set -g fish_color_param cyan",
			"# Useful aliases",
			"alias ll 'ls -la'",
			"alias la 'ls -A'",
			"alias l 'ls -CF'",
			"alias .. 'cd ..'",
			"alias ... 'cd ../..'",
		}
	default:
		return []string{}
	}
}

func main() {
	plugin := NewShellPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
