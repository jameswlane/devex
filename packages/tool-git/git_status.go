package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// HandleStatus displays the current Git configuration and version information
func (p *GitPlugin) HandleStatus(ctx context.Context, args []string) error {
	fmt.Println("Git Configuration Status:")

	// Show Git version
	if err := p.ShowGitVersion(); err != nil {
		return err
	}

	// Show user configuration
	p.ShowUserConfig()

	// Show key configuration settings
	p.ShowKeyConfigs()

	// Show installed aliases count
	p.ShowAliasesCount()

	return nil
}

// ShowGitVersion displays the installed Git version
func (p *GitPlugin) ShowGitVersion() error {
	output, err := sdk.RunCommand("git", "--version")
	if err != nil {
		return fmt.Errorf("failed to get git version: %w", err)
	}
	fmt.Printf("Version: %s\n", strings.TrimSpace(output))
	return nil
}

// ShowUserConfig displays the current username and email configuration
func (p *GitPlugin) ShowUserConfig() {
	// Show user name
	if user := p.GetCurrentConfig("user.name"); user != "" {
		fmt.Printf("User Name: %s\n", user)
	} else {
		fmt.Println("User Name: Not configured")
	}

	// Show user email
	if email := p.GetCurrentConfig("user.email"); email != "" {
		fmt.Printf("User Email: %s\n", email)
	} else {
		fmt.Println("User Email: Not configured")
	}
}

// ShowKeyConfigs displays important Git configuration settings
func (p *GitPlugin) ShowKeyConfigs() {
	keyConfigs := []string{
		"init.defaultBranch",
		"core.editor",
		"color.ui",
		"pull.rebase",
		"push.default",
	}

	fmt.Println("\nKey Configuration Settings:")
	for _, key := range keyConfigs {
		if value := p.GetCurrentConfig(key); value != "" {
			fmt.Printf("  %s: %s\n", key, value)
		} else {
			fmt.Printf("  %s: Not set\n", key)
		}
	}
}

// ShowAliasesCount displays the number of configured Git aliases
func (p *GitPlugin) ShowAliasesCount() {
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
