// Package script handles downloading, validating, and executing installation scripts
package script

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// Config holds configuration for script operations
type Config struct {
	MaxScriptSize  int64         // Maximum allowed script size in bytes
	HTTPTimeout    time.Duration // Timeout for HTTP operations
	TrustedDomains []string      // List of trusted domains for downloads
}

// DefaultConfig returns default script configuration
func DefaultConfig() Config {
	return Config{
		MaxScriptSize: 5 * 1024 * 1024, // 5MB
		HTTPTimeout:   30 * time.Second,
		TrustedDomains: []string{
			"mise.run",
			"mise.jdx.dev",
			"get.docker.com",
			"download.docker.com",
			"raw.githubusercontent.com",
		},
	}
}

// Manager handles script operations
type Manager struct {
	config Config
	client *http.Client
}

// New creates a new script manager
func New(config Config) *Manager {
	return &Manager{
		config: config,
		client: &http.Client{
			Timeout: config.HTTPTimeout,
		},
	}
}

// NewWithDefaults creates a script manager with default configuration
func NewWithDefaults() *Manager {
	return New(DefaultConfig())
}

// ValidateURL validates that a URL is from a trusted domain
func (m *Manager) ValidateURL(urlStr string) error {
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
	for _, trustedDomain := range m.config.TrustedDomains {
		if hostname == trustedDomain {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not in trusted domains list", hostname)
}

// Download downloads a script from URL and saves it to a temporary file
func (m *Manager) Download(ctx context.Context, urlStr string) (string, error) {
	// Validate URL first
	if err := m.ValidateURL(urlStr); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	// Create temporary file
	tmpFile, err := m.createTempFile()
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Download script
	if err := m.downloadToFile(ctx, urlStr, tmpFile); err != nil {
		_ = os.Remove(tmpFile) // Ignore cleanup errors
		return "", err
	}

	return tmpFile, nil
}

// ValidateContent performs security validation on script content
func (m *Manager) ValidateContent(filepath string) error {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	scriptContent := string(content)

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

	lowerContent := strings.ToLower(scriptContent)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerContent, strings.ToLower(pattern)) {
			return fmt.Errorf("script contains potentially dangerous pattern: %s", pattern)
		}
	}

	// Basic sanity checks
	if len(scriptContent) == 0 {
		return fmt.Errorf("script is empty")
	}

	if int64(len(content)) > m.config.MaxScriptSize {
		return fmt.Errorf("script is too large: %d bytes (max: %d)", len(content), m.config.MaxScriptSize)
	}

	// Check for shebang (optional but good practice)
	if !strings.HasPrefix(scriptContent, "#!") {
		log.Warn("Script does not start with shebang - may not be a shell script")
	}

	return nil
}

// DownloadAndValidate downloads a script and validates its content
func (m *Manager) DownloadAndValidate(ctx context.Context, urlStr string) (string, error) {
	scriptPath, err := m.Download(ctx, urlStr)
	if err != nil {
		return "", err
	}

	if err := m.ValidateContent(scriptPath); err != nil {
		_ = os.Remove(scriptPath) // Ignore cleanup errors
		return "", fmt.Errorf("script validation failed: %w", err)
	}

	return scriptPath, nil
}

// Cleanup removes a temporary script file
func (m *Manager) Cleanup(filepath string) {
	if filepath == "" {
		return
	}

	if err := os.Remove(filepath); err != nil {
		log.Warn("Failed to cleanup temp script", "path", filepath, "error", err)
	} else {
		log.Debug("Cleaned up temp script", "path", filepath)
	}
}

// createTempFile creates a secure temporary file for the script
func (m *Manager) createTempFile() (string, error) {
	tmpFile, err := os.CreateTemp("", "devex-install-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Validate the resulting path
	if err := m.validateTempPath(tmpFile.Name()); err != nil {
		_ = tmpFile.Close()           // Ignore close errors
		_ = os.Remove(tmpFile.Name()) // Ignore cleanup errors
		return "", fmt.Errorf("temp file path validation failed: %w", err)
	}

	// Set secure permissions (readable/writable/executable by owner only)
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		_ = tmpFile.Close()           // Ignore close errors
		_ = os.Remove(tmpFile.Name()) // Ignore cleanup errors
		return "", fmt.Errorf("failed to set secure permissions: %w", err)
	}

	fileName := tmpFile.Name()
	_ = tmpFile.Close() // Ignore close errors
	return fileName, nil
}

// validateTempPath validates that a path is safe for temporary file operations
func (m *Manager) validateTempPath(path string) error {
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

// downloadToFile downloads content from URL to a file
func (m *Manager) downloadToFile(ctx context.Context, urlStr, filepath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download script: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Open file for writing
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	// Copy content with size limit
	_, err = io.CopyN(file, resp.Body, m.config.MaxScriptSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to write script: %w", err)
	}

	return nil
}

// AddTrustedDomain adds a domain to the trusted list
func (m *Manager) AddTrustedDomain(domain string) {
	m.config.TrustedDomains = append(m.config.TrustedDomains, domain)
}

// GetTrustedDomains returns the list of trusted domains
func (m *Manager) GetTrustedDomains() []string {
	return m.config.TrustedDomains
}
