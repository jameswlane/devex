package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaxToolNameLength defines the maximum allowed tool name length
	MaxToolNameLength = 100
)

// ValidateToolSpec validates tool specifications to prevent command injection
func (m *MisePlugin) ValidateToolSpec(toolSpec string) error {
	// Basic validation
	if toolSpec == "" {
		return fmt.Errorf("tool specification cannot be empty")
	}

	// Check length
	if len(toolSpec) > MaxToolNameLength {
		return fmt.Errorf("tool specification too long")
	}

	// Check for null bytes and control characters
	for _, r := range toolSpec {
		if r == 0 {
			return fmt.Errorf("tool specification contains null bytes")
		}
		if r < 32 && r != 9 && r != 10 && r != 13 { // Allow tab, LF, CR
			return fmt.Errorf("tool specification contains invalid control characters")
		}
	}

	// Check for dangerous characters that could be used in command injection
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">", "*", "?", "~", "\\n", "\\r"}
	for _, char := range dangerousChars {
		if strings.Contains(toolSpec, char) {
			return fmt.Errorf("tool specification contains potentially dangerous character: %s", char)
		}
	}

	// Validate tool specification format (tool@version or just tool)
	if err := m.ValidateToolSpecFormat(toolSpec); err != nil {
		return fmt.Errorf("invalid tool specification format: %w", err)
	}

	return nil
}

// ValidateToolSpecFormat validates the format of tool specifications
func (m *MisePlugin) ValidateToolSpecFormat(toolSpec string) error {
	// Valid formats:
	// - tool (e.g., "node", "python")
	// - tool@version (e.g., "node@18", "python@3.11")
	// - tool@latest (e.g., "node@latest")

	// Split by @ to check format
	parts := strings.Split(toolSpec, "@")

	if len(parts) > 2 {
		return fmt.Errorf("tool specification can only contain one '@' character")
	}

	// Validate tool name (first part)
	toolName := parts[0]
	if toolName == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	// Tool name should start with a letter and contain only alphanumeric characters, hyphens, and underscores
	toolNameRegex := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	if !toolNameRegex.MatchString(toolName) {
		return fmt.Errorf("tool name must start with a letter and contain only alphanumeric, hyphens, and underscores")
	}

	// If version is specified, validate it
	if len(parts) == 2 {
		version := parts[1]
		if version == "" {
			return fmt.Errorf("version cannot be empty when '@' is used")
		}

		// Version can be "latest" or a semantic version-like string
		if version == "latest" {
			return nil
		}

		// Allow common version patterns with safer regex to prevent ReDoS
		// Limit complexity and use more specific patterns
		versionRegex := regexp.MustCompile(`^[~^]?v?[0-9]{1,3}(?:\.[0-9]{1,3}){0,3}(?:[a-zA-Z0-9_.-]{0,20})?$`)
		if !versionRegex.MatchString(version) {
			return fmt.Errorf("invalid version format")
		}
	}

	return nil
}

// ValidateShellType validates shell type to ensure it's supported
func (m *MisePlugin) ValidateShellType(shellType string) error {
	if shellType == "" {
		return fmt.Errorf("shell type cannot be empty")
	}

	// Check for dangerous characters
	if strings.ContainsAny(shellType, ";|&`$(){}[]<>\\") {
		return fmt.Errorf("shell type contains invalid characters")
	}

	// Validate against supported shells
	supportedShells := []string{"bash", "zsh", "fish"}
	for _, supported := range supportedShells {
		if shellType == supported {
			return nil
		}
	}

	return fmt.Errorf("unsupported shell type: %s (supported: bash, zsh, fish)", shellType)
}

// ValidateCommandArg validates individual command arguments
func (m *MisePlugin) ValidateCommandArg(arg string) error {
	if arg == "" {
		return fmt.Errorf("argument cannot be empty")
	}

	// Check for null bytes
	for _, r := range arg {
		if r == 0 {
			return fmt.Errorf("argument contains null bytes")
		}
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// ValidateEnvironmentVar validates environment variable names
func (m *MisePlugin) ValidateEnvironmentVar(varName string) error {
	if varName == "" {
		return fmt.Errorf("environment variable name cannot be empty")
	}

	// Environment variable names should match the pattern [A-Z_][A-Z0-9_]*
	envVarRegex := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	if !envVarRegex.MatchString(varName) {
		return fmt.Errorf("invalid environment variable name: must start with letter or underscore and contain only letters, numbers, and underscores")
	}

	return nil
}
