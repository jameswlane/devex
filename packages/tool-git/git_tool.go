package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// GitPlugin implements the Git configuration plugin
type GitPlugin struct {
	*sdk.BasePlugin
}

// Execute handles command execution
func (p *GitPlugin) Execute(command string, args []string) error {
	// Ensure git is available
	if !sdk.CommandExists("git") {
		return fmt.Errorf("git is not installed on this system")
	}

	// Validate command and arguments for security
	if err := p.validateInputs(command, args); err != nil {
		return err
	}

	ctx := context.Background()

	switch command {
	case "config":
		return p.HandleConfig(ctx, args)
	case "aliases":
		return p.HandleAliases(ctx, args)
	case "status":
		return p.HandleStatus(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// validateInputs performs security validation on command inputs
func (p *GitPlugin) validateInputs(command string, args []string) error {
	// Check for dangerous shell metacharacters
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}

	// Validate command
	for _, char := range dangerousChars {
		if strings.Contains(command, char) {
			return fmt.Errorf("command contains potentially dangerous character: %s", char)
		}
	}

	// Validate all arguments
	for _, arg := range args {
		for _, char := range dangerousChars {
			if strings.Contains(arg, char) {
				return fmt.Errorf("argument contains potentially dangerous character: %s", char)
			}
		}

		// Additional validation for specific patterns
		if err := p.validateArgument(arg); err != nil {
			return err
		}
	}

	return nil
}

// validateArgument validates individual arguments for malicious patterns
func (p *GitPlugin) validateArgument(arg string) error {
	// Patterns that indicate potential command injection
	maliciousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`;\s*rm\s+-rf`),
		regexp.MustCompile(`&&\s*curl.*\|.*sh`),
		regexp.MustCompile(`\|\|\s*\w+`),
		regexp.MustCompile(`\$\([^)]*\)`),
		regexp.MustCompile("`[^`]*`"),
	}

	for _, pattern := range maliciousPatterns {
		if pattern.MatchString(arg) {
			return fmt.Errorf("argument contains potentially malicious pattern: %s", arg)
		}
	}

	return nil
}
