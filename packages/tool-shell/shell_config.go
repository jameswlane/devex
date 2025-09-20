package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// handleConfig displays and manages current shell configuration settings
func (p *ShellPlugin) handleConfig(ctx context.Context, args []string) error {
	fmt.Println("Shell Configuration Status:")

	currentShell := p.DetectCurrentShell()
	if currentShell == "unknown" {
		return fmt.Errorf("could not detect current shell")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Show current shell information
	fmt.Printf("Current shell: %s\n", currentShell)

	// Get configuration file path
	rcFile := p.GetShellConfigFile(currentShell, homeDir)
	if rcFile == "" {
		return fmt.Errorf("unsupported shell: %s", currentShell)
	}

	fmt.Printf("Configuration file: %s\n", rcFile)

	// Check if a configuration file exists
	if err := p.checkConfigurationStatus(rcFile); err != nil {
		return err
	}

	return nil
}

// checkConfigurationStatus checks and reports the status of shell configuration
func (p *ShellPlugin) checkConfigurationStatus(rcFile string) error {
	// Check if configuration exists
	if _, err := os.Stat(rcFile); os.IsNotExist(err) {
		fmt.Printf("❌ Configuration file does not exist: %s\n", rcFile)
		fmt.Println("💡 Run 'shell setup' to create it with DevEx configurations")
		return nil
	}

	fmt.Printf("✅ Configuration file exists: %s\n", rcFile)

	// Check for DevEx configurations
	content, err := os.ReadFile(rcFile)
	if err != nil {
		return fmt.Errorf("failed to read configuration file: %w", err)
	}

	if strings.Contains(string(content), "# DevEx Shell Configuration") {
		fmt.Println("✅ DevEx configurations are present")
		p.showDevExConfigurationDetails(string(content))
	} else {
		fmt.Println("❌ No DevEx configurations found")
		fmt.Println("💡 Run 'shell setup' to add them")
	}

	return nil
}

// showDevExConfigurationDetails shows what DevEx configurations are active
func (p *ShellPlugin) showDevExConfigurationDetails(content string) {
	fmt.Println("\nDevEx Configuration Features:")

	features := map[string][]string{
		"History Settings": {
			"HISTSIZE", "HISTFILESIZE", "HISTCONTROL", "SAVEHIST",
			"HIST_IGNORE_DUPS", "HIST_IGNORE_SPACE", "SHARE_HISTORY",
		},
		"Color Support": {
			"CLICOLOR", "colors", "fish_color_command", "fish_color_error",
		},
		"Useful Aliases": {
			"alias ll=", "alias la=", "alias l=", "alias ..=", "alias ...=",
		},
	}

	for featureName, keywords := range features {
		hasFeature := false
		for _, keyword := range keywords {
			if strings.Contains(content, keyword) {
				hasFeature = true
				break
			}
		}

		if hasFeature {
			fmt.Printf("  ✅ %s\n", featureName)
		} else {
			fmt.Printf("  ❌ %s\n", featureName)
		}
	}
}
