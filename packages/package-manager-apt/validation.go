package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// MaxPackageNameLength defines the maximum allowed package name length
	MaxPackageNameLength = 100
)

// validatePackageName validates package names to prevent command injection
func (a *APTInstaller) validatePackageName(packageName string) error {
	// Basic package name validation
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	// Check for null bytes and control characters
	for i, r := range packageName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) { // Allow tab, LF, CR
			return fmt.Errorf("package name contains invalid characters")
		}
		if i == 0 && (r == '-' || r == '.') {
			return fmt.Errorf("package name contains invalid characters")
		}
	}

	// Check for dangerous characters including backticks and newlines
	dangerousPattern := regexp.MustCompile(`[;&|$(){}\[\]<>*?~\s` + "`" + `\n\r]`)
	if dangerousPattern.MatchString(packageName) {
		return fmt.Errorf("package name contains invalid characters")
	}

	// Check length
	if len(packageName) > MaxPackageNameLength {
		return fmt.Errorf("package name too long (max %d characters)", MaxPackageNameLength)
	}

	return nil
}

// validateAptRepo ensures the repository string is valid
// Based on the original robust validation with plugin SDK integration
func (a *APTInstaller) validateAptRepo(repo string) error {
	a.logger.Debug("Validating APT repository", "repo", repo)

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

	a.logger.Debug("Repository validation passed", "repo", repo)
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
			_, err := url.Parse(part)
			return err == nil
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

// validateFilePath validates file paths to prevent directory traversal and dangerous paths
func (a *APTInstaller) validateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("file path cannot be empty")
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
	}

	for _, dangerous := range dangerousPaths {
		if cleanPath == dangerous || strings.HasPrefix(cleanPath, dangerous+"/") {
			return fmt.Errorf("access to system directory not allowed: %s", path)
		}
	}

	return nil
}
