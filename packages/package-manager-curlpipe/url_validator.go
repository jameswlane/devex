package main

import (
	"fmt"
	"net/url"
	"strings"
)

// Trusted domains for curl pipe installations
var trustedDomains = []string{
	"get.docker.com",
	"sh.rustup.rs",
	"raw.githubusercontent.com",
	"github.com",
	"install.python-poetry.org",
	"mise.jdx.dev",
	"get.helm.sh",
	"install.k3s.io",
	"get.k3s.io",
	"installer.id",
	"sh.brew.sh",
	"deno.land",
	"bun.sh",
}

// GetTrustedDomains returns the list of trusted domains
func (p *CurlpipePlugin) GetTrustedDomains() []string {
	return trustedDomains
}

// ValidateScriptURL validates the format of a script URL
func (p *CurlpipePlugin) ValidateScriptURL(scriptURL string) error {
	if err := p.ValidateURL(scriptURL); err != nil {
		return err
	}

	parsedURL, err := url.Parse(scriptURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Additional validation for script URLs
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("URL must use HTTP or HTTPS protocol")
	}

	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a valid host")
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

// validateTrustedDomain checks if a URL is from a trusted domain
func (p *CurlpipePlugin) validateTrustedDomain(scriptURL string) error {
	parsedURL, err := url.Parse(scriptURL)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	host := parsedURL.Host

	// Check if host matches any trusted domain
	for _, trusted := range trustedDomains {
		if host == trusted || strings.HasSuffix(host, "."+trusted) {
			return nil
		}
	}

	return fmt.Errorf("domain '%s' is not in the trusted domains list", host)
}

// IsTrustedDomain checks if a domain is in the trusted list
func (p *CurlpipePlugin) IsTrustedDomain(domain string) bool {
	for _, trusted := range trustedDomains {
		if domain == trusted || strings.HasSuffix(domain, "."+trusted) {
			return true
		}
	}
	return false
}

// extractAppNameFromURL extracts application name from installation URL
func (p *CurlpipePlugin) extractAppNameFromURL(scriptURL string) string {
	parsedURL, err := url.Parse(scriptURL)
	if err != nil {
		return "unknown"
	}

	// Extract from common patterns
	host := parsedURL.Host
	path := parsedURL.Path

	// Common patterns for app names
	if strings.Contains(host, "get.") {
		// get.docker.com -> docker
		parts := strings.Split(host, ".")
		if len(parts) >= 2 && parts[0] == "get" {
			return parts[1]
		}
	}

	if strings.Contains(path, "/install") {
		// Extract from path like /install/docker.sh
		parts := strings.Split(path, "/")
		for _, part := range parts {
			if part != "" && part != "install" && !strings.HasSuffix(part, ".sh") {
				return part
			}
		}
	}

	// Extract from specific known patterns
	if strings.Contains(host, "rustup") {
		return "rust"
	}

	if strings.Contains(host, "deno") {
		return "deno"
	}

	if strings.Contains(host, "bun") {
		return "bun"
	}

	// Fallback: use host domain
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] // Second-to-last part of domain
	}

	return "unknown"
}

// ValidateDomainName validates domain name format
func (p *CurlpipePlugin) ValidateDomainName(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	if len(domain) > 253 {
		return fmt.Errorf("domain too long (max 253 characters)")
	}

	// Basic domain validation
	if strings.Contains(domain, "..") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("invalid domain format")
	}

	// Check for valid characters
	for _, r := range domain {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '.' && r != '-' {
			return fmt.Errorf("domain contains invalid characters")
		}
	}

	return nil
}
