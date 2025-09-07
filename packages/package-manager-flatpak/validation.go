package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	// MaxAppIDLength defines the maximum allowed application ID length
	MaxAppIDLength = 255

	// MaxRemoteNameLength defines the maximum allowed remote name length
	MaxRemoteNameLength = 100

	// MaxSearchTermLength defines the maximum allowed search term length
	MaxSearchTermLength = 100
)

// validateAppID validates Flatpak application IDs
func (f *FlatpakInstaller) validateAppID(appID string) error {
	if appID == "" {
		return fmt.Errorf("application ID cannot be empty")
	}

	if len(appID) > MaxAppIDLength {
		return fmt.Errorf("application ID too long (max %d characters)", MaxAppIDLength)
	}

	// Check for null bytes and control characters
	for _, r := range appID {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("application ID contains invalid characters")
		}
	}

	// Flatpak app IDs should follow reverse domain notation
	// e.g., org.mozilla.Firefox, com.github.user.app
	if !f.isValidFlatpakID(appID) {
		return fmt.Errorf("invalid Flatpak application ID format: must follow reverse domain notation")
	}

	return nil
}

// validateRemoteName validates Flatpak remote names
func (f *FlatpakInstaller) validateRemoteName(remoteName string) error {
	if remoteName == "" {
		return fmt.Errorf("remote name cannot be empty")
	}

	if len(remoteName) > MaxRemoteNameLength {
		return fmt.Errorf("remote name too long (max %d characters)", MaxRemoteNameLength)
	}

	// Check for null bytes and control characters
	for _, r := range remoteName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("remote name contains invalid characters")
		}
	}

	// Remote names should contain only alphanumeric characters, hyphens, and dots
	validRemoteNameRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	if !validRemoteNameRegex.MatchString(remoteName) {
		return fmt.Errorf("invalid remote name format: must contain only letters, numbers, dots, and hyphens")
	}

	// Remote names cannot start with a hyphen
	if strings.HasPrefix(remoteName, "-") {
		return fmt.Errorf("remote name cannot start with a hyphen")
	}

	return nil
}

// validateRemoteURL validates Flatpak repository URLs
func (f *FlatpakInstaller) validateRemoteURL(remoteURL string) error {
	if remoteURL == "" {
		return fmt.Errorf("remote URL cannot be empty")
	}

	// Parse the URL
	parsedURL, err := url.Parse(remoteURL)
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

	// The URL should point to a .flatpakrepo file
	if !strings.HasSuffix(strings.ToLower(parsedURL.Path), ".flatpakrepo") {
		return fmt.Errorf("URL must point to a .flatpakrepo file")
	}

	return nil
}

// validateSearchTerm validates search terms to prevent injection
func (f *FlatpakInstaller) validateSearchTerm(searchTerm string) error {
	if searchTerm == "" {
		return fmt.Errorf("search term cannot be empty")
	}

	if len(searchTerm) > MaxSearchTermLength {
		return fmt.Errorf("search term too long (max %d characters)", MaxSearchTermLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range searchTerm {
		if r == 0 {
			return fmt.Errorf("search term contains null bytes")
		}
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(searchTerm, char) {
			return fmt.Errorf("search term contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// isValidFlatpakID checks if an application ID follows Flatpak naming conventions
func (f *FlatpakInstaller) isValidFlatpakID(appID string) bool {
	// Flatpak IDs should follow reverse domain notation
	// Must contain at least one dot and consist of valid characters
	parts := strings.Split(appID, ".")
	if len(parts) < 2 {
		return false
	}

	// Each part should be a valid identifier
	for _, part := range parts {
		if part == "" {
			return false
		}

		// Check if part contains only valid characters
		validPartRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
		if !validPartRegex.MatchString(part) {
			return false
		}

		// Parts cannot start with a digit
		if len(part) > 0 && part[0] >= '0' && part[0] <= '9' {
			return false
		}
	}

	return true
}

// SanitizeOutput sanitizes command output for safe logging
func (f *FlatpakInstaller) SanitizeOutput(output string) string {
	// Remove null bytes
	output = strings.ReplaceAll(output, "\x00", "")

	// Limit output length for logging
	const maxLogLength = 1000
	if len(output) > maxLogLength {
		output = output[:maxLogLength] + "...[truncated]"
	}

	return output
}

// ValidateCommand validates command arguments to prevent injection
func (f *FlatpakInstaller) ValidateCommand(args []string) error {
	for _, arg := range args {
		if err := f.ValidateCommandArg(arg); err != nil {
			return fmt.Errorf("invalid command argument '%s': %w", arg, err)
		}
	}
	return nil
}

// ValidateCommandArg validates individual command arguments
func (f *FlatpakInstaller) ValidateCommandArg(arg string) error {
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
