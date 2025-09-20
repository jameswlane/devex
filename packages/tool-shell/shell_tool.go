package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Execute handles command execution
func (p *ShellPlugin) Execute(command string, args []string) error {
	ctx := context.Background()

	// Validate inputs for security
	if err := p.validateInputs(command, args); err != nil {
		return err
	}

	switch command {
	case "setup":
		return p.handleSetup(ctx, args)
	case "switch":
		return p.handleSwitch(ctx, args)
	case "config":
		return p.handleConfig(ctx, args)
	case "backup":
		return p.handleBackup(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// DetectCurrentShell detects the current shell from environment variables
func (p *ShellPlugin) DetectCurrentShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}

	// Extract shell name from path
	parts := strings.Split(shell, "/")
	return parts[len(parts)-1]
}

// validateInputs performs security validation on command inputs
func (p *ShellPlugin) validateInputs(command string, args []string) error {
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
		if err := p.validateArgument(arg); err != nil {
			return err
		}
	}

	return nil
}

// validateArgument validates a single argument for security
func (p *ShellPlugin) validateArgument(arg string) error {
	// Check for dangerous shell metacharacters
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}

	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains potentially dangerous character: %s", char)
		}
	}

	// Check for dangerous system paths
	dangerousPaths := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/root/",
		"~/../../../",
	}

	for _, path := range dangerousPaths {
		if strings.Contains(arg, path) {
			return fmt.Errorf("argument contains potentially dangerous path")
		}
	}

	// Check for malicious patterns using regex
	maliciousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`;\s*rm\s+-rf`),
		regexp.MustCompile(`&&\s*curl.*\|.*sh`),
		regexp.MustCompile(`\|\|\s*\w+`),
		regexp.MustCompile(`\$\([^)]*\)`),
		regexp.MustCompile("`[^`]*`"),
		regexp.MustCompile(`\.\./`), // Directory traversal
	}

	for _, pattern := range maliciousPatterns {
		if pattern.MatchString(arg) {
			return fmt.Errorf("argument contains potentially malicious pattern")
		}
	}

	return nil
}
