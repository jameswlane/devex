package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// HandleConfig configures Git user info and explicitly requested defaults
func (p *GitPlugin) HandleConfig(ctx context.Context, args []string) error {
	fmt.Println("Configuring Git...")

	// Parse command line arguments for name and email
	fullName, email := p.ParseConfigArgs(args)

	// Get current configuration or use provided values
	if fullName == "" {
		if currentName := p.GetCurrentConfig("user.name"); currentName != "" {
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
		if currentEmail := p.GetCurrentConfig("user.email"); currentEmail != "" {
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
	if err := p.SetUserConfig(ctx, fullName, email); err != nil {
		return err
	}

	if defaultsRequested(args) {
		if err := p.SetSensibleDefaults(ctx); err != nil {
			return err
		}
	}

	fmt.Println("\nGit configuration complete!")
	return nil
}

// ParseConfigArgs parses command line arguments for name and email
func (p *GitPlugin) ParseConfigArgs(args []string) (string, string) {
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

// GetCurrentConfig gets the current value of a git configuration key
func (p *GitPlugin) GetCurrentConfig(key string) string {
	output, err := sdk.ExecCommandOutputWithTimeoutAndOperation(p.GetTimeout("shell"), "shell", "git", "config", "--global", key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(output)
}

// SetUserConfig sets the Git username and email
func (p *GitPlugin) SetUserConfig(ctx context.Context, fullName, email string) error {
	if fullName != "" {
		if err := sdk.ExecCommandWithTimeoutAndOperation(p.GetTimeout("shell"), "shell", false, "git", "config", "--global", "user.name", fullName); err != nil {
			return fmt.Errorf("failed to set git user name: %w", err)
		}
		fmt.Printf("Set git user name: %s\n", fullName)
	}

	if email != "" {
		if err := sdk.ExecCommandWithTimeoutAndOperation(p.GetTimeout("shell"), "shell", false, "git", "config", "--global", "user.email", email); err != nil {
			return fmt.Errorf("failed to set git user email: %w", err)
		}
		fmt.Printf("Set git user email: %s\n", email)
	}

	return nil
}

// SetSensibleDefaults sets recommended Git configuration defaults that are unset
func (p *GitPlugin) SetSensibleDefaults(ctx context.Context) error {
	configs := map[string]string{
		"init.defaultBranch":  "main",
		"core.editor":         "vim",
		"color.ui":            "auto",
		"pull.rebase":         "false",
		"push.default":        "simple",
		"credential.helper":   "cache --timeout=3600",
		"merge.conflictstyle": "diff3",
		"diff.colorMoved":     "default",
		"fetch.prune":         "true",
	}

	for key, value := range configs {
		if _, err := p.setGlobalConfigIfUnset(ctx, key, value); err != nil {
			fmt.Printf("Warning: failed to set %s: %v\n", key, err)
		}
	}

	return nil
}

func (p *GitPlugin) setGlobalConfigIfUnset(ctx context.Context, key, value string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, p.GetTimeout("shell"))
	defer cancel()

	_, err := sdk.ExecCommandOutputWithContext(ctx, "git", "config", "--global", "--get", key)
	if err == nil {
		fmt.Printf("Kept %s\n", key)
		return false, nil
	}
	// Only Git's "no value" status means unset. Anything else (several values,
	// an unreadable config) leaves the key alone.
	var exit interface{ ExitCode() int }
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return false, fmt.Errorf("read %s before setting it: %w", key, err)
	}

	if err := sdk.ExecCommandWithContext(ctx, false, "git", "config", "--global", key, value); err != nil {
		return false, err
	}
	fmt.Printf("Set %s\n", key)
	return true, nil
}

func defaultsRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--defaults" {
			return true
		}
	}
	return false
}
