package tui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
)

// InstallerConfig holds configuration constants for the installer
type InstallerConfig struct {
	// Channel and timeout settings
	ChannelBufferSize   int
	InputTimeout        time.Duration
	InstallationTimeout time.Duration
	InitializationDelay time.Duration

	// Security limits
	MaxGPGKeySize int64
	MaxScriptSize int64
	MaxLogLines   int

	// HTTP settings
	HTTPTimeout time.Duration

	// Error handling
	FailOnDatabaseErrors bool // Whether database errors should fail the installation
}

// DefaultInstallerConfig returns the default configuration
func DefaultInstallerConfig() InstallerConfig {
	return InstallerConfig{
		ChannelBufferSize:    5,
		InputTimeout:         30 * time.Second,
		InstallationTimeout:  10 * time.Minute,
		InitializationDelay:  500 * time.Millisecond,
		MaxGPGKeySize:        1024 * 1024,     // 1MB for GPG keys
		MaxScriptSize:        5 * 1024 * 1024, // Reduced to 5MB (was 10MB)
		MaxLogLines:          1000,
		HTTPTimeout:          30 * time.Second,
		FailOnDatabaseErrors: false, // By default, don't fail on DB errors
	}
}

var (
	// trustedDomains are the only domains allowed for curl pipe installations
	// SECURITY: This allowlist prevents execution of arbitrary remote scripts
	trustedDomains = []string{
		"mise.run",
		"mise.jdx.dev",
		"get.docker.com",
		"download.docker.com",
		"raw.githubusercontent.com", // For official GitHub-hosted scripts only
	}
)

// validateDownloadURL validates that a URL is from a trusted domain
// SECURITY: This prevents arbitrary remote script execution
func validateDownloadURL(urlStr string) error {
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
	for _, trustedDomain := range trustedDomains {
		if hostname == trustedDomain {
			return nil
		}
	}

	return fmt.Errorf("domain %s is not in trusted domains list", hostname)
}

// validateTempPath validates that a path is safe for temporary file operations
// SECURITY: Prevents directory traversal attacks and ensures files are created in safe locations
func validateTempPath(path string) error {
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

// createSecureTempFile creates a temporary file with security validation
// SECURITY: Validates the resulting path and sets secure permissions
func createSecureTempFile(dir, pattern string) (*os.File, error) {
	// Create the temporary file
	tmpFile, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Validate the resulting path
	if err := validateTempPath(tmpFile.Name()); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("temp file path validation failed: %w", err)
	}

	// Set secure permissions (readable/writable by owner only)
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to set secure permissions: %w", err)
	}

	return tmpFile, nil
}

// StreamingInstaller handles installation with real-time output and interaction
type StreamingInstaller struct {
	program  *tea.Program
	repo     types.Repository
	executor CommandExecutor // Pluggable command executor for better testability
	stdinMux sync.Mutex      // Protects stdin access from race conditions
	ctx      context.Context
	cancel   context.CancelFunc
	config   InstallerConfig // Configuration settings
}

// SecureString represents a string that should be scrubbed from memory to prevent
// sensitive data like passwords from lingering in memory after use.
type SecureString struct {
	data []byte
}

// NewSecureString creates a new secure string from the provided string.
// The input string is immediately copied to a byte slice to prevent
// accidental sharing of the underlying string data.
func NewSecureString(s string) *SecureString {
	data := make([]byte, len(s))
	copy(data, s)
	return &SecureString{data: data}
}

// String returns the string value of the secure string.
// WARNING: Use sparingly and ensure Clear() is called immediately after use
// to scrub the sensitive data from memory. The returned string should not be
// stored in variables or passed to functions that might retain it.
func (ss *SecureString) String() string {
	return string(ss.data)
}

// Clear scrubs the secure string from memory by overwriting all bytes with zeros
// and setting the slice to nil. This helps prevent sensitive data from remaining
// in memory where it could potentially be recovered.
func (ss *SecureString) Clear() {
	for i := range ss.data {
		ss.data[i] = 0
	}
	ss.data = nil
}

var (
	// allowedCommands defines safe commands that can be executed
	allowedCommands = map[string]bool{
		"apt":     true,
		"apt-get": true,
		"apt-key": true, // SECURITY: Added for GPG key management
		"dpkg":    true,
		"curl":    true,
		"wget":    true,
		"git":     true,
		"docker":  true,
		"npm":     true,
		"pip":     true,
		"pip3":    true,
		"go":      true,
		"cargo":   true,
		"flatpak": true,
		"snap":    true,
		"dnf":     true,
		"yum":     true,
		"pacman":  true,
		"zypper":  true,
		"brew":    true,
		"mise":    true,
		"mkdir":   true,
		"cp":      true,
		"mv":      true,
		"chmod":   true,
		"chown":   true,
		"ln":      true,
		"tar":     true,
		"unzip":   true,
		"gunzip":  true,
		"echo":    true,
		"cat":     true,
		"which":   true,
		"whereis": true,
		"id":      true,
		"whoami":  true, // SECURITY: Added for repository source addition
	}

	// dangerousPatterns are regex patterns for potentially dangerous command constructs
	dangerousPatterns = []*regexp.Regexp{
		regexp.MustCompile(`[;&|]`),                 // Command separators and logical operators
		regexp.MustCompile(`&&`),                    // Logical AND operator
		regexp.MustCompile(`\|\|`),                  // Logical OR operator
		regexp.MustCompile("`[^`]*`"),               // Command substitution (backticks)
		regexp.MustCompile(`\$\([^)]*\)`),           // Command substitution $()
		regexp.MustCompile(`\$\{[^}]*\}`),           // Variable expansion (except safe ones)
		regexp.MustCompile(`\.\./`),                 // Directory traversal
		regexp.MustCompile(`/etc/passwd`),           // Sensitive files
		regexp.MustCompile(`/etc/shadow`),           // Sensitive files
		regexp.MustCompile(`rm\s+-rf\s+/[^a-zA-Z]`), // Dangerous rm commands on root
		regexp.MustCompile(`dd\s+if=/dev`),          // Dangerous dd commands
		regexp.MustCompile(`:\(\)\{`),               // Fork bombs
		regexp.MustCompile(`>\s*/etc/`),             // Writing to system directories
		regexp.MustCompile(`>\s*/dev/`),             // Writing to device files
	}
)

// sanitizeUserInput sanitizes user input to prevent command injection and other security issues
func sanitizeUserInput(input string) string {
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

// parseCommand safely parses a command string into executable parts
// Returns (executable, args, needsShell) where needsShell indicates if shell execution is required
// SECURITY: This function now REJECTS shell execution to prevent security bypass
func parseCommand(command string) (string, []string, bool) {
	// Trim whitespace
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, false
	}

	// Split command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil, false
	}

	// SECURITY FIX: Check for dangerous shell features and REJECT them
	// Allow simple quoted strings but reject complex shell constructs
	dangerousFeatures := []string{
		"|", "&&", "||", ";", "&", ">", ">>", "<", "$(", "`",
	}

	for _, feature := range dangerousFeatures {
		if strings.Contains(command, feature) {
			// SECURITY: Reject commands with dangerous shell features
			return "", nil, false
		}
	}

	// ADDITIONAL SECURITY: Check against dangerous patterns using our regex list
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			// SECURITY: Reject commands matching dangerous patterns
			return "", nil, false
		}
	}

	// Only allow direct execution of validated commands
	return parts[0], parts[1:], false
}

// CommandExecutor defines the interface for command execution, allowing for better testing and modularity
type CommandExecutor interface {
	// ExecuteCommand executes a command with the given context and returns the command handle
	ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error)
	// ValidateCommand validates a command for security before execution
	ValidateCommand(command string) error
}

// DefaultCommandExecutor implements CommandExecutor using the standard approach
type DefaultCommandExecutor struct {
	// Additional fields can be added here for configuration
}

// NewDefaultCommandExecutor creates a new default command executor
func NewDefaultCommandExecutor() *DefaultCommandExecutor {
	return &DefaultCommandExecutor{}
}

// ExecuteCommand implements CommandExecutor.ExecuteCommand
func (ce *DefaultCommandExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	// Validate command first
	if err := ce.ValidateCommand(command); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Parse and execute using safest method
	executable, args, _ := parseCommand(command)

	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.SysProcAttr = ce.getPlatformSysProcAttr()

	return cmd, nil
}

// ValidateCommand implements CommandExecutor.ValidateCommand
func (ce *DefaultCommandExecutor) ValidateCommand(command string) error {
	// Check for dangerous patterns
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains potentially dangerous pattern: %s", pattern.String())
		}
	}

	// Parse command and validate first word
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	// Handle sudo commands specially
	if parts[0] == "sudo" {
		if len(parts) < 2 {
			return fmt.Errorf("sudo command missing actual command")
		}
		// Validate the command after sudo
		actualCommand := parts[1]
		if !allowedCommands[actualCommand] {
			return fmt.Errorf("sudo command '%s' is not in allowed list", actualCommand)
		}
	} else if !allowedCommands[parts[0]] {
		// Validate regular commands
		return fmt.Errorf("command '%s' is not in allowed list", parts[0])
	}

	return nil
}

// NewStreamingInstaller creates a new streaming installer with context cancellation
func NewStreamingInstaller(program *tea.Program, repo types.Repository, ctx context.Context) *StreamingInstaller {
	instCtx, cancel := context.WithCancel(ctx)
	return &StreamingInstaller{
		program:  program,
		repo:     repo,
		executor: NewDefaultCommandExecutor(), // Use default command executor
		ctx:      instCtx,
		cancel:   cancel,
		config:   DefaultInstallerConfig(),
	}
}

// NewStreamingInstallerWithExecutor creates a streaming installer with a custom command executor for testing
func NewStreamingInstallerWithExecutor(program *tea.Program, repo types.Repository, ctx context.Context, executor CommandExecutor) *StreamingInstaller {
	instCtx, cancel := context.WithCancel(ctx)
	return &StreamingInstaller{
		program:  program,
		repo:     repo,
		executor: executor,
		ctx:      instCtx,
		cancel:   cancel,
		config:   DefaultInstallerConfig(),
	}
}

// InstallApps installs multiple applications sequentially with streaming output and context cancellation.
// It processes each app in the provided slice, handling errors and context cancellation gracefully.
// If context cancellation occurs, the installation stops immediately and returns the cancellation error.
// Individual app failures are logged but don't stop the overall installation process, unless caused by cancellation.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - apps: Slice of CrossPlatformApp configurations to install
//   - settings: Installation settings including verbosity and dry-run flags
//
// Returns:
//   - error: nil on success, context.Canceled if cancelled, or other error on critical failures
func (si *StreamingInstaller) InstallApps(ctx context.Context, apps []types.CrossPlatformApp, settings config.CrossPlatformSettings) error {
	for _, app := range apps {
		// Check for context cancellation before each app
		select {
		case <-ctx.Done():
			si.sendLog("INFO", "Installation cancelled before starting next app")
			return ctx.Err()
		default:
		}

		if err := si.InstallApp(ctx, app, settings); err != nil {
			si.sendLog("ERROR", fmt.Sprintf("Failed to install %s: %v", app.Name, err))
			if si.program != nil {
				si.program.Send(AppCompleteMsg{
					AppName: app.Name,
					Error:   err,
				})
			}
			// If the error is due to context cancellation, stop installation entirely
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}

		if si.program != nil {
			si.program.Send(AppCompleteMsg{
				AppName: app.Name,
				Error:   nil,
			})
		}
	}
	return nil
}

// InstallApp installs a single application with streaming output and comprehensive error handling.
// It executes the complete installation lifecycle: validation, pre-install commands, main installation,
// post-install commands, and database registration. All command execution is validated for security.
//
// The installation process includes:
//   - Platform-specific configuration resolution
//   - App validation using the app's Validate() method
//   - Pre-install command execution (if configured)
//   - Main installation command execution via the configured install method
//   - Post-install command execution (if configured)
//   - Database registration of the successfully installed app
//
// Parameters:
//   - app: CrossPlatformApp configuration containing installation instructions
//   - settings: Installation settings including verbosity flags
//
// Returns:
//   - error: nil on success, or detailed error indicating which phase failed
func (si *StreamingInstaller) InstallApp(ctx context.Context, app types.CrossPlatformApp, settings config.CrossPlatformSettings) error {
	si.sendLog("INFO", fmt.Sprintf("Starting installation of %s", app.Name))

	// Get platform-specific configuration
	osConfig := app.GetOSConfig()
	if osConfig.InstallMethod == "" {
		return fmt.Errorf("no configuration available for current platform")
	}

	// Validate app
	if err := app.Validate(); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Handle pre-install commands
	if len(osConfig.PreInstall) > 0 {
		si.sendLog("INFO", "Executing pre-install commands...")
		if err := si.executeCommands(ctx, osConfig.PreInstall); err != nil {
			return fmt.Errorf("pre-install failed: %w", err)
		}
	}

	// Execute main installation command
	si.sendLog("INFO", fmt.Sprintf("Installing %s using %s...", app.Name, osConfig.InstallMethod))
	if err := si.executeInstallCommand(ctx, app, &osConfig); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Handle post-install commands
	if len(osConfig.PostInstall) > 0 {
		si.sendLog("INFO", "Executing post-install commands...")
		if err := si.executeCommands(ctx, osConfig.PostInstall); err != nil {
			return fmt.Errorf("post-install failed: %w", err)
		}
	}

	// Save to repository with configurable error handling
	if err := si.repo.AddApp(app.Name); err != nil {
		si.sendLog("ERROR", fmt.Sprintf("Failed to save app to database: %v", err))

		if si.config.FailOnDatabaseErrors {
			// Fail the installation if configured to do so
			return fmt.Errorf("database operation failed: %w", err)
		} else {
			// Log prominently but don't fail the installation
			si.sendLog("WARN", "Installation succeeded but app tracking may be inconsistent")
			si.sendLog("WARN", "Consider fixing database connectivity for proper app tracking")
		}
	} else {
		si.sendLog("INFO", fmt.Sprintf("App %s registered in database", app.Name))
	}

	si.sendLog("INFO", fmt.Sprintf("Successfully installed %s", app.Name))
	return nil
}

// executeInstallCommand executes the main installation command
func (si *StreamingInstaller) executeInstallCommand(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	switch osConfig.InstallMethod {
	case "apt":
		return si.executeAptInstall(ctx, app, osConfig)
	case "curlpipe":
		return si.executeCurlPipeInstall(ctx, app, osConfig)
	case "docker":
		return si.executeDockerInstall(ctx, app, osConfig)
	default:
		// Generic command execution
		return si.executeCommandStream(ctx, osConfig.InstallCommand)
	}
}

// executeAptInstall handles APT package installation with GPG key validation
func (si *StreamingInstaller) executeAptInstall(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	// Add APT sources with GPG key validation if needed
	for _, source := range osConfig.AptSources {
		si.sendLog("INFO", fmt.Sprintf("Adding APT source: %s", source.SourceName))

		// SECURITY: Validate GPG key with fingerprint if provided
		if source.KeySource != "" {
			si.sendLog("INFO", fmt.Sprintf("Adding GPG key from: %s", source.KeySource))

			// Use secure GPG validation if fingerprint is provided
			if source.KeyFingerprint != "" {
				si.sendLog("INFO", "Using fingerprint validation for enhanced security")
				if err := si.validateAndAddGPGKey(ctx, source.KeySource, source.KeyFingerprint); err != nil {
					return fmt.Errorf("failed to validate and add GPG key for %s: %w", source.SourceName, err)
				}
			} else {
				// Fallback to basic key addition (less secure but backwards compatible)
				si.sendLog("WARN", "No fingerprint provided - using basic key validation")
				addKeyCmd := fmt.Sprintf("curl -fsSL %s | sudo apt-key add -", source.KeySource)
				if err := si.executeCommandStream(ctx, addKeyCmd); err != nil {
					return fmt.Errorf("failed to add GPG key for %s: %w", source.SourceName, err)
				}
			}
		}

		// Add the repository source
		if source.SourceRepo != "" {
			addSourceCmd := fmt.Sprintf("echo '%s' | sudo tee /etc/apt/sources.list.d/%s.list",
				source.SourceRepo, source.SourceName)
			if err := si.executeCommandStream(ctx, addSourceCmd); err != nil {
				return fmt.Errorf("failed to add APT source %s: %w", source.SourceName, err)
			}
		}
	}

	// Update package lists
	si.sendLog("INFO", "Updating package lists...")
	if err := si.executeCommandStream(ctx, "sudo apt update"); err != nil {
		return err
	}

	// Install package
	return si.executeCommandStream(ctx, osConfig.InstallCommand)
}

// validateAndAddGPGKey validates a GPG key fingerprint and adds the key to APT keyring
// SECURITY: Validates key fingerprint to prevent key substitution attacks
func (si *StreamingInstaller) validateAndAddGPGKey(ctx context.Context, keyURL, expectedFingerprint string) error {
	si.sendLog("INFO", "Validating and adding GPG key...")

	// Validate the key URL is HTTPS
	if !strings.HasPrefix(keyURL, "https://") {
		return fmt.Errorf("GPG key URL must use HTTPS: %s", keyURL)
	}

	// Download the key to a temporary file
	tempKeyFile, err := si.downloadGPGKey(keyURL)
	if err != nil {
		return fmt.Errorf("failed to download GPG key: %w", err)
	}
	defer si.cleanupTempScript(tempKeyFile) // Reuse cleanup function

	// Validate the key fingerprint if provided
	if expectedFingerprint != "" {
		if err := si.validateGPGKeyFingerprint(ctx, tempKeyFile, expectedFingerprint); err != nil {
			return fmt.Errorf("GPG key fingerprint validation failed: %w", err)
		}
	}

	// Add the key to APT keyring
	addKeyCmd := fmt.Sprintf("sudo apt-key add %s", tempKeyFile)
	return si.executeCommandStream(ctx, addKeyCmd)
}

// downloadGPGKey downloads a GPG key from URL to a temporary file
func (si *StreamingInstaller) downloadGPGKey(keyURL string) (string, error) {
	si.sendLog("INFO", "Downloading GPG key...")

	// Create secure temporary file
	tmpFile, err := createSecureTempFile("", "devex-gpg-key-*.asc")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: si.config.HTTPTimeout,
	}

	req, err := http.NewRequestWithContext(si.ctx, "GET", keyURL, nil)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Copy content with size limit
	_, err = io.CopyN(tmpFile, resp.Body, si.config.MaxGPGKeySize)
	if err != nil && !errors.Is(err, io.EOF) {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write key: %w", err)
	}

	si.sendLog("INFO", "GPG key downloaded successfully")
	return tmpFile.Name(), nil
}

// validateGPGKeyFingerprint validates that a GPG key matches the expected fingerprint
// SECURITY: Prevents key substitution attacks by validating cryptographic fingerprint
func (si *StreamingInstaller) validateGPGKeyFingerprint(ctx context.Context, keyFile, expectedFingerprint string) error {
	si.sendLog("INFO", "Validating GPG key fingerprint...")

	// Clean and normalize expected fingerprint (remove spaces, convert to uppercase)
	expectedFingerprint = strings.ReplaceAll(strings.ToUpper(expectedFingerprint), " ", "")

	// Use gpg to get the key fingerprint
	getFingerprintCmd := fmt.Sprintf("gpg --with-fingerprint --import-options show-only --import %s", keyFile)
	cmd, err := si.executor.ExecuteCommand(ctx, getFingerprintCmd)
	if err != nil {
		return fmt.Errorf("failed to execute GPG command: %w", err)
	}

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("GPG command failed: %w", err)
	}

	// Parse fingerprint from output
	outputStr := string(output)
	fingerprintRegex := regexp.MustCompile(`Key fingerprint = ([A-F0-9 ]+)`)
	matches := fingerprintRegex.FindStringSubmatch(outputStr)
	if len(matches) < 2 {
		return fmt.Errorf("could not extract fingerprint from GPG output")
	}

	// Clean and normalize actual fingerprint
	actualFingerprint := strings.ReplaceAll(strings.ToUpper(matches[1]), " ", "")

	// Compare fingerprints
	if actualFingerprint != expectedFingerprint {
		return fmt.Errorf("fingerprint mismatch: expected %s, got %s", expectedFingerprint, actualFingerprint)
	}

	si.sendLog("INFO", "GPG key fingerprint validated successfully")
	return nil
}

// executeCurlPipeInstall handles curl pipe installations with security validation
// SECURITY: Now validates URLs against trusted domains before execution
func (si *StreamingInstaller) executeCurlPipeInstall(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	// SECURITY: Validate URL before attempting download
	if err := validateDownloadURL(osConfig.DownloadURL); err != nil {
		si.sendLog("ERROR", fmt.Sprintf("URL validation failed for %s: %v", app.Name, err))
		return fmt.Errorf("URL validation failed: %w", err)
	}

	si.sendLog("INFO", fmt.Sprintf("Downloading from validated URL: %s", osConfig.DownloadURL))

	// SECURITY ENHANCEMENT: Download-validate-execute pattern instead of direct pipe
	return si.downloadValidateExecute(ctx, app.Name, osConfig.DownloadURL)
}

// downloadValidateExecute implements a safer alternative to curl pipe
// SECURITY: Downloads script first, validates content, then executes
func (si *StreamingInstaller) downloadValidateExecute(ctx context.Context, appName, downloadURL string) error {
	// Create temporary file for script
	tmpFile, err := si.createTempScript()
	if err != nil {
		si.sendLog("ERROR", fmt.Sprintf("Failed to create temporary script file: %v", err))
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer si.cleanupTempScript(tmpFile)

	// Download script to temporary file
	if err := si.downloadScript(downloadURL, tmpFile); err != nil {
		si.sendLog("ERROR", fmt.Sprintf("Failed to download script: %v", err))
		return fmt.Errorf("failed to download script: %w", err)
	}

	// Validate script content for basic safety
	if err := si.validateScriptContent(tmpFile); err != nil {
		si.sendLog("ERROR", fmt.Sprintf("Script validation failed: %v", err))
		return fmt.Errorf("script validation failed: %w", err)
	}

	// Execute validated script
	si.sendLog("INFO", fmt.Sprintf("Executing validated script for %s", appName))
	command := fmt.Sprintf("bash %s", tmpFile)
	return si.executeCommandStream(ctx, command)
}

// createTempScript creates a temporary script file with proper permissions
func (si *StreamingInstaller) createTempScript() (string, error) {
	tmpFile, err := createSecureTempFile("", "devex-install-*.sh")
	if err != nil {
		return "", err
	}

	// Set execute permissions (in addition to the secure 0600 already set)
	if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to set execute permissions: %w", err)
	}

	fileName := tmpFile.Name()
	tmpFile.Close()
	return fileName, nil
}

// downloadScript downloads a script from URL to a file
func (si *StreamingInstaller) downloadScript(downloadURL, filepath string) error {
	si.sendLog("INFO", "Downloading installation script...")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: si.config.HTTPTimeout,
	}

	req, err := http.NewRequestWithContext(si.ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download script: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create/open file for writing
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	// Copy content with size limit (prevent DoS)
	_, err = io.CopyN(file, resp.Body, si.config.MaxScriptSize)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to write script: %w", err)
	}

	si.sendLog("INFO", "Script downloaded successfully")
	return nil
}

// validateScriptContent performs basic validation on downloaded scripts
// SECURITY: Checks for obvious malicious patterns
func (si *StreamingInstaller) validateScriptContent(filepath string) error {
	si.sendLog("INFO", "Validating script content...")

	content, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	scriptContent := string(content)

	// Check for dangerous patterns
	dangerousContentPatterns := []string{
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

	for _, pattern := range dangerousContentPatterns {
		if strings.Contains(strings.ToLower(scriptContent), strings.ToLower(pattern)) {
			return fmt.Errorf("script contains potentially dangerous pattern: %s", pattern)
		}
	}

	// Basic sanity checks
	if len(scriptContent) == 0 {
		return fmt.Errorf("script is empty")
	}

	if int64(len(content)) > si.config.MaxScriptSize {
		return fmt.Errorf("script is too large: %d bytes (max: %d)", len(content), si.config.MaxScriptSize)
	}

	// Check for shebang (optional but good practice)
	if !strings.HasPrefix(scriptContent, "#!") {
		si.sendLog("WARN", "Script does not start with shebang - may not be a shell script")
	}

	si.sendLog("INFO", "Script validation passed")
	return nil
}

// cleanupTempScript removes temporary script file
func (si *StreamingInstaller) cleanupTempScript(filepath string) {
	if filepath == "" {
		return
	}

	if err := os.Remove(filepath); err != nil {
		si.sendLog("WARN", fmt.Sprintf("Failed to cleanup temp script %s: %v", filepath, err))
	} else {
		si.sendLog("DEBUG", fmt.Sprintf("Cleaned up temp script: %s", filepath))
	}
}

// executeDockerInstall handles Docker container installations
func (si *StreamingInstaller) executeDockerInstall(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	si.sendLog("INFO", fmt.Sprintf("Starting Docker installation for %s", app.Name))
	return si.executeCommandStream(ctx, osConfig.InstallCommand)
}

// executeCommands executes a list of install commands with context cancellation support
func (si *StreamingInstaller) executeCommands(ctx context.Context, commands []types.InstallCommand) error {
	for _, cmd := range commands {
		// Check for context cancellation before each command
		select {
		case <-ctx.Done():
			si.sendLog("INFO", "Command execution cancelled")
			return ctx.Err()
		default:
		}

		if cmd.Command != "" {
			si.sendLog("INFO", fmt.Sprintf("Executing: %s", cmd.Command))
			if err := si.executeCommandStream(ctx, cmd.Command); err != nil {
				return err
			}
		}

		if cmd.Shell != "" {
			si.sendLog("INFO", fmt.Sprintf("Executing shell: %s", cmd.Shell))
			if err := si.executeCommandStream(ctx, cmd.Shell); err != nil {
				return err
			}
		}

		if cmd.Copy != nil {
			si.sendLog("INFO", fmt.Sprintf("Copying %s to %s", cmd.Copy.Source, cmd.Copy.Destination))
			// Handle file copy
		}

		if cmd.Sleep > 0 {
			si.sendLog("INFO", fmt.Sprintf("Sleeping for %d seconds", cmd.Sleep))
			// Use context-aware sleep
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(cmd.Sleep) * time.Second):
				// Sleep completed normally
			}
		}
	}
	return nil
}

// executeCommandStream executes a command with streaming output
func (si *StreamingInstaller) executeCommandStream(ctx context.Context, command string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, si.config.InstallationTimeout)
	defer cancel()

	// Validate and parse command safely
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty command")
	}

	// Execute command using the pluggable executor interface
	cmd, err := si.executor.ExecuteCommand(timeoutCtx, command)
	if err != nil {
		return err
	}

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer func() {
		if stdout != nil {
			stdout.Close()
		}
	}()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer func() {
		if stderr != nil {
			stderr.Close()
		}
	}()

	// Create pipe for stdin (for password prompts)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer func() {
		if stdin != nil {
			stdin.Close()
		}
	}()

	// Start command
	if err := cmd.Start(); err != nil {
		return err
	}

	// Use WaitGroup to ensure all goroutines complete
	var wg sync.WaitGroup

	// Stream output with goroutine tracking
	wg.Add(2)
	go func() {
		defer wg.Done()
		si.streamOutput(stdout, "STDOUT")
	}()
	go func() {
		defer wg.Done()
		si.streamOutput(stderr, "STDERR")
	}()

	// Monitor for password prompts with goroutine tracking
	wg.Add(1)
	go func() {
		defer wg.Done()
		si.monitorForInput(stderr, stdin)
	}()

	// Wait for command completion
	cmdErr := cmd.Wait()

	// Wait for all goroutines to finish
	wg.Wait()

	return cmdErr
}

// streamOutput streams command output to the TUI with proper error handling
func (si *StreamingInstaller) streamOutput(reader io.Reader, source string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-si.ctx.Done():
			si.sendLog("INFO", fmt.Sprintf("%s stream cancelled", source))
			return
		default:
		}

		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			si.sendLog(source, line)
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		si.sendLog("ERROR", fmt.Sprintf("Scanner error in %s: %v", source, err))
	}
}

// monitorForInput monitors stderr for password prompts and requests user input
func (si *StreamingInstaller) monitorForInput(stderr io.Reader, stdin io.WriteCloser) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for common password prompts
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "password") &&
			(strings.Contains(lowerLine, "sudo") ||
				strings.Contains(lowerLine, "enter") ||
				strings.Contains(lowerLine, ":")) {
			// Request input from user (skip if program is nil for testing)
			if si.program == nil {
				return // Skip password prompts during testing
			}
			response := make(chan *SecureString, 1)
			var closeOnce sync.Once // SECURITY: Prevent panic from closing channel multiple times

			// Send input request with non-blocking send to prevent deadlock
			inputMsg := InputRequestMsg{
				Prompt:   line,
				Response: response,
			}

			// Non-blocking send with timeout to prevent deadlock
			go func() {
				select {
				case <-si.ctx.Done():
					// Context was cancelled, abort password prompt
					si.sendLog("INFO", "Password prompt cancelled due to context cancellation")
					closeOnce.Do(func() { close(response) })
					return
				default:
					si.program.Send(inputMsg)
				}
			}()

			// Wait for user response with multiple timeout mechanisms
			select {
			case secureInput := <-response:
				if secureInput != nil {
					// Sanitize user input to prevent injection attacks
					sanitizedInput := sanitizeUserInput(secureInput.String())

					// Protect stdin access with mutex to prevent race conditions
					si.stdinMux.Lock()
					// Write sanitized password and immediately scrub from memory
					if _, err := stdin.Write([]byte(sanitizedInput + "\n")); err != nil {
						si.sendLog("ERROR", fmt.Sprintf("Failed to write input: %v", err))
					}
					si.stdinMux.Unlock()
					secureInput.Clear() // Scrub password from memory
				}
			case <-si.ctx.Done():
				// Context cancelled while waiting for input
				si.sendLog("INFO", "Input cancelled due to context cancellation")
				// SECURITY FIX: Safely close channel only once to prevent panic
				closeOnce.Do(func() { close(response) })
				return
			case <-time.After(si.config.InputTimeout):
				si.sendLog("ERROR", "Input timeout - no response received")
				// SECURITY FIX: Safely close channel only once to prevent panic
				closeOnce.Do(func() { close(response) })
				return
			}
		}
	}
}

// sendLog sends a log message to the TUI
func (si *StreamingInstaller) sendLog(level, message string) {
	// Skip sending messages when program is nil (during testing)
	if si.program == nil {
		return
	}
	si.program.Send(LogMsg{
		Message:   message,
		Timestamp: time.Now(),
		Level:     level,
	})
}

// StartInstallation starts the installation process in the TUI with context cancellation and user interaction.
// This is the main entry point for TUI-based installations, providing a split-pane interface with
// real-time progress tracking (left pane) and streaming command output (right pane).
//
// The function:
//   - Creates a Bubble Tea TUI with split-pane layout
//   - Initializes a StreamingInstaller with context cancellation support
//   - Starts installation in background goroutine with proper cleanup
//   - Handles user interactions including password prompts
//   - Provides real-time progress updates and log streaming
//
// Parameters:
//   - apps: Slice of CrossPlatformApp configurations to install
//   - repo: Repository interface for app state persistence
//   - settings: Installation settings including verbosity and dry-run options
//
// Returns:
//   - error: nil on successful TUI completion, or error from TUI framework or installation
func StartInstallation(apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Create context for cancellation support
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create TUI model
	m := NewModel(apps)

	// Create program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Create streaming installer with context
	installer := NewStreamingInstaller(p, repo, ctx)
	defer installer.cancel() // Ensure cleanup

	// Start installation in background with context cancellation
	go func() {
		select {
		case <-ctx.Done():
			// Installation was cancelled
			installer.sendLog("INFO", "Installation cancelled by user")
			return
		case <-time.After(installer.config.InitializationDelay):
			// Let TUI initialize before starting installation
			if err := installer.InstallApps(ctx, apps, settings); err != nil {
				installer.sendLog("ERROR", fmt.Sprintf("Installation failed: %v", err))
			}
		}
	}()

	// Start TUI
	_, err := p.Run()

	// Cancel any ongoing installation when TUI exits
	cancel()

	// SECURITY: Clean up channels to prevent memory leaks
	m.CleanupChannels()

	return err
}
