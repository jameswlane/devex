package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// validatePackageName validates package names to prevent command injection
func (a *APTInstaller) validatePackageName(packageName string) error {
	// Basic package name validation
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	// Check for dangerous characters
	dangerousPattern := regexp.MustCompile(`[;&|$(){}[\\]<>*?~\\s]`)
	if dangerousPattern.MatchString(packageName) {
		return fmt.Errorf("package name contains invalid characters")
	}

	// Check length
	if len(packageName) > 100 {
		return fmt.Errorf("package name too long")
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
	suspiciousChars := []string{";", "&", "|", "$", "`", "(", ")", "{", "}", "<", ">"}
	for _, char := range suspiciousChars {
		if strings.Contains(repo, char) {
			return true
		}
	}
	return false
}
