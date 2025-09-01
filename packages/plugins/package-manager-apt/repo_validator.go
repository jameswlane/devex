package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/jameswlane/devex/pkg/log"
)

// ValidateAptRepo ensures the repository string is valid.
func ValidateAptRepo(repo string) error {
	log.Info("Validating APT repository", "repo", repo)

	if repo == "" {
		return fmt.Errorf("repository string cannot be empty")
	}

	if len(repo) < 10 {
		return fmt.Errorf("repository string too short: %s", repo)
	}

	// Basic format validation
	if !containsValidKeywords(repo) {
		return fmt.Errorf("repository string missing required keywords (deb, http/https): %s", repo)
	}

	// Check for valid URL format
	if !containsValidURL(repo) {
		return fmt.Errorf("repository string contains invalid URL format: %s", repo)
	}

	// Check for potential security issues
	if containsSuspiciousContent(repo) {
		return fmt.Errorf("repository string contains suspicious content: %s", repo)
	}

	log.Info("APT repository validated successfully", "repo", repo)
	return nil
}

func containsValidKeywords(repo string) bool {
	return strings.Contains(repo, "deb") && (strings.Contains(repo, "http://") || strings.Contains(repo, "https://"))
}

func containsValidURL(repo string) bool {
	// Extract URL from APT repository string using regex
	// APT repo format: "deb [options] http://example.com/ubuntu focal main"
	urlRegex := regexp.MustCompile(`https?://[^\s]+`)
	matches := urlRegex.FindAllString(repo, -1)

	if len(matches) == 0 {
		return false
	}

	// Validate each URL found in the repository string
	for _, urlStr := range matches {
		if _, err := url.Parse(urlStr); err != nil {
			log.Warn("Invalid URL found in repository string", "url", urlStr, "error", err)
			return false
		}

		// Additional validation: ensure URL has proper scheme and host
		parsedURL, _ := url.Parse(urlStr)
		if parsedURL.Scheme == "" || parsedURL.Host == "" {
			log.Warn("URL missing scheme or host", "url", urlStr)
			return false
		}

		// Only allow HTTP and HTTPS
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			log.Warn("URL uses unsupported scheme", "url", urlStr, "scheme", parsedURL.Scheme)
			return false
		}
	}

	return true
}

func containsSuspiciousContent(repo string) bool {
	// Check for potentially dangerous characters or patterns
	suspicious := []string{
		";", "|", "&", "$", "`", "$(", ")",
		"rm ", "sudo ", "wget ", "curl ", "bash", "sh ",
		"../", "./", "~", "*",
	}

	repoLower := strings.ToLower(repo)
	for _, pattern := range suspicious {
		if strings.Contains(repoLower, pattern) {
			log.Warn("Suspicious pattern detected in repository string", "pattern", pattern, "repo", repo)
			return true
		}
	}
	return false
}
