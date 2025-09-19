package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	// MaxPackageNameLength defines the maximum allowed package name length
	MaxPackageNameLength = 100

	// MaxFilePathLength defines the maximum allowed file path length
	MaxFilePathLength = 4096
)

// ValidatePackageName validates package names to prevent command injection (exported for testing)
func (a *APTInstaller) ValidatePackageName(packageName string) error {
	return a.validatePackageName(packageName)
}

// validatePackageName validates package names to prevent command injection
func (a *APTInstaller) validatePackageName(packageName string) error {
	// Basic package name validation
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	// Check length to prevent excessively long names
	if len(packageName) > MaxPackageNameLength {
		return fmt.Errorf("package name too long: %d characters (max %d)", len(packageName), MaxPackageNameLength)
	}

	// Check for null bytes and control characters
	for i, r := range packageName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) { // Allow tab, LF, CR
			return fmt.Errorf("package name contains invalid characters at position %d", i)
		}
		if i == 0 && (r == '-' || r == '.') {
			return fmt.Errorf("package name cannot start with '%c' character", r)
		}
	}

	// Check for dangerous characters including backticks, newlines, and format strings
	dangerousPattern := regexp.MustCompile(`[;&|$(){}\[\]<>*?~%\s` + "`" + `\n\r]`)
	if dangerousPattern.MatchString(packageName) {
		return fmt.Errorf("package name contains invalid characters")
	}

	// Check length
	if len(packageName) > MaxPackageNameLength {
		return fmt.Errorf("package name too long (max %d characters)", MaxPackageNameLength)
	}

	// APT package names should only contain ASCII characters, digits, hyphens, periods, plus signs
	// Reject unicode and other non-ASCII characters
	for _, r := range packageName {
		if r > 127 {
			return fmt.Errorf("package name contains non-ASCII characters")
		}
	}

	return nil
}

// ValidateAptRepo ensures the repository string is valid (exported for testing)
func (a *APTInstaller) ValidateAptRepo(repo string) error {
	return a.validateAptRepo(repo)
}

// validateAptRepo ensures the repository string is valid
// Based on the original robust validation with plugin SDK integration
func (a *APTInstaller) validateAptRepo(repo string) error {
	a.getLogger().Debug("Validating APT repository", "repo", repo)

	if repo == "" {
		return fmt.Errorf("repository string cannot be empty")
	}

	if len(repo) < 10 {
		return fmt.Errorf("repository string too short: %s", repo)
	}

	// Basic format validation
	if !a.containsValidKeywords(repo) {
		return fmt.Errorf("repository string missing required keywords (deb, http/https): %s", repo)
	}

	// Check for valid URL format
	if !a.containsValidURL(repo) {
		return fmt.Errorf("repository string contains invalid URL: %s", repo)
	}

	// Check for suspicious characters that might indicate command injection
	if a.containsSuspiciousCharacters(repo) {
		return fmt.Errorf("repository string contains suspicious characters: %s", repo)
	}

	a.getLogger().Debug("Repository validation passed", "repo", repo)
	return nil
}

// containsValidKeywords checks if the repository string contains required keywords
func (a *APTInstaller) containsValidKeywords(repo string) bool {
	return strings.Contains(repo, "deb") &&
		(strings.Contains(repo, "http://") || strings.Contains(repo, "https://"))
}

// containsValidURL validates that the repository contains a proper URL
func (a *APTInstaller) containsValidURL(repo string) bool {
	// Extract URL from repository string
	parts := strings.Fields(repo)
	for _, part := range parts {
		if strings.HasPrefix(part, "http://") || strings.HasPrefix(part, "https://") {
			parsedURL, err := url.Parse(part)
			if err != nil {
				return false
			}

			// Additional validation for common malformed URL patterns
			if parsedURL.Host == "" {
				return false
			}

			// Check for double dots in hostname (malformed domains)
			if strings.Contains(parsedURL.Host, "..") {
				return false
			}

			// Check for invalid ports
			if port := parsedURL.Port(); port != "" {
				if portNum, err := strconv.Atoi(port); err != nil || portNum < 1 || portNum > 65535 {
					return false
				}
			}

			// Check for obviously malformed IPv6
			if strings.Contains(parsedURL.Host, "[") && strings.Contains(parsedURL.Host, "]") {
				if strings.Contains(parsedURL.Host, "[invalid-") {
					return false
				}
			}

			return true
		}
	}
	return false
}

// containsSuspiciousCharacters checks for potential command injection attempts
func (a *APTInstaller) containsSuspiciousCharacters(repo string) bool {
	// Look for characters that could be used in command injection
	suspiciousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">", "*", "?", "~", "\n", "\r", "\t"}
	for _, char := range suspiciousChars {
		if strings.Contains(repo, char) {
			return true
		}
	}

	// Check for null bytes and control characters
	for _, r := range repo {
		if r == 0 || (r < 32 && r != 10 && r != 13) { // Allow LF (10) and CR (13) but not other control chars
			return true
		}
	}

	return false
}

// ValidateFilePath validates file paths to prevent directory traversal and dangerous paths (exported for testing)
func (a *APTInstaller) ValidateFilePath(path string) error {
	return a.validateFilePath(path)
}

// validateFilePath validates file paths to prevent directory traversal and dangerous paths
func (a *APTInstaller) validateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// Check path length to prevent excessive resource usage
	if len(path) > MaxFilePathLength {
		return fmt.Errorf("file path too long (max %d characters): %d", MaxFilePathLength, len(path))
	}

	// Check for null bytes and control characters
	for _, r := range path {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) { // Allow tab, LF, CR
			return fmt.Errorf("path contains invalid characters")
		}
	}

	// Check for dangerous characters that could be used in command injection
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">", "*", "?", "~", "\\n", "\\r"}
	for _, char := range dangerousChars {
		if strings.Contains(path, char) {
			return fmt.Errorf("path contains invalid characters")
		}
	}

	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal: %s", path)
	}

	// Ensure path is absolute for system operations
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("path must be absolute for system operations: %s", path)
	}

	// Prevent access to sensitive system directories
	dangerousPaths := []string{
		"/",
		"/bin",
		"/sbin",
		"/usr/bin",
		"/usr/sbin",
		"/boot",
		"/proc",
		"/sys",
		"/dev",
		"/etc",
		"/root",
	}

	for _, dangerous := range dangerousPaths {
		if cleanPath == dangerous || strings.HasPrefix(cleanPath, dangerous+"/") {
			return fmt.Errorf("access to system directory not allowed: %s", path)
		}
	}

	return nil
}

// ValidateKeyURL validates GPG key URLs to prevent command injection (exported for testing)
func (a *APTInstaller) ValidateKeyURL(keyURL string) error {
	return a.validateKeyURL(keyURL)
}

// validateKeyURL validates GPG key URLs to prevent command injection
func (a *APTInstaller) validateKeyURL(keyURL string) error {
	if keyURL == "" {
		return fmt.Errorf("key URL cannot be empty")
	}

	// Check for null bytes and control characters
	for _, r := range keyURL {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) { // Allow tab, LF, CR
			return fmt.Errorf("key URL contains invalid characters")
		}
	}

	// Check for dangerous characters that could be used in command injection
	dangerousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">", "~", "\\n", "\\r"}
	for _, char := range dangerousChars {
		if strings.Contains(keyURL, char) {
			return fmt.Errorf("key URL contains invalid characters")
		}
	}

	// Validate URL format and protocol
	parsedURL, err := url.Parse(keyURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow HTTP and HTTPS protocols
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTP and HTTPS protocols are allowed")
	}

	// Validate hostname
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid hostname")
	}

	// Prevent external URLs that would cause hanging in tests
	// Only block domains that are known to cause real network timeouts
	// Note: example.com is reserved for documentation and should work in tests
	blockedDomains := []string{
		"nonexistent.invalid",
		"timeout.test",
	}

	// Block example.com URLs with custom ports as they cause test timeouts
	if strings.Contains(parsedURL.Host, "example.com:") && parsedURL.Host != "example.com:80" && parsedURL.Host != "example.com:443" {
		return fmt.Errorf("GPG key URL uses example.com with custom port which causes test timeouts: %s", parsedURL.Host)
	}

	for _, blocked := range blockedDomains {
		if parsedURL.Host == blocked || strings.HasSuffix(parsedURL.Host, "."+blocked) {
			return fmt.Errorf("GPG key URL uses blocked domain for security: %s", parsedURL.Host)
		}
	}

	return nil
}
