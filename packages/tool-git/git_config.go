package main

import (
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleConfig configures Git settings including user info and sensible defaults
func (p *GitPlugin) handleConfig(args []string) error {
	fmt.Println("Configuring Git...")

	// Parse command line arguments for name and email
	fullName, email := p.parseConfigArgs(args)

	// Get current configuration or use provided values
	if fullName == "" {
		if currentName := p.getCurrentConfig("user.name"); currentName != "" {
			fmt.Printf("Current user name: %s\n", currentName)
			fullName = currentName
		} else {
			fmt.Println("No git user name configured")
			if len(args) == 0 {
				fmt.Println("Use: git config --name \"Your Name\" --email \"your@email.com\"")
				return nil
			}
		}
	}

	if email == "" {
		if currentEmail := p.getCurrentConfig("user.email"); currentEmail != "" {
			fmt.Printf("Current user email: %s\n", currentEmail)
			email = currentEmail
		} else {
			fmt.Println("No git user email configured")
			if len(args) == 0 {
				fmt.Println("Use: git config --name \"Your Name\" --email \"your@email.com\"")
				return nil
			}
		}
	}

	// Set user configuration
	if err := p.setUserConfig(fullName, email); err != nil {
		return err
	}

	// Set sensible defaults
	if err := p.setSensibleDefaults(); err != nil {
		return err
	}

	fmt.Println("\nGit configuration complete!")
	return nil
}

// parseConfigArgs parses command line arguments for name and email
func (p *GitPlugin) parseConfigArgs(args []string) (string, string) {
	var fullName, email string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name":
			if i+1 < len(args) {
				fullName = args[i+1]
				i++
			}
		case "--email":
			if i+1 < len(args) {
				email = args[i+1]
				i++
			}
		}
	}

	return fullName, email
}

// getCurrentConfig gets the current value of a git configuration key
func (p *GitPlugin) getCurrentConfig(key string) string {
	output, err := sdk.RunCommand("git", "config", "--global", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

// setUserConfig sets the Git user name and email
func (p *GitPlugin) setUserConfig(fullName, email string) error {
	if fullName != "" {
		if err := sdk.ExecCommand(false, "git", "config", "--global", "user.name", fullName); err != nil {
			return fmt.Errorf("failed to set git user name: %w", err)
		}
		fmt.Printf("Set git user name: %s\n", fullName)
	}

	if email != "" {
		if err := sdk.ExecCommand(false, "git", "config", "--global", "user.email", email); err != nil {
			return fmt.Errorf("failed to set git user email: %w", err)
		}
		fmt.Printf("Set git user email: %s\n", email)
	}

	return nil
}

// setSensibleDefaults sets recommended Git configuration defaults
func (p *GitPlugin) setSensibleDefaults() error {
	// Set default branch name
	if err := sdk.ExecCommand(false, "git", "config", "--global", "init.defaultBranch", "main"); err != nil {
		fmt.Printf("Warning: failed to set default branch name: %v\n", err)
	} else {
		fmt.Println("Set default branch name to 'main'")
	}

	// Define sensible defaults
	configs := map[string]string{
		"core.editor":         "vim",
		"color.ui":            "auto",
		"pull.rebase":         "false",
		"push.default":        "simple",
		"credential.helper":   "cache --timeout=3600",
		"merge.conflictstyle": "diff3",
		"diff.colorMoved":     "default",
		"fetch.prune":         "true",
	}

	// Apply each configuration setting
	for key, value := range configs {
		if err := sdk.ExecCommand(false, "git", "config", "--global", key, value); err != nil {
			fmt.Printf("Warning: failed to set %s: %v\n", key, err)
		}
	}

	return nil
}