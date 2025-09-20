package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	// MaxURLLength defines the maximum allowed URL length
	MaxURLLength = 2048

	// MaxAppNameLength defines the maximum allowed application name length
	MaxAppNameLength = 100

	// MaxCommandLength defines the maximum allowed command length
	MaxCommandLength = 1000
)

// ValidateURL validates URL format and basic security checks
func (p *CurlpipePlugin) ValidateURL(inputURL string) error {
	if inputURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	if len(inputURL) > MaxURLLength {
		return fmt.Errorf("URL too long (max %d characters)", MaxURLLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range inputURL {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("URL contains invalid characters")
		}
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

	// Check for shell metacharacters that could be dangerous
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(inputURL, char) {
			return fmt.Errorf("URL contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// validateAppName validates application names
func (p *CurlpipePlugin) validateAppName(appName string) error {
	if appName == "" {
		return fmt.Errorf("application name cannot be empty")
	}

	if len(appName) > MaxAppNameLength {
		return fmt.Errorf("application name too long (max %d characters)", MaxAppNameLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range appName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("application name contains invalid characters")
		}
	}

	// Application names should contain only valid characters
	validNameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	if !validNameRegex.MatchString(appName) {
		return fmt.Errorf("invalid application name format: must start with alphanumeric and contain only letters, numbers, dots, hyphens, and underscores")
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(appName, char) {
			return fmt.Errorf("application name contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// ValidateCommand validates command arguments to prevent injection
func (p *CurlpipePlugin) ValidateCommand(args []string) error {
	for _, arg := range args {
		if err := p.ValidateCommandArg(arg); err != nil {
			return fmt.Errorf("invalid command argument '%s': %w", arg, err)
		}
	}
	return nil
}

// ValidateCommandArg validates individual command arguments
func (p *CurlpipePlugin) ValidateCommandArg(arg string) error {
	if arg == "" {
		return fmt.Errorf("argument cannot be empty")
	}

	if len(arg) > MaxCommandLength {
		return fmt.Errorf("argument too long (max %d characters)", MaxCommandLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range arg {
		if r == 0 {
			return fmt.Errorf("argument contains null bytes")
		}
	}

	// Allow common flags but check for shell metacharacters
	if !strings.HasPrefix(arg, "--") && !strings.HasPrefix(arg, "-") {
		// For non-flags, check for shell metacharacters
		dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
		for _, char := range dangerousChars {
			if strings.Contains(arg, char) {
				return fmt.Errorf("argument contains potentially dangerous character: %s", char)
			}
		}
	}

	return nil
}

// ValidateFileExtension validates file extensions for script files
func (p *CurlpipePlugin) ValidateFileExtension(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Common script extensions that are generally safe
	safeExtensions := []string{".sh", ".bash", ".py", ".rb", ".js", ".pl"}

	for _, ext := range safeExtensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return nil
		}
	}

	// Allow files without extensions (like install scripts)
	if !strings.Contains(filename, ".") {
		return nil
	}

	return fmt.Errorf("potentially unsafe file extension")
}

// SanitizeOutput sanitizes command output for safe logging
func (p *CurlpipePlugin) SanitizeOutput(output string) string {
	// Remove null bytes
	output = strings.ReplaceAll(output, "\x00", "")

	// Limit output length for logging
	const maxLogLength = 1000
	if len(output) > maxLogLength {
		output = output[:maxLogLength] + "...[truncated]"
	}

	return output
}

// ValidateScriptPath validates script paths to prevent directory traversal
func (p *CurlpipePlugin) ValidateScriptPath(path string) error {
	if path == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Prevent directory traversal
	if strings.Contains(path, "..") {
		return fmt.Errorf("script path contains directory traversal")
	}

	// Prevent absolute paths for security
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return fmt.Errorf("absolute script paths are not allowed")
	}

	return nil
}

// IsValidScriptType checks if the script type is allowed
func (p *CurlpipePlugin) IsValidScriptType(scriptType string) bool {
	allowedTypes := []string{"sh", "bash", "shell", "script", "install"}

	for _, allowed := range allowedTypes {
		if strings.EqualFold(scriptType, allowed) {
			return true
		}
	}

	return false
}

// ValidateScriptSecurity performs runtime security validation on script content
func (p *CurlpipePlugin) ValidateScriptSecurity(content string) error {
	if content == "" {
		return fmt.Errorf("script content is empty")
	}

	// Check for obviously destructive commands
	destructivePatterns := []struct {
		pattern     *regexp.Regexp
		description string
	}{
		{regexp.MustCompile(`\brm\s+(-[rfRF]*\s+)*(/|/home|/usr|/var|/etc|/boot|/opt)\s*$`), "destructive file removal"},
		{regexp.MustCompile(`\bdd\s+.*\bof=/dev/(sd[a-z]|hd[a-z]|nvme\d+n\d+)\b`), "disk overwrite command"},
		{regexp.MustCompile(`\bmkfs(\.[a-zA-Z0-9]+)?\s+/dev/`), "filesystem format command"},
		{regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`), "fork bomb pattern"},
		{regexp.MustCompile(`\bchmod\s+000\s+/`), "permission destruction"},
		{regexp.MustCompile(`\bchown\s+root:\s*/`), "ownership change to root"},
		{regexp.MustCompile(`\b(nc|netcat)\s+.*\s-e\s`), "netcat shell backdoor"},
		{regexp.MustCompile(`/dev/(tcp|udp)/.*/.*/exec`), "network shell backdoor"},
	}

	for _, pattern := range destructivePatterns {
		if pattern.pattern.MatchString(content) {
			return fmt.Errorf("script contains potentially destructive pattern: %s", pattern.description)
		}
	}

	// Check for suspicious network activity
	suspiciousNetworkPatterns := []struct {
		pattern     *regexp.Regexp
		description string
	}{
		{regexp.MustCompile(`\b(wget|curl).*\|\s*(bash|sh|python|perl|ruby)\b`), "chained network download execution"},
		{regexp.MustCompile(`\b(nc|netcat).*-l.*-p\s+\d+\b`), "listening network service"},
		{regexp.MustCompile(`\bssh\s+.*@.*\s.*\s&\s*$`), "background SSH connection"},
		{regexp.MustCompile(`\bcrontab\s+.*<<.*EOF`), "cron job installation"},
	}

	for _, pattern := range suspiciousNetworkPatterns {
		if pattern.pattern.MatchString(content) {
			p.logger.Printf("⚠️  Warning: Script contains suspicious network pattern: %s\n", pattern.description)
		}
	}

	// Check for privilege escalation attempts
	privEscPatterns := []struct {
		pattern     *regexp.Regexp
		description string
	}{
		{regexp.MustCompile(`\bsu\s+-\s+root\b`), "root privilege escalation"},
		{regexp.MustCompile(`\bsudo\s+su\s+-\b`), "sudo privilege escalation"},
		{regexp.MustCompile(`\bpasswd\s+root\b`), "root password change"},
		{regexp.MustCompile(`\buseradd.*-G.*sudo\b`), "sudo group addition"},
		{regexp.MustCompile(`\bsetuid\(\s*0\s*\)`), "setuid root call"},
	}

	for _, pattern := range privEscPatterns {
		if pattern.pattern.MatchString(content) {
			p.logger.Printf("⚠️  Warning: Script contains privilege escalation pattern: %s\n", pattern.description)
		}
	}

	// Check script size limits for performance
	const maxScriptSize = 10 * 1024 * 1024 // 10MB
	if len(content) > maxScriptSize {
		return fmt.Errorf("script size %d bytes exceeds maximum allowed size %d bytes", len(content), maxScriptSize)
	}

	// Check for excessive nested commands
	if strings.Count(content, "$(") > 50 || strings.Count(content, "`") > 50 {
		return fmt.Errorf("script contains excessive command substitution, possible obfuscation")
	}

	// Check for binary content (likely malicious or corrupted)
	nonPrintableCount := 0
	for _, r := range content {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Not tab, newline, or carriage return
			nonPrintableCount++
		}
	}

	if nonPrintableCount > len(content)/100 { // More than 1% non-printable
		return fmt.Errorf("script contains excessive non-printable characters, possibly binary content")
	}

	return nil
}
