package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// handleSetup initializes shell configuration with sensible defaults
func (p *ShellPlugin) handleSetup(ctx context.Context, args []string) error {
	fmt.Println("Setting up shell configuration...")

	// Detect current shell
	currentShell := p.DetectCurrentShell()
	fmt.Printf("Current shell: %s\n", currentShell)

	if currentShell == "unknown" {
		return fmt.Errorf("could not detect current shell")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Get shell configuration file path
	rcFile := p.GetShellConfigFile(currentShell, homeDir)
	if rcFile == "" {
		return fmt.Errorf("unsupported shell: %s", currentShell)
	}

	// Create an RC file if it doesn't exist
	if err := p.createShellConfigFile(currentShell, rcFile); err != nil {
		return err
	}

	// Add DevEx configurations
	if err := p.addShellConfigurations(currentShell, rcFile); err != nil {
		return err
	}

	return nil
}

// GetShellConfigFile returns the configuration file path for the given shell
func (p *ShellPlugin) GetShellConfigFile(shell, homeDir string) string {
	switch shell {
	case "bash":
		return filepath.Join(homeDir, ".bashrc")
	case "zsh":
		return filepath.Join(homeDir, ".zshrc")
	case "fish":
		return filepath.Join(homeDir, ".config", "fish", "config.fish")
	default:
		return ""
	}
}

// createShellConfigFile creates the shell configuration file if it doesn't exist
func (p *ShellPlugin) createShellConfigFile(shell, rcFile string) error {
	if _, err := os.Stat(rcFile); os.IsNotExist(err) {
		// Create a parent directory for fish
		if shell == "fish" {
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
	return nil
}

// addShellConfigurations adds DevEx configurations to the shell RC file
func (p *ShellPlugin) addShellConfigurations(shell, rcFile string) error {
	// Get shell-specific configurations
	configs := p.GetShellConfigs(shell)

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

// GetShellConfigs returns shell-specific configurations
func (p *ShellPlugin) GetShellConfigs(shell string) []string {
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
