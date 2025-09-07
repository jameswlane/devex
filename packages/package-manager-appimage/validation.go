package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	// MaxBinaryNameLength defines the maximum allowed binary name length
	MaxBinaryNameLength = 100

	// MaxURLLength defines the maximum allowed URL length
	MaxURLLength = 2048
)

// validateAppImageParameters validates download URL and binary name
func (p *AppimagePlugin) validateAppImageParameters(downloadURL, binaryName string) error {
	// Validate URL
	if err := p.validateURL(downloadURL); err != nil {
		return fmt.Errorf("invalid download URL '%s': %w", downloadURL, err)
	}

	// Validate binary name
	if err := p.validateBinaryName(binaryName); err != nil {
		return fmt.Errorf("invalid binary name '%s': %w", binaryName, err)
	}

	return nil
}

// validateURL validates URL format and basic security checks
func (p *AppimagePlugin) validateURL(inputURL string) error {
	if inputURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	if len(inputURL) > MaxURLLength {
		return fmt.Errorf("URL too long (max %d characters)", MaxURLLength)
	}

	// Parse URL
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Validate scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS URLs are allowed")
	}

	// Validate hostname
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	// Prevent localhost and private IP access
	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return fmt.Errorf("access to localhost is not allowed")
	}

	// Check for private IP ranges (basic protection)
	privateRanges := []string{"10.", "192.168.", "172.16.", "172.17.", "172.18.", "172.19."}
	for _, privateRange := range privateRanges {
		if strings.HasPrefix(hostname, privateRange) {
			return fmt.Errorf("access to private IP ranges is not allowed")
		}
	}

	return nil
}

// validateBinaryName validates binary names for AppImages
func (p *AppimagePlugin) validateBinaryName(binaryName string) error {
	if binaryName == "" {
		return fmt.Errorf("binary name cannot be empty")
	}

	if len(binaryName) > MaxBinaryNameLength {
		return fmt.Errorf("binary name too long (max %d characters)", MaxBinaryNameLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range binaryName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("binary name contains invalid characters")
		}
	}

	// Validate against regex for safe filenames
	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	if !validName.MatchString(binaryName) {
		return fmt.Errorf("binary name must start with alphanumeric and contain only letters, numbers, dots, hyphens, and underscores")
	}

	// Check for potentially dangerous names
	if binaryName == "." || binaryName == ".." {
		return fmt.Errorf("invalid binary name")
	}

	// Validate against path separators
	if strings.ContainsAny(binaryName, "/\\") {
		return fmt.Errorf("binary name cannot contain path separators")
	}

	// Check for shell metacharacters that could be dangerous
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">"}
	for _, char := range dangerousChars {
		if strings.Contains(binaryName, char) {
			return fmt.Errorf("binary name contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// ValidateInstallLocation validates installation location
func (p *AppimagePlugin) ValidateInstallLocation(location string) error {
	validLocations := []string{"gui", "cli"}

	for _, valid := range validLocations {
		if location == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid install location '%s', must be 'gui' or 'cli'", location)
}

// SanitizeOutput sanitizes command output for safe logging
func (p *AppimagePlugin) SanitizeOutput(output string) string {
	// Remove null bytes
	output = strings.ReplaceAll(output, "\x00", "")

	// Limit output length for logging
	const maxLogLength = 1000
	if len(output) > maxLogLength {
		output = output[:maxLogLength] + "...[truncated]"
	}

	return output
}
