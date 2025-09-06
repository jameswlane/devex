package main

import (
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleStatus displays current Git configuration and version information
func (p *GitPlugin) handleStatus(args []string) error {
	fmt.Println("Git Configuration Status:")

	// Show Git version
	if err := p.showGitVersion(); err != nil {
		return err
	}

	// Show user configuration
	p.showUserConfig()

	// Show key configuration settings
	p.showKeyConfigs()

	// Show installed aliases count
	p.showAliasesCount()

	return nil
}

// showGitVersion displays the installed Git version
func (p *GitPlugin) showGitVersion() error {
	output, err := sdk.RunCommand("git", "--version")
	if err != nil {
		return fmt.Errorf("failed to get git version: %w", err)
	}
	fmt.Printf("Version: %s\n", strings.TrimSpace(output))
	return nil
}

// showUserConfig displays the current user name and email configuration
func (p *GitPlugin) showUserConfig() {
	// Show user name
	if user := p.getCurrentConfig("user.name"); user != "" {
		fmt.Printf("User Name: %s\n", user)
	} else {
		fmt.Println("User Name: Not configured")
	}

	// Show user email
	if email := p.getCurrentConfig("user.email"); email != "" {
		fmt.Printf("User Email: %s\n", email)
	} else {
		fmt.Println("User Email: Not configured")
	}
}

// showKeyConfigs displays important Git configuration settings
func (p *GitPlugin) showKeyConfigs() {
	keyConfigs := []string{
		"init.defaultBranch",
		"core.editor",
		"color.ui",
		"pull.rebase",
		"push.default",
	}

	fmt.Println("\nKey Configuration Settings:")
	for _, key := range keyConfigs {
		if value := p.getCurrentConfig(key); value != "" {
			fmt.Printf("  %s: %s\n", key, value)
		} else {
			fmt.Printf("  %s: Not set\n", key)
		}
	}
}

// showAliasesCount displays the number of configured Git aliases
func (p *GitPlugin) showAliasesCount() {
	output, err := sdk.RunCommand("git", "config", "--global", "--get-regexp", "^alias\\.")
	if err != nil {
		fmt.Println("\nAliases: None configured")
		return
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	aliasCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			aliasCount++
		}
	}

	fmt.Printf("\nAliases: %d configured\n", aliasCount)
	if aliasCount > 0 {
		fmt.Println("  Use 'git la' to list all aliases")
	}
}