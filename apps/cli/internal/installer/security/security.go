// Package security provides security validation helpers for installation operations
package security

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// SecureString represents a string that should be scrubbed from memory
type SecureString struct {
	data []byte
}

// NewSecureString creates a new secure string from the provided string
func NewSecureString(s string) *SecureString {
	data := make([]byte, len(s))
	copy(data, s)
	return &SecureString{data: data}
}

// String returns the string value of the secure string
// WARNING: Use sparingly and ensure Clear() is called immediately after use
func (ss *SecureString) String() string {
	if ss.data == nil {
		return ""
	}
	return string(ss.data)
}

// Clear scrubs the secure string from memory
func (ss *SecureString) Clear() {
	if ss.data != nil {
		for i := range ss.data {
			ss.data[i] = 0
		}
		ss.data = nil
	}
}

// URLValidator validates URLs for security
type URLValidator struct {
	trustedDomains []string
}

// NewURLValidator creates a new URL validator with trusted domains
func NewURLValidator(trustedDomains []string) *URLValidator {
	return &URLValidator{
		trustedDomains: trustedDomains,
	}
}

// ValidateURL validates that a URL is from a trusted domain and uses HTTPS
func (v *URLValidator) ValidateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("empty URL not allowed")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow HTTPS URLs
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed, got: %s", parsedURL.Scheme)
	}

	// Check if domain is in trusted list
	hostname := parsedURL.Hostname()
	for _, trustedDomain := range v.trustedDomains {
		if hostname == trustedDomain {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not in trusted domains list", hostname)
}

// AddTrustedDomain adds a domain to the trusted list
func (v *URLValidator) AddTrustedDomain(domain string) {
	v.trustedDomains = append(v.trustedDomains, domain)
}

// GetTrustedDomains returns the list of trusted domains
func (v *URLValidator) GetTrustedDomains() []string {
	return v.trustedDomains
}

// PathValidator validates file paths for security
type PathValidator struct{}

// NewPathValidator creates a new path validator
func NewPathValidator() *PathValidator {
	return &PathValidator{}
}

// ValidateTempPath validates that a path is safe for temporary file operations
func (v *PathValidator) ValidateTempPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path not allowed")
	}

	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Ensure the path is absolute
	if !filepath.IsAbs(cleanPath) {
		return fmt.Errorf("path must be absolute: %s", path)
	}

	// Get the system temp directory
	tempDir := os.TempDir()
	tempDirAbs, err := filepath.Abs(tempDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute temp directory: %w", err)
	}

	// Ensure the path is within the temp directory
	rel, err := filepath.Rel(tempDirAbs, cleanPath)
	if err != nil {
		return fmt.Errorf("failed to get relative path: %w", err)
	}

	// Check for directory traversal attempts
	if strings.HasPrefix(rel, "..") || strings.Contains(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("directory traversal attempt detected: %s", path)
	}

	return nil
}

// ValidateConfigPath validates configuration file paths
func (v *PathValidator) ValidateConfigPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path not allowed")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check for dangerous paths
	dangerousPaths := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"/root/",
		"/sys/",
		"/proc/",
		"/dev/",
	}

	for _, dangerous := range dangerousPaths {
		if strings.HasPrefix(cleanPath, dangerous) {
			return fmt.Errorf("access to dangerous path not allowed: %s", path)
		}
	}

	return nil
}

// InputSanitizer sanitizes user input
type InputSanitizer struct{}

// NewInputSanitizer creates a new input sanitizer
func NewInputSanitizer() *InputSanitizer {
	return &InputSanitizer{}
}

// SanitizeUserInput sanitizes user input to prevent command injection
func (s *InputSanitizer) SanitizeUserInput(input string) string {
	// Remove null bytes which can be used for injection
	input = strings.ReplaceAll(input, "\x00", "")

	// Remove control characters except common ones (tab, newline, carriage return)
	var sanitized strings.Builder
	for _, r := range input {
		if r == '\t' || r == '\n' || r == '\r' || !unicode.IsControl(r) {
			sanitized.WriteRune(r)
		}
	}

	// Trim whitespace to prevent padding attacks
	return strings.TrimSpace(sanitized.String())
}

// SanitizePassword specifically sanitizes password input
func (s *InputSanitizer) SanitizePassword(password string) *SecureString {
	// Basic sanitization for passwords - mostly just removing null bytes and control chars
	sanitized := s.SanitizeUserInput(password)
	return NewSecureString(sanitized)
}

// TempFileManager manages temporary files securely
type TempFileManager struct {
	pathValidator *PathValidator
}

// NewTempFileManager creates a new temporary file manager
func NewTempFileManager() *TempFileManager {
	return &TempFileManager{
		pathValidator: NewPathValidator(),
	}
}

// CreateSecureTempFile creates a temporary file with security validation
func (m *TempFileManager) CreateSecureTempFile(dir, pattern string) (*os.File, error) {
	// Create the temporary file
	tmpFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Validate the resulting path
	if err := m.pathValidator.ValidateTempPath(tmpFile.Name()); err != nil {
		_ = tmpFile.Close()           // Ignore close errors
		_ = os.Remove(tmpFile.Name()) // Ignore cleanup errors
		return nil, fmt.Errorf("temp file path validation failed: %w", err)
	}

	// Set secure permissions (readable/writable by owner only)
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		_ = tmpFile.Close()           // Ignore close errors
		_ = os.Remove(tmpFile.Name()) // Ignore cleanup errors
		return nil, fmt.Errorf("failed to set secure permissions: %w", err)
	}

	return tmpFile, nil
}

// CleanupTempFile removes a temporary file safely
func (m *TempFileManager) CleanupTempFile(path string) error {
	if path == "" {
		return nil // Nothing to clean
	}

	// Validate it's a temp file before removal
	if err := m.pathValidator.ValidateTempPath(path); err != nil {
		return fmt.Errorf("refusing to remove non-temp file: %w", err)
	}

	return os.Remove(path)
}

// ContentValidator validates file contents for security
type ContentValidator struct{}

// NewContentValidator creates a new content validator
func NewContentValidator() *ContentValidator {
	return &ContentValidator{}
}

// ValidateScriptContent validates script content for dangerous patterns
func (v *ContentValidator) ValidateScriptContent(content string, maxSize int64) error {
	// Basic sanity checks
	if len(content) == 0 {
		return fmt.Errorf("script is empty")
	}

	if int64(len(content)) > maxSize {
		return fmt.Errorf("script is too large: %d bytes (max: %d)", len(content), maxSize)
	}

	// Check for dangerous patterns
	dangerousPatterns := []string{
		"rm -rf /",
		"dd if=/dev/zero",
		"format c:",
		"mkfs.",
		"fdisk",
		"/etc/passwd",
		"/etc/shadow",
		"curl.*|.*sh",   // Nested curl pipes
		"wget.*|.*sh",   // Nested wget pipes
		":(){ :|:& };:", // Fork bomb
		"chmod 777 /",   // Dangerous permissions
		"chown root /",  // Dangerous ownership changes
		"> /dev/sd",     // Writing to disk devices
		"cryptsetup",    // Disk encryption tools
		"parted",        // Partition manipulation
		"mount /dev",    // Mounting devices
		"/dev/tcp",      // Network backdoors
	}

	lowerContent := strings.ToLower(content)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerContent, strings.ToLower(pattern)) {
			return fmt.Errorf("script contains potentially dangerous pattern: %s", pattern)
		}
	}

	return nil
}

// SecurityManager combines all security validators
type SecurityManager struct {
	URLValidator     *URLValidator
	PathValidator    *PathValidator
	InputSanitizer   *InputSanitizer
	TempFileManager  *TempFileManager
	ContentValidator *ContentValidator
}

// NewSecurityManager creates a comprehensive security manager
func NewSecurityManager(trustedDomains []string) *SecurityManager {
	return &SecurityManager{
		URLValidator:     NewURLValidator(trustedDomains),
		PathValidator:    NewPathValidator(),
		InputSanitizer:   NewInputSanitizer(),
		TempFileManager:  NewTempFileManager(),
		ContentValidator: NewContentValidator(),
	}
}

// DefaultTrustedDomains returns the default list of trusted domains
func DefaultTrustedDomains() []string {
	return []string{
		"mise.run",
		"mise.jdx.dev",
		"get.docker.com",
		"download.docker.com",
		"raw.githubusercontent.com",
	}
}
