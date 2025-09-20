package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// MaxPackageNameLength defines the maximum allowed package name length
	MaxPackageNameLength = 100

	// MaxFilePathLength defines the maximum allowed file path length
	MaxFilePathLength = 4096

	// MaxURLLength defines the maximum allowed URL length
	MaxURLLength = 2048
)

// validatePackageName validates package names to prevent command injection
func (d *DebInstaller) validatePackageName(packageName string) error {
	// Basic package name validation
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	if len(packageName) > MaxPackageNameLength {
		return fmt.Errorf("package name too long (max %d characters)", MaxPackageNameLength)
	}

	// Check for null bytes and control characters
	for _, r := range packageName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) { // Allow tab, LF, CR
			return fmt.Errorf("package name contains invalid characters")
		}
	}

	// Debian package name validation
	// Package names must consist of lowercase letters (a-z), digits (0-9),
	// plus (+) and minus (-) signs, and periods (.)
	validPackageNameRegex := regexp.MustCompile(`^[a-z0-9+.-]+$`)
	if !validPackageNameRegex.MatchString(packageName) {
		return fmt.Errorf("invalid package name format: must contain only lowercase letters, digits, +, -, and")
	}

	// Package names cannot start with a hyphen or period
	if strings.HasPrefix(packageName, "-") || strings.HasPrefix(packageName, ".") {
		return fmt.Errorf("package name cannot start with - or")
	}

	return nil
}

// validateFilePath validates file paths to prevent directory traversal and other attacks
func (d *DebInstaller) validateFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if len(filePath) > MaxFilePathLength {
		return fmt.Errorf("file path too long (max %d characters)", MaxFilePathLength)
	}

	// Check for null bytes
	if strings.Contains(filePath, "\x00") {
		return fmt.Errorf("file path contains null bytes")
	}

	// Resolve the path to check for directory traversal
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Basic directory traversal protection
	if strings.Contains(absPath, "..") {
		return fmt.Errorf("file path contains directory traversal")
	}

	// For .deb files, check if they exist and have proper extension
	if strings.HasSuffix(strings.ToLower(filePath), ".deb") {
		if _, err := os.Stat(filePath); err != nil {
			return fmt.Errorf("package file does not exist: %s", filePath)
		}
	}

	return nil
}

// validateURL validates URLs for downloading packages
func (d *DebInstaller) validateURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	if len(rawURL) > MaxURLLength {
		return fmt.Errorf("URL too long (max %d characters)", MaxURLLength)
	}

	// Parse the URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow HTTP and HTTPS
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS URLs are allowed")
	}

	// Validate hostname
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	// Prevent access to localhost and private IP ranges (basic protection)
	hostname := strings.ToLower(parsedURL.Hostname())
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return fmt.Errorf("access to localhost is not allowed")
	}

	// Check for private IP ranges (simplified)
	privateRanges := []string{"10.", "192.168.", "172.16.", "172.17.", "172.18.", "172.19."}
	for _, privateRange := range privateRanges {
		if strings.HasPrefix(hostname, privateRange) {
			return fmt.Errorf("access to private IP ranges is not allowed")
		}
	}

	// The URL should point to a .deb file
	if !strings.HasSuffix(strings.ToLower(parsedURL.Path), ".deb") {
		return fmt.Errorf("URL must point to a .deb file")
	}

	return nil
}

// ValidateCommand validates command arguments to prevent injection
func (d *DebInstaller) ValidateCommand(args []string) error {
	for _, arg := range args {
		if err := d.ValidateCommandArg(arg); err != nil {
			return fmt.Errorf("invalid command argument '%s': %w", arg, err)
		}
	}
	return nil
}

// ValidateCommandArg validates individual command arguments
func (d *DebInstaller) ValidateCommandArg(arg string) error {
	if arg == "" {
		return fmt.Errorf("argument cannot be empty")
	}

	// Check for null bytes and dangerous control characters
	for _, r := range arg {
		if r == 0 {
			return fmt.Errorf("argument contains null bytes")
		}
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// SanitizeOutput sanitizes command output for safe logging
func (d *DebInstaller) SanitizeOutput(output string) string {
	// Remove null bytes
	output = strings.ReplaceAll(output, "\x00", "")

	// Limit output length for logging
	const maxLogLength = 1000
	if len(output) > maxLogLength {
		output = output[:maxLogLength] + "...[truncated]"
	}

	return output
}
