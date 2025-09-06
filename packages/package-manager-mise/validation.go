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

// validateMiseCommand validates tool specifications to prevent command injection
func (m *MisePlugin) validateMiseCommand(toolSpec string) error {
	// Basic validation
	if toolSpec == "" {
		return fmt.Errorf("tool specification cannot be empty")
	}

	// Check length
	if len(toolSpec) > MaxToolNameLength {
		return fmt.Errorf("tool specification too long (max %d characters)", MaxToolNameLength)
	}

	// Check for null bytes and control characters
	for _, r := range toolSpec {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) { // Allow tab, LF, CR
			return fmt.Errorf("tool specification contains invalid characters")
		}
	}

	// Check for dangerous characters that could be used in command injection
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">", "*", "?", "~", "\\n", "\\r"}
	for _, char := range dangerousChars {
		if strings.Contains(toolSpec, char) {
			return fmt.Errorf("tool specification contains invalid characters")
		}
	}

	// Validate tool specification format (tool@version or just tool)
	if err := m.validateToolSpecFormat(toolSpec); err != nil {
		return fmt.Errorf("invalid tool specification format: %w", err)
	}

	return nil
}

// validateToolSpecFormat validates the format of tool specifications
func (m *MisePlugin) validateToolSpecFormat(toolSpec string) error {
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

	// Tool name should only contain alphanumeric characters, hyphens, and underscores
	toolNameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !toolNameRegex.MatchString(toolName) {
		return fmt.Errorf("tool name contains invalid characters (only alphanumeric, hyphens, and underscores allowed)")
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

		// Allow common version patterns (e.g., "18", "3.11", "1.0.0", "v1.2.3")
		versionRegex := regexp.MustCompile(`^v?[0-9]+(\.[0-9]+)*([a-zA-Z0-9_.-]*)?$`)
		if !versionRegex.MatchString(version) {
			return fmt.Errorf("invalid version format")
		}
	}

	return nil
}
