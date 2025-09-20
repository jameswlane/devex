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
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/performance"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	progresspkg "github.com/jameswlane/devex/apps/cli/internal/progress"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// Command constants for improved maintainability
const (
	// Package managers
	CmdApt              = "apt"
	CmdAptGet           = "apt-get"
	CmdAptKey           = "apt-key"
	CmdAptCache         = "apt-cache"
	CmdAddAptRepository = "add-apt-repository"
	CmdDpkg             = "dpkg"
	CmdDnf              = "dnf"
	CmdYum              = "yum"
	CmdPacman           = "pacman"
	CmdZypper           = "zypper"
	CmdFlatpak          = "flatpak"
	CmdSnap             = "snap"
	CmdBrew             = "brew"

	// Development tools
	CmdGit    = "git"
	CmdDocker = "docker"
	CmdNpm    = "npm"
	CmdPip    = "pip"
	CmdPip3   = "pip3"
	CmdGo     = "go"
	CmdCargo  = "cargo"
	CmdMise   = "mise"

	// System utilities
	CmdCurl  = "curl"
	CmdWget  = "wget"
	CmdBash  = "bash"
	CmdSh    = "sh"
	CmdMkdir = "mkdir"
	CmdCp    = "cp"
	CmdMv    = "mv"
	CmdChmod = "chmod"
	CmdChown = "chown"
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
		if closeErr := tmpFile.Close(); closeErr != nil {
			log.Warn("Failed to close temp file during cleanup", "error", closeErr)
		}
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			log.Warn("Failed to remove temp file during cleanup", "error", removeErr)
		}
		return nil, fmt.Errorf("temp file path validation failed: %w", err)
	}

	// Set secure permissions (readable/writable by owner only)
	if err := os.Chmod(tmpFile.Name(), 0600); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			log.Warn("Failed to close temp file during cleanup", "error", closeErr)
		}
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			log.Warn("Failed to remove temp file during cleanup", "error", removeErr)
		}
		return nil, fmt.Errorf("failed to set secure permissions: %w", err)
	}

	return tmpFile, nil
}

// StreamingInstaller handles installation with real-time output and interaction
type StreamingInstaller struct {
	program             *tea.Program
	repo                types.Repository
	executor            CommandExecutor // Pluggable command executor for better testability
	stdinMux            sync.Mutex      // Protects stdin access from race conditions
	repoMutex           sync.RWMutex    // Protects repository access from race conditions
	ctx                 context.Context
	cancel              context.CancelFunc
	config              InstallerConfig                  // Configuration settings
	performanceAnalyzer *performance.PerformanceAnalyzer // Performance analysis and warnings
	progressManager     *progresspkg.ProgressManager     // Optional progress manager for enhanced tracking
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

// Legacy validation variables removed - now using security package
//
//nolint:unused // Legacy code will be removed in next cleanup
var (
	allowedCommands = map[string]bool{
		CmdApt:              true,
		CmdAptGet:           true,
		CmdAptKey:           true, // SECURITY: Added for GPG key management
		CmdAptCache:         true, // SECURITY: Added for package information
		CmdAddAptRepository: true, // SECURITY: Added for PPA management
		CmdDpkg:             true,
		CmdCurl:             true,
		CmdWget:             true,
		CmdGit:              true,
		CmdDocker:           true,
		CmdNpm:              true,
		CmdPip:              true,
		CmdPip3:             true,
		CmdGo:               true,
		CmdCargo:            true,
		CmdFlatpak:          true,
		CmdSnap:             true,
		CmdDnf:              true,
		CmdYum:              true,
		CmdPacman:           true,
		CmdZypper:           true,
		CmdBrew:             true,
		CmdMise:             true,
		CmdBash:             true, // SECURITY: Added for script execution (curlpipe installs)
		CmdSh:               true, // SECURITY: Added for basic shell execution
		CmdMkdir:            true,
		CmdCp:               true,
		CmdMv:               true,
		CmdChmod:            true,
		CmdChown:            true,
		"ln":                true,
		"tar":               true,
		"unzip":             true,
		"gunzip":            true,
		"echo":              true,
		"cat":               true,
		"which":             true,
		"whereis":           true,
		"id":                true,
		"whoami":            true, // SECURITY: Added for repository source addition
		"tee":               true, // SECURITY: Added for repository source addition
		"sleep":             true, // SECURITY: Added for timing operations
		"rm":                true, // SECURITY: Added for file removal (validated by dangerous patterns)
		"gpg":               true, // SECURITY: Added for GPG key management
		"gpg2":              true, // SECURITY: Added for GPG key management
		"fastfetch":         true, // SECURITY: Added for system information
		"neofetch":          true, // SECURITY: Added for system information
		"systemctl":         true, // SECURITY: Added for service management
		"usermod":           true, // SECURITY: Added for user management
		"sudo":              true, // SECURITY: Added for privilege escalation (validated separately)
		"nvim":              true, // SECURITY: Added for text editor
		"gtk-launch":        true, // SECURITY: Added for application launching
	}

	// dangerousPatterns are regex patterns for potentially dangerous command constructs
	dangerousPatterns = []*regexp.Regexp{
		regexp.MustCompile(`[;&|]{1,2}\s*rm\s+-rf\s+/`),               // Dangerous rm -rf / after command separators
		regexp.MustCompile(`[;&|]{1,2}\s*sudo\s+rm\s+-rf`),            // Dangerous sudo rm -rf after command separators
		regexp.MustCompile(`[;&|]{1,2}\s*rm\s+-rf\s+/(home|var|usr)`), // Dangerous rm -rf on system directories
		regexp.MustCompile(`\|\s*(bash|sh)\s+-c\s+.*rm`),              // Pipes to shell with rm commands
		regexp.MustCompile(`\|\s*(bash|sh)\s+-c\s+.*>\s*/`),           // Pipes to shell writing to filesystem
		regexp.MustCompile(`[;&|]{1,2}\s*curl\s+.*\|\s*(sh|bash)`),    // Download and execute patterns
		regexp.MustCompile(`[;&|]{1,2}\s*wget\s+.*\|\s*(sh|bash)`),    // Download and execute patterns
		// Removed overly restrictive command substitution blocking
		// regexp.MustCompile(`\$\([^)]*\)`),                          // Command substitution - TOO RESTRICTIVE
		// regexp.MustCompile(`\$\{[^}]*\}`),                          // Variable expansion - TOO RESTRICTIVE
		// Allow safe command substitution patterns but block dangerous ones
		regexp.MustCompile(`\$\((rm|dd|mkfs|fdisk|kill|killall|shutdown|reboot)\s`), // Dangerous commands in substitution
		regexp.MustCompile(`\$\{(rm|dd|mkfs|fdisk|kill|killall|shutdown|reboot)\s`), // Dangerous commands in variable expansion
		regexp.MustCompile(`\.\./.*\.\./.*\.\./`),                                   // Multiple directory traversal attempts
		regexp.MustCompile(`\.\./`),                                                 // Directory traversal patterns
		regexp.MustCompile(`/etc/passwd`),                                           // Sensitive files
		regexp.MustCompile(`/etc/shadow`),                                           // Sensitive files
		regexp.MustCompile(`rm\s+-rf\s+/(\w+|$)`),                                   // Dangerous rm commands on system dirs and root
		regexp.MustCompile(`dd\s+if=/dev.*of=/`),                                    // Dangerous dd commands writing to files
		regexp.MustCompile(`:\(\)\{.*;\s*:\s*\|`),                                   // Fork bombs
		regexp.MustCompile(`>\s*/etc/(passwd|shadow|sudoers)`),                      // Writing to critical system files
		regexp.MustCompile(`>\s*/dev/(sd[a-z]|hd[a-z])\b`),                          // Writing to block devices (not /dev/null)
		regexp.MustCompile(`\s+&\s+[^&]+`),                                          // Background processes with additional commands (but not &&)
		regexp.MustCompile(`\|\s*(sh|bash)\s*<?`),                                   // Pipes specifically to shell interpreters (more specific)
		regexp.MustCompile(`\|\s+[a-zA-Z_][a-zA-Z0-9_]*\s*$`),                       // Pipes to potentially malicious commands (not safe patterns)
		regexp.MustCompile(`\s+\|\|\s+\w+`),                                         // OR operator with additional commands
		regexp.MustCompile("`[^`]*`"),                                               // Backtick command substitution
		regexp.MustCompile(`>\s*/dev/(sd[a-z]|hd[a-z]|tty)`),                        // Writing to specific dangerous device files
		regexp.MustCompile(`;\s*\w+.*&&.*chmod`),                                    // Multi-command with chmod
		regexp.MustCompile(`&&.*python.*-c`),                                        // Python code execution
		regexp.MustCompile(`\b(sh|bash)\s+-c\b`),                                    // Direct shell code execution
		regexp.MustCompile(`[;&|]+\s*$`),                                            // Commands ending with operators
		regexp.MustCompile(`>\s*/etc/`),                                             // Writing to /etc directory
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
// SECURITY: This function allows shell execution for pipes but validates patterns first
func parseCommand(command string) (string, []string, bool) {
	// Trim whitespace
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, false
	}

	// SECURITY: Check against dangerous patterns using our specific regex list
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			// Allow specific safe patterns that might otherwise be flagged
			if strings.Contains(command, ">/dev/null") || strings.Contains(command, "2>/dev/null") {
				continue // Allow redirections to /dev/null
			}
			// SECURITY: Reject commands matching dangerous patterns
			return "", nil, false
		}
	}

	// Check if command contains shell operators that require shell execution
	shellOperators := []string{"|", "&&", "||", ";", ">", "<", ">>", "2>", "&"}
	needsShell := false
	for _, operator := range shellOperators {
		if strings.Contains(command, operator) {
			needsShell = true
			break
		}
	}

	if needsShell {
		// Return shell execution for complex commands
		return "bash", []string{"-c", command}, true
	}

	// Split command into parts for direct execution
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", nil, false
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

// ValidateCommand implements CommandExecutor.ValidateCommand using the new security package
func (ce *DefaultCommandExecutor) ValidateCommand(command string) error {
	// Use the new security validation package with moderate security level
	validator := security.NewCommandValidator(security.SecurityLevelModerate)
	return validator.ValidateCommand(command)
}

// validateCommandWhitelist function removed - now using security package

// SecureCommandExecutor implements CommandExecutor using pattern-based validation
type SecureCommandExecutor struct {
	validator *security.CommandValidator
}

// NewSecureCommandExecutor creates a new secure command executor with configuration-aware validation
func NewSecureCommandExecutor(level security.SecurityLevel, apps []types.CrossPlatformApp) *SecureCommandExecutor {
	return &SecureCommandExecutor{
		validator: security.NewCommandValidator(level),
	}
}

// ExecuteCommand implements CommandExecutor.ExecuteCommand for SecureCommandExecutor
func (sce *SecureCommandExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	// Validate command using pattern-based approach
	if err := sce.validator.ValidateCommand(command); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Parse and execute using safest method
	executable, args, needsShell := parseCommand(command)

	var cmd *exec.Cmd
	if needsShell {
		// Use shell for complex commands (pipes, redirections, etc.)
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	} else {
		// Direct execution for simple commands
		cmd = exec.CommandContext(ctx, executable, args...)
	}

	// Set platform-specific security attributes
	cmd.SysProcAttr = sce.getPlatformSysProcAttr()

	return cmd, nil
}

// ValidateCommand implements CommandExecutor.ValidateCommand for SecureCommandExecutor
func (sce *SecureCommandExecutor) ValidateCommand(command string) error {
	return sce.validator.ValidateCommand(command)
}

// ValidateConfigCommand validates a command in the context of a specific application
func (sce *SecureCommandExecutor) ValidateConfigCommand(command string, appName string) error {
	return sce.validator.ValidateCommand(command)
}

// getPlatformSysProcAttr returns platform-specific security attributes for SecureCommandExecutor
func (sce *SecureCommandExecutor) getPlatformSysProcAttr() *syscall.SysProcAttr {
	return getPlatformSysProcAttr()
}

// NewStreamingInstaller creates a new streaming installer with context cancellation
func NewStreamingInstaller(program *tea.Program, repo types.Repository, ctx context.Context, settings config.CrossPlatformSettings) *StreamingInstaller {
	instCtx, cancel := context.WithCancel(ctx)

	// Initialize performance analyzer
	analyzer, err := performance.NewPerformanceAnalyzer(settings)
	if err != nil {
		log.Warn("Failed to initialize performance analyzer", "error", err)
		// Continue without performance analysis rather than failing
		analyzer = nil
	}

	return &StreamingInstaller{
		program:             program,
		repo:                repo,
		executor:            NewDefaultCommandExecutor(), // Use default command executor
		ctx:                 instCtx,
		cancel:              cancel,
		config:              DefaultInstallerConfig(),
		performanceAnalyzer: analyzer,
	}
}

// NewStreamingInstallerWithSecureExecutor creates a streaming installer with secure pattern-based validation
func NewStreamingInstallerWithSecureExecutor(program *tea.Program, repo types.Repository, ctx context.Context, level security.SecurityLevel, apps []types.CrossPlatformApp, settings config.CrossPlatformSettings) *StreamingInstaller {
	instCtx, cancel := context.WithCancel(ctx)

	// Initialize performance analyzer
	analyzer, err := performance.NewPerformanceAnalyzer(settings)
	if err != nil {
		log.Warn("Failed to initialize performance analyzer", "error", err)
		analyzer = nil
	}

	return &StreamingInstaller{
		program:             program,
		repo:                repo,
		executor:            NewSecureCommandExecutor(level, apps),
		ctx:                 instCtx,
		cancel:              cancel,
		config:              DefaultInstallerConfig(),
		performanceAnalyzer: analyzer,
	}
}

// NewStreamingInstallerWithExecutor creates a streaming installer with a custom command executor for testing
func NewStreamingInstallerWithExecutor(program *tea.Program, repo types.Repository, ctx context.Context, executor CommandExecutor, settings config.CrossPlatformSettings) *StreamingInstaller {
	instCtx, cancel := context.WithCancel(ctx)

	// Initialize performance analyzer
	analyzer, err := performance.NewPerformanceAnalyzer(settings)
	if err != nil {
		log.Warn("Failed to initialize performance analyzer", "error", err)
		analyzer = nil
	}

	return &StreamingInstaller{
		program:             program,
		repo:                repo,
		executor:            executor,
		ctx:                 instCtx,
		cancel:              cancel,
		config:              DefaultInstallerConfig(),
		performanceAnalyzer: analyzer,
	}
}

// SetProgressManager sets the progress manager for enhanced progress tracking
func (si *StreamingInstaller) SetProgressManager(manager *progresspkg.ProgressManager) {
	si.progressManager = manager
}

// GetProgressManager returns the current progress manager
func (si *StreamingInstaller) GetProgressManager() *progresspkg.ProgressManager {
	return si.progressManager
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
	for i, app := range apps {
		// Check for context cancellation before each app
		select {
		case <-ctx.Done():
			si.sendLog("INFO", "Installation cancelled before starting next app")
			return ctx.Err()
		default:
		}

		// Send app started message to update TUI display
		if si.program != nil {
			si.program.Send(AppStartedMsg{
				AppName:  app.Name,
				AppIndex: i,
			})
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
//   - Performance analysis and warnings (if enabled)
//   - Platform-specific configuration resolution
//   - App validation using the app's Validate() method
//   - Pre-install command execution (if configured)
//   - Main installation command execution via the configured install method
//   - Post-install command execution (if configured)
//   - Database registration of the successfully installed app
//   - Post-installation performance tracking
//
// Parameters:
//   - app: CrossPlatformApp configuration containing installation instructions
//   - settings: Installation settings including verbosity flags
//
// Returns:
//   - error: nil on success, or detailed error indicating which phase failed
func (si *StreamingInstaller) InstallApp(ctx context.Context, app types.CrossPlatformApp, settings config.CrossPlatformSettings) error {
	si.sendLog("INFO", fmt.Sprintf("Starting installation of %s", app.Name))

	startTime := time.Now()

	// Get platform-specific configuration
	osConfig := app.GetOSConfig()
	if osConfig.InstallMethod == "" {
		return fmt.Errorf("no configuration available for current platform")
	}

	// Validate app
	if err := app.Validate(); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Perform performance analysis and show warnings
	if si.performanceAnalyzer != nil {
		si.sendLog("INFO", fmt.Sprintf("Analyzing performance characteristics for %s...", app.Name))
		warnings := si.performanceAnalyzer.AnalyzePreInstall(app.Name, app)

		// Display warnings to user
		for _, warning := range warnings {
			formattedWarning := performance.FormatWarning(warning)

			// Send warning with appropriate level
			switch warning.Level {
			case performance.WarningLevelCritical:
				si.sendLog("CRITICAL", formattedWarning)
			case performance.WarningLevelWarning:
				si.sendLog("WARN", formattedWarning)
			case performance.WarningLevelCaution:
				si.sendLog("CAUTION", formattedWarning)
			case performance.WarningLevelInfo:
				si.sendLog("INFO", formattedWarning)
			}

			// Note: Removed synchronous sleep to prevent TUI thread blocking
			// Critical warnings are highlighted through styling instead
		}

		if len(warnings) > 0 {
			si.sendLog("INFO", fmt.Sprintf("Performance analysis complete. Found %d warning(s) for %s", len(warnings), app.Name))
		} else {
			si.sendLog("INFO", fmt.Sprintf("Performance analysis complete. No issues detected for %s", app.Name))
		}
	}

	// Handle theme selection if app has themes
	if len(osConfig.Themes) > 0 {
		si.sendLog("INFO", fmt.Sprintf("App %s has %d themes available, prompting user for selection", app.Name, len(osConfig.Themes)))
		if err := si.handleThemeSelection(ctx, app.Name, osConfig.Themes); err != nil {
			return fmt.Errorf("theme selection failed: %w", err)
		}
	}

	// Handle pre-install commands
	if len(osConfig.PreInstall) > 0 {
		si.sendLog("INFO", "Executing pre-install commands...")
		if err := si.executeCommands(ctx, osConfig.PreInstall); err != nil {
			si.recordFailedInstallation(app.Name, startTime, err)
			return fmt.Errorf("pre-install failed: %w", err)
		}
	}

	// Check and install platform dependencies before main installation
	if err := si.checkAndInstallDependencies(ctx, osConfig); err != nil {
		si.recordFailedInstallation(app.Name, startTime, err)
		return fmt.Errorf("dependency checking failed: %w", err)
	}

	// Execute main installation command
	si.sendLog("INFO", fmt.Sprintf("Installing %s using %s...", app.Name, osConfig.InstallMethod))
	if err := si.executeInstallCommand(ctx, app, &osConfig); err != nil {
		si.recordFailedInstallation(app.Name, startTime, err)
		return fmt.Errorf("installation failed: %w", err)
	}

	// Handle post-install commands
	if len(osConfig.PostInstall) > 0 {
		si.sendLog("INFO", "Executing post-install commands...")
		if err := si.executeCommands(ctx, osConfig.PostInstall); err != nil {
			si.recordFailedInstallation(app.Name, startTime, err)
			return fmt.Errorf("post-install failed: %w", err)
		}
	}

	// Apply selected theme if user made a choice and themes are available
	if len(osConfig.Themes) > 0 {
		if err := si.applySelectedTheme(ctx, app.Name, osConfig.Themes); err != nil {
			si.sendLog("WARN", fmt.Sprintf("Failed to apply theme for %s: %v", app.Name, err))
			// Don't fail installation if theme application fails
		}
	}

	// Save to repository with configurable error handling
	si.repoMutex.Lock()
	err := si.repo.AddApp(app.Name)
	si.repoMutex.Unlock()
	if err != nil {
		// Enhanced error context with specific details
		osConfig := app.GetOSConfig()
		errorContext := fmt.Sprintf("Failed to save app '%s' to database (method: %s, installer: %s): %v",
			app.Name, osConfig.InstallMethod, si.getInstallerType(), err)
		si.sendLog("ERROR", errorContext)

		if si.config.FailOnDatabaseErrors {
			// Fail the installation if configured to do so with enhanced context
			return fmt.Errorf("database operation failed for app '%s' using %s installer: %w",
				app.Name, si.getInstallerType(), err)
		} else {
			// Log prominently but don't fail the installation with specific guidance
			si.sendLog("WARN", fmt.Sprintf("Installation of %s succeeded but app tracking may be inconsistent", app.Name))
			si.sendLog("WARN", "Consider fixing database connectivity for proper app tracking")
			si.sendLog("WARN", "This may affect 'devex uninstall' and installation status queries")
		}
	} else {
		si.sendLog("INFO", fmt.Sprintf("App %s (via %s) registered in database successfully",
			app.Name, si.getInstallerType()))
	}

	// Record post-installation performance metrics
	if si.performanceAnalyzer != nil {
		// Estimate download size (we don't have exact size without more complex tracking)
		estimatedSize := int64(50 * 1024 * 1024) // Default 50MB, could be improved with actual tracking

		// Record metrics with success status
		if err := si.performanceAnalyzer.AnalyzePostInstall(app.Name, startTime, true, estimatedSize); err != nil {
			si.sendLog("WARN", fmt.Sprintf("Failed to record performance metrics for %s: %v", app.Name, err))
			// Don't fail installation if metrics recording fails
		} else {
			installTime := time.Since(startTime)
			si.sendLog("INFO", fmt.Sprintf("Performance metrics recorded for %s (install time: %v)", app.Name, installTime))
		}
	}

	si.sendLog("INFO", fmt.Sprintf("Successfully installed %s", app.Name))
	return nil
}

// recordFailedInstallation records performance metrics for failed installations
func (si *StreamingInstaller) recordFailedInstallation(appName string, startTime time.Time, err error) {
	if si.performanceAnalyzer != nil {
		// Record failed installation metrics
		if recordErr := si.performanceAnalyzer.AnalyzePostInstall(appName, startTime, false, 0); recordErr != nil {
			si.sendLog("WARN", fmt.Sprintf("Failed to record failure metrics for %s: %v", appName, recordErr))
		}
	}
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
	case "mise":
		return si.executeMiseInstall(ctx, app, osConfig)
	case "dnf", "yum", "pacman", "zypper", "brew", "apk", "emerge", "eopkg", "flatpak", "snap", "xbps", "yay":
		return si.executePackageManagerInstall(ctx, app, osConfig)
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

			// Use modern GPG keyring approach (no apt-key)
			si.sendLog("INFO", "Using modern GPG keyring approach")
			if err := si.addModernGPGKey(ctx, source.KeySource, source.KeyName, source.RequireDearmor); err != nil {
				return fmt.Errorf("failed to add GPG key for %s: %w", source.SourceName, err)
			}
		}

		// Add the repository source
		if source.SourceRepo != "" {
			addSourceCmd := fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null",
				source.SourceRepo, source.SourceName)
			if err := si.executeCommandStream(ctx, addSourceCmd); err != nil {
				return fmt.Errorf("failed to add APT source %s: %w", source.SourceName, err)
			}
		}
	}

	// Update package lists
	si.sendLog("INFO", "Updating package lists...")
	if err := si.executeCommandStream(ctx, "sudo apt-get update"); err != nil {
		return err
	}

	// Install package - construct proper APT command
	// Use 'apt-get' for scripted operations to avoid CLI warnings
	aptCommand := fmt.Sprintf("sudo apt-get install -y %s", osConfig.InstallCommand)
	return si.executeCommandStream(ctx, aptCommand)
}

// addModernGPGKey downloads and processes a GPG key using modern keyring approach
func (si *StreamingInstaller) addModernGPGKey(ctx context.Context, keyURL, keyName string, requireDearmor bool) error {
	si.sendLog("INFO", "Downloading and processing GPG key...")

	// Validate the key URL is HTTPS
	if !strings.HasPrefix(keyURL, "https://") {
		return fmt.Errorf("GPG key URL must use HTTPS: %s", keyURL)
	}

	// Check if the GPG key file already exists
	checkExistsCmd := fmt.Sprintf("test -f %s", keyName)
	if err := si.executeCommandStream(ctx, checkExistsCmd); err == nil {
		si.sendLog("INFO", "GPG key file already exists")
		return nil
	}

	// Ensure the keyrings directory exists
	createDirCmd := "sudo mkdir -p /etc/apt/keyrings"
	if err := si.executeCommandStream(ctx, createDirCmd); err != nil {
		return fmt.Errorf("failed to create keyrings directory: %w", err)
	}

	if requireDearmor {
		// Download and dearmor the key in one command
		downloadAndDearmorCmd := fmt.Sprintf("curl -fsSL %s | sudo gpg --dearmor -o %s", keyURL, keyName)
		si.sendLog("INFO", fmt.Sprintf("Downloading and dearmorying GPG key: %s", downloadAndDearmorCmd))
		return si.executeCommandStream(ctx, downloadAndDearmorCmd)
	} else {
		// Download the key directly
		downloadCmd := fmt.Sprintf("curl -fsSL %s | sudo tee %s > /dev/null", keyURL, keyName)
		si.sendLog("INFO", fmt.Sprintf("Downloading GPG key: %s", downloadCmd))
		return si.executeCommandStream(ctx, downloadCmd)
	}
}

// executePackageManagerInstall handles package manager installations with intelligent updates
func (si *StreamingInstaller) executePackageManagerInstall(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	packageManager := osConfig.InstallMethod

	// Package manager cache updates are now handled by respective package manager plugins
	si.sendLog("INFO", fmt.Sprintf("Package manager %s cache updates handled by plugins", packageManager))

	// Construct install command based on package manager
	var installCmd string
	switch packageManager {
	case "dnf":
		installCmd = fmt.Sprintf("sudo dnf install -y %s", osConfig.InstallCommand)
	case "yum":
		installCmd = fmt.Sprintf("sudo yum install -y %s", osConfig.InstallCommand)
	case "pacman":
		installCmd = fmt.Sprintf("sudo pacman -S --noconfirm %s", osConfig.InstallCommand)
	case "zypper":
		installCmd = fmt.Sprintf("sudo zypper install -y %s", osConfig.InstallCommand)
	case "brew":
		installCmd = fmt.Sprintf("brew install %s", osConfig.InstallCommand)
	case "apk":
		installCmd = fmt.Sprintf("sudo apk add %s", osConfig.InstallCommand)
	case "emerge":
		installCmd = fmt.Sprintf("sudo emerge %s", osConfig.InstallCommand)
	case "eopkg":
		installCmd = fmt.Sprintf("sudo eopkg install -y %s", osConfig.InstallCommand)
	case "flatpak":
		installCmd = fmt.Sprintf("flatpak install -y %s", osConfig.InstallCommand)
	case "snap":
		installCmd = fmt.Sprintf("sudo snap install %s", osConfig.InstallCommand)
	case "xbps":
		installCmd = fmt.Sprintf("sudo xbps-install -y %s", osConfig.InstallCommand)
	case "yay":
		installCmd = fmt.Sprintf("yay -S --noconfirm %s", osConfig.InstallCommand)
	default:
		// Fallback to the configured install command
		installCmd = osConfig.InstallCommand
	}

	si.sendLog("INFO", fmt.Sprintf("Installing %s using %s...", app.Name, packageManager))
	return si.executeCommandStream(ctx, installCmd)
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
	// #nosec G302 -- Script files need execute permissions
	if err := os.Chmod(tmpFile.Name(), 0700); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil {
			log.Warn("Failed to close temp file during cleanup", "error", closeErr)
		}
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			log.Warn("Failed to remove temp file during cleanup", "error", removeErr)
		}
		return "", fmt.Errorf("failed to set execute permissions: %w", err)
	}

	fileName := tmpFile.Name()
	if closeErr := tmpFile.Close(); closeErr != nil {
		log.Warn("Failed to close temp file", "error", closeErr)
	}
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
	// #nosec G302 -- Script files need execute permissions
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

// executeMiseInstall handles mise tool installations with proper command construction
func (si *StreamingInstaller) executeMiseInstall(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	si.sendLog("INFO", fmt.Sprintf("Installing %s using mise...", app.Name))

	// Construct proper mise command with PATH setup and secure execution
	// This mirrors the logic from the mise installer but adapts it for TUI streaming
	miseCommand := fmt.Sprintf(`export PATH="$HOME/.local/bin:$PATH" && if command -v mise >/dev/null 2>&1; then mise use --global %s; else echo "mise not found in PATH"; exit 1; fi`, osConfig.InstallCommand)

	// Execute the mise command using bash
	bashCommand := fmt.Sprintf("bash -c '%s'", strings.ReplaceAll(miseCommand, "'", "'\"'\"'"))
	return si.executeCommandStream(ctx, bashCommand)
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
			// Replace placeholders in shell commands
			processedCommand := utils.ReplacePlaceholders(cmd.Shell, map[string]string{})
			si.sendLog("INFO", fmt.Sprintf("Executing shell: %s", processedCommand))
			if err := si.executeCommandStream(ctx, processedCommand); err != nil {
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
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Create pipe for stdin (for password prompts)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

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

	// Note: cmd.Wait() automatically closes stdout, stderr, and stdin pipes
	// so we don't need to close them manually to avoid "file already closed" errors

	// Wait for all goroutines to finish processing the remaining data
	wg.Wait()

	return cmdErr
}

// streamOutput streams command output to the TUI with proper error handling
// It handles carriage returns and progress indicators from package managers like apt
func (si *StreamingInstaller) streamOutput(reader io.Reader, source string) {
	// Use a custom scanner that handles both \n and \r as delimiters
	scanner := bufio.NewScanner(reader)

	// Custom split function that handles both newlines and carriage returns
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// Look for \n or \r
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' {
				// Found newline, return the line including the newline
				return i + 1, data[:i], nil
			}
			if data[i] == '\r' {
				// Found carriage return
				if i+1 < len(data) && data[i+1] == '\n' {
					// \r\n sequence (Windows style)
					return i + 2, data[:i], nil
				}
				// Just \r (progress update style)
				return i + 1, data[:i], nil
			}
		}

		// If we're at EOF, return what we have
		if atEOF {
			return len(data), data, nil
		}

		// Request more data
		return 0, nil, nil
	})

	var currentLine string // Track the current line being updated

	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-si.ctx.Done():
			si.sendLog("INFO", fmt.Sprintf("%s stream cancelled", source))
			return
		default:
		}

		line := scanner.Text()

		// Clean up ANSI escape sequences and control characters
		line = cleanTerminalOutput(line)

		// Skip empty lines and apt database reading progress
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Filter out apt progress messages that cause display issues
		if strings.Contains(line, "Reading database") ||
			strings.Contains(line, "Scanning processes") ||
			strings.Contains(line, "Scanning candidates") ||
			strings.Contains(line, "Scanning linux images") ||
			strings.Contains(line, "Readin database") { // Handle partial lines from carriage returns
			// These are progress indicators that update in place
			// We'll show them once when complete
			if strings.Contains(line, "done") ||
				strings.Contains(line, "100%") ||
				strings.Contains(line, "... done") {
				currentLine = line
			} else {
				// Store but don't display intermediate progress
				currentLine = line
				continue
			}
		}

		// Send the cleaned line
		if currentLine != "" {
			si.sendLog(source, currentLine)
			currentLine = ""
		} else {
			si.sendLog(source, line)
		}
	}

	// Send any remaining line
	if currentLine != "" {
		si.sendLog(source, currentLine)
	}

	// Check for scanner errors (but ignore closed pipe errors)
	if err := scanner.Err(); err != nil && !strings.Contains(err.Error(), "file already closed") {
		si.sendLog("ERROR", fmt.Sprintf("Scanner error in %s: %v", source, err))
	}
}

// cleanTerminalOutput removes ANSI escape sequences and control characters
func cleanTerminalOutput(s string) string {
	// Remove ANSI escape sequences (including all variants)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	s = ansiRegex.ReplaceAllString(s, "")

	// Remove cursor positioning sequences
	cursorRegex := regexp.MustCompile(`\x1b\[[0-9]*[ABCD]`)
	s = cursorRegex.ReplaceAllString(s, "")

	// Remove additional control sequences
	ctrlRegex := regexp.MustCompile(`\x1b[()][0-9A-Z]`)
	s = ctrlRegex.ReplaceAllString(s, "")

	// Remove carriage returns that aren't followed by newlines
	s = strings.ReplaceAll(s, "\r", "")

	// Remove other control characters except tabs and newlines
	var result strings.Builder
	for _, r := range s {
		if r == '\t' || r == '\n' || (r >= 32 && r < 127) || r > 127 {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
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

// sendLog sends a log message to both the TUI and persistent log file
func (si *StreamingInstaller) sendLog(level, message string) {
	// ALWAYS write to persistent log file first (for debugging support)
	switch strings.ToUpper(level) {
	case "ERROR", "STDERR":
		log.Error(message, nil, "source", "tui_installer")
	case "WARN", "WARNING":
		log.Warn(message, "source", "tui_installer")
	case "DEBUG":
		log.Debug(message, "source", "tui_installer")
	case "INFO", "STDOUT":
		log.Info(message, "source", "tui_installer")
	default:
		log.Info(fmt.Sprintf("[%s] %s", level, message), "source", "tui_installer")
	}

	// Skip TUI sending when program is nil (during testing)
	if si.program == nil {
		return
	}

	// Add panic protection for program.Send calls
	defer func() {
		if r := recover(); r != nil {
			// TUI program may have exited, log to stderr and file
			errorMsg := fmt.Sprintf("TUI unavailable, message: %s", message)
			log.Error(errorMsg, nil, "source", "tui_panic")
			fmt.Fprintf(os.Stderr, "[%s] %s: %s (TUI unavailable)\n",
				time.Now().Format("15:04:05"), level, message)
		}
	}()

	// Check if context is cancelled before sending to TUI
	select {
	case <-si.ctx.Done():
		// Context cancelled, don't send to TUI but log was already written
		log.Info("Context cancelled while sending to TUI", "level", level, "message", message)
		return
	default:
		si.program.Send(LogMsg{
			Message:   message,
			Timestamp: time.Now(),
			Level:     level,
		})
	}
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
func StartInstallation(ctx context.Context, apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Add recovery mechanism to prevent panics from hanging the application
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("\n❌ Streaming installer panic: %v\n", r)
			// Print stack trace for debugging
			fmt.Printf("Stack trace: %s\n", string(debug.Stack()))
		}
	}()

	// Create cancellable context from the passed context for installation control
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create TUI model
	m := NewModel(apps)

	// Create program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Ensure proper cleanup even if Run() panics
	defer func() {
		if p != nil {
			p.Kill()
		}
	}()

	// Create streaming installer with context
	installer := NewStreamingInstaller(p, repo, ctx, settings)
	defer installer.cancel() // Ensure cleanup

	// Start installation in background with context cancellation
	go func() {
		defer func() {
			if r := recover(); r != nil {
				installer.sendLog("ERROR", fmt.Sprintf("Installation goroutine panic: %v", r))
			}
		}()

		select {
		case <-ctx.Done():
			// Installation was cancelled
			installer.sendLog("INFO", "Installation cancelled by user")
			return
		case <-time.After(installer.config.InitializationDelay):
			// Let TUI initialize before starting installation
			if err := installer.InstallApps(ctx, apps, settings); err != nil {
				installer.sendLog("ERROR", fmt.Sprintf("Installation failed: %v", err))
			} else {
				installer.sendLog("INFO", "Installation completed successfully")
			}

			// Send quit message to exit TUI - use a timer instead of blocking sleep
			if p != nil {
				go func() {
					time.Sleep(2 * time.Second)
					p.Send(tea.Quit())
				}()
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

// handleThemeSelection uses the global theme preference instead of prompting for individual apps
func (si *StreamingInstaller) handleThemeSelection(ctx context.Context, appName string, themes []types.Theme) error {
	si.sendLog("INFO", fmt.Sprintf("Using global theme preference for %s", appName))

	// Get the global theme preference that was set during setup
	if si.repo == nil {
		si.sendLog("INFO", fmt.Sprintf("No repository available, skipping theme selection for %s", appName))
		return nil
	}

	si.repoMutex.RLock()
	globalTheme, err := si.repo.Get("global_theme")
	si.repoMutex.RUnlock()

	if err != nil || globalTheme == "" {
		si.sendLog("INFO", fmt.Sprintf("No global theme preference found, skipping theme selection for %s", appName))
		return nil
	}

	// Find the global theme in the app's available themes
	var selectedTheme *types.Theme
	for _, theme := range themes {
		if theme.Name == globalTheme {
			selectedTheme = &theme
			break
		}
	}

	if selectedTheme == nil {
		si.sendLog("WARN", fmt.Sprintf("Global theme '%s' not found in available themes for %s, skipping", globalTheme, appName))
		return nil
	}

	si.sendLog("INFO", fmt.Sprintf("Using global theme '%s' for %s", selectedTheme.Name, appName))

	// Store app-specific theme preference using the global theme
	themeKey := fmt.Sprintf("app_theme_%s", appName)
	si.repoMutex.Lock()
	err = si.repo.Set(themeKey, selectedTheme.Name)
	si.repoMutex.Unlock()
	if err != nil {
		si.sendLog("WARN", fmt.Sprintf("Failed to store theme preference for %s: %v", appName, err))
		// Don't fail installation if theme preference storage fails
	} else {
		si.sendLog("INFO", fmt.Sprintf("Theme preference stored: %s -> %s", appName, selectedTheme.Name))
	}

	return nil
}

// applySelectedTheme applies the selected theme files for a specific app
func (si *StreamingInstaller) applySelectedTheme(ctx context.Context, appName string, themes []types.Theme) error {
	// Get the selected theme preference from repository
	if si.repo == nil {
		si.sendLog("INFO", fmt.Sprintf("No repository available, skipping theme application for %s", appName))
		return nil
	}

	themeKey := fmt.Sprintf("app_theme_%s", appName)
	si.repoMutex.RLock()
	selectedThemeName, err := si.repo.Get(themeKey)
	si.repoMutex.RUnlock()
	if err != nil {
		si.sendLog("INFO", fmt.Sprintf("No theme preference found for %s, skipping theme application", appName))
		return nil
	}

	// Find the selected theme in the available themes
	var selectedTheme *types.Theme
	for _, theme := range themes {
		if theme.Name == selectedThemeName {
			selectedTheme = &theme
			break
		}
	}

	if selectedTheme == nil {
		si.sendLog("WARN", fmt.Sprintf("Selected theme '%s' not found for %s", selectedThemeName, appName))
		return nil
	}

	si.sendLog("INFO", fmt.Sprintf("Applying theme '%s' for %s", selectedTheme.Name, appName))

	// Apply theme files by copying them to their destinations
	for _, configFile := range selectedTheme.Files {
		si.sendLog("INFO", fmt.Sprintf("Copying theme file from %s to %s", configFile.Source, configFile.Destination))

		// Expand tilde in paths
		source := expandPath(configFile.Source)
		destination := expandPath(configFile.Destination)

		// Create destination directory if it doesn't exist
		if err := si.createDirectoryForFile(ctx, destination); err != nil {
			si.sendLog("WARN", fmt.Sprintf("Failed to create directory for %s: %v", destination, err))
			continue
		}

		// Copy the theme file
		copyCmd := fmt.Sprintf("cp '%s' '%s'", source, destination)
		if err := si.executeCommandStream(ctx, copyCmd); err != nil {
			si.sendLog("WARN", fmt.Sprintf("Failed to copy theme file from %s to %s: %v", source, destination, err))
		} else {
			si.sendLog("INFO", fmt.Sprintf("Successfully copied theme file to %s", destination))
		}
	}

	si.sendLog("INFO", fmt.Sprintf("Theme '%s' applied successfully for %s", selectedTheme.Name, appName))
	return nil
}

// createDirectoryForFile creates the parent directory for a file path
func (si *StreamingInstaller) createDirectoryForFile(ctx context.Context, filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "/" {
		return nil
	}

	cmd := fmt.Sprintf("mkdir -p '%s'", dir)
	return si.executeCommandStream(ctx, cmd)
}

// getInstallerType returns a human-readable description of the current installer context
func (si *StreamingInstaller) getInstallerType() string {
	return "StreamingInstaller"
}

// expandPath expands tilde (~) in file paths to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir := os.Getenv("HOME")
		return strings.Replace(path, "~", homeDir, 1)
	}
	return path
}

// checkAndInstallDependencies checks platform-specific dependencies and installs missing ones
func (si *StreamingInstaller) checkAndInstallDependencies(ctx context.Context, osConfig types.OSConfig) error {
	// Only check dependencies if there are platform requirements
	if len(osConfig.PlatformRequirements) == 0 {
		si.sendLog("INFO", "No platform requirements specified, skipping dependency check")
		return nil
	}

	si.sendLog("INFO", "Checking platform-specific dependencies...")

	// Detect current platform
	platformDetector := platform.NewPlatformDetector()
	currentPlatform := platformDetector.Detect()

	si.sendLog("INFO", fmt.Sprintf("Detected platform: OS=%s, Distribution=%s", currentPlatform.OS, currentPlatform.Distribution))

	// Create appropriate package manager based on platform
	// Package manager detection and system updates are now handled by plugins
	switch currentPlatform.OS {
	case "linux":
		switch currentPlatform.Distribution {
		case "debian", "ubuntu":
			si.sendLog("INFO", "System updates would be handled by package-manager-apt plugin")
		default:
			si.sendLog("WARN", fmt.Sprintf("No package manager support for distribution: %s", currentPlatform.Distribution))
			return nil
		}
	default:
		si.sendLog("WARN", fmt.Sprintf("No package manager support for OS: %s", currentPlatform.OS))
		return nil
	}

	// System updates are now handled by plugins
	si.sendLog("INFO", "System updates would be handled by package manager plugins")

	// Plugin system handles dependency checking
	// This functionality has been migrated to the plugin architecture
	si.sendLog("INFO", "System update is handled by package manager plugins")
	return nil
}
