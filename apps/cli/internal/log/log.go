package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/charmbracelet/log"
)

// Logger wraps the charmbracelet logger with additional context support.
type Logger struct {
	logger     *log.Logger
	context    map[string]any
	logFile    *os.File
	debugMode  bool
	testMode   bool // For silent testing
	silentMode bool // For completely suppressing all output
}

var logger *Logger
var cliVersion = "unknown" // CLI version set during initialization

// Log levels
var (
	DebugLevel = log.DebugLevel
	ErrorLevel = log.ErrorLevel
	FatalLevel = log.FatalLevel
	InfoLevel  = log.InfoLevel
	WarnLevel  = log.WarnLevel
)

// New initializes a new logger with the provided writer.
func New(w io.Writer) *Logger {
	l := log.New(w)           // Create a logger with the provided writer
	l.SetLevel(log.InfoLevel) // Set the default log level
	return &Logger{
		logger:     l,
		context:    make(map[string]any),
		debugMode:  false,
		testMode:   false,
		silentMode: false,
	}
}

// InitDefaultLogger initializes the default logger with a specified writer.
func InitDefaultLogger(w io.Writer) {
	logger = New(w)
}

// CLIVersion returns the build version initialized by the CLI entry point.
func CLIVersion() string { return cliVersion }

// SetCLIVersion sets the CLI version for inclusion in system information
func SetCLIVersion(version string) {
	if version != "" {
		cliVersion = version
		// Update the log file with correct version info if logger is initialized
		updateLogFileHeader()
	}
}

// updateLogFileHeader writes an updated system info header to the log file
func updateLogFileHeader() {
	if logger != nil && logger.logFile != nil {
		// Write updated CLI version info to log file
		header := fmt.Sprintf("\n%s\nUpdated CLI Version: %s\n%s\n",
			strings.Repeat("-", 50),
			cliVersion,
			strings.Repeat("-", 50))
		_, _ = logger.logFile.WriteString(header)
	}
}

// InitTestLogger initializes a silent logger for tests.
func InitTestLogger() {
	logger = &Logger{
		logger:     log.New(io.Discard), // Discard all output
		context:    make(map[string]any),
		debugMode:  false,
		testMode:   true,
		silentMode: true,
	}
}

// InitSilentLogger initializes a completely silent logger.
func InitSilentLogger() {
	logger = &Logger{
		logger:     log.New(io.Discard),
		context:    make(map[string]any),
		debugMode:  false,
		testMode:   false,
		silentMode: true,
	}
}

// IsTestMode returns whether test mode is enabled.
func IsTestMode() bool {
	if logger != nil {
		return logger.testMode
	}
	return false
}

// IsSilentMode returns whether silent mode is enabled.
func IsSilentMode() bool {
	if logger != nil {
		return logger.silentMode
	}
	return false
}

// InitFileLogger initializes a file-based logger with optional debug mode and enhanced system information.
func InitFileLogger(debugMode bool) error {
	// Import platform package and gather system info - doing this here to avoid circular imports
	systemInfo := gatherBasicSystemInfo()

	// Create logs directory
	dirs, err := devexpaths.Resolve()
	if err != nil {
		return err
	}
	logsDir := dirs.Logs()
	if err := os.MkdirAll(logsDir, 0750); err != nil { // #nosec G703 -- fixed path under the user home directory
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	var writer io.Writer

	if debugMode {
		// Debug mode: log to both file and stderr
		timestamp := time.Now().Format("20060102-150405")
		logFile := filepath.Join(logsDir, fmt.Sprintf("devex-%s.log", timestamp))

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G703 -- timestamped name in the fixed logs dir
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}

		// Write enhanced header to file
		header := fmt.Sprintf("%s\nMode: %s\n%s\n",
			systemInfo,
			getMode(debugMode),
			strings.Repeat("-", 50))
		_, _ = file.WriteString(header)

		// Write to both file and stderr in debug mode
		writer = io.MultiWriter(file, os.Stderr)

		logger = &Logger{
			logger:     log.New(writer),
			context:    make(map[string]any),
			logFile:    file,
			debugMode:  true,
			testMode:   false,
			silentMode: false,
		}
	} else {
		// Normal mode: log only to file
		timestamp := time.Now().Format("20060102-150405")
		logFile := filepath.Join(logsDir, fmt.Sprintf("devex-%s.log", timestamp))

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600) // #nosec G703 -- timestamped name in the fixed logs dir
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}

		// Write enhanced header to file
		header := fmt.Sprintf("%s\nMode: %s\n%s\n",
			systemInfo,
			getMode(debugMode),
			strings.Repeat("-", 50))
		_, _ = file.WriteString(header)

		writer = file

		logger = &Logger{
			logger:     log.New(writer),
			context:    make(map[string]any),
			logFile:    file,
			debugMode:  false,
			testMode:   false,
			silentMode: false,
		}
	}

	logger.logger.SetLevel(log.InfoLevel)
	return nil
}

// GetLogFile returns the current log file path if available.
func GetLogFile() string {
	if logger != nil && logger.logFile != nil {
		return logger.logFile.Name()
	}
	return ""
}

// IsDebugMode returns whether debug mode is enabled.
func IsDebugMode() bool {
	if logger != nil {
		return logger.debugMode
	}
	return false
}

// Close closes the log file if it's open.
func Close() error {
	if logger != nil && logger.logFile != nil {
		return logger.logFile.Close()
	}
	return nil
}

// getMode returns a string representation of the logging mode.
func getMode(debugMode bool) string {
	if debugMode {
		return "debug (file + console)"
	}
	return "normal (file only)"
}

// SetLevel dynamically updates the logging level.
func SetLevel(level log.Level) {
	if logger != nil && logger.logger != nil {
		logger.logger.SetLevel(level)
	}
}

// WithContext adds contextual metadata to the logger.
func WithContext(ctx map[string]any) {
	if logger != nil {
		for k, v := range ctx {
			logger.context[k] = v
		}
	}
}

// logWithContext ensures all logs include the injected context.
func (l *Logger) logWithContext(level log.Level, msg string, keyvals ...any) {
	if l == nil || l.logger == nil {
		return // Skip logging if the logger is not initialized
	}

	// Merge the context into the key-values
	var mergedKeyvals []any
	for k, v := range l.context {
		mergedKeyvals = append(mergedKeyvals, k, v)
	}
	mergedKeyvals = append(mergedKeyvals, keyvals...)

	l.logger.Log(level, msg, mergedKeyvals...)
}

// Info logs an informational message with the current context.
func Info(msg string, keyvals ...any) {
	if logger != nil {
		logger.logWithContext(log.InfoLevel, msg, keyvals...)
	}
}

// Warn logs a warning message with the current context.
func Warn(msg string, keyvals ...any) {
	if logger != nil {
		logger.logWithContext(log.WarnLevel, msg, keyvals...)
	}
}

// Error logs an error message with the current context.
func Error(msg string, err error, keyvals ...any) {
	if logger != nil {
		if err != nil {
			keyvals = append(keyvals, "error", err.Error())
		}
		logger.logWithContext(log.ErrorLevel, msg, keyvals...)
	}
}

// ErrorToFile logs an error message with the current context to the log file only,
// even in debug mode. It is a no-op when no log file is open.
func ErrorToFile(msg string, err error, keyvals ...any) {
	if logger == nil || logger.logFile == nil || logger.silentMode {
		return
	}
	if err != nil {
		keyvals = append(keyvals, "error", err.Error())
	}
	fileLogger := &Logger{logger: log.New(logger.logFile), context: logger.context}
	fileLogger.logger.SetLevel(logger.logger.GetLevel())
	fileLogger.logWithContext(log.ErrorLevel, msg, keyvals...)
}

// Fatal logs a fatal error and exits the application.
func Fatal(msg string, err error, keyvals ...any) {
	if logger != nil {
		if err != nil {
			keyvals = append(keyvals, "error", err.Error())
		}
		logger.logWithContext(log.FatalLevel, msg, keyvals...)
		os.Exit(1)
	}
}

// Debug logs a debug message with the current context.
func Debug(msg string, keyvals ...any) {
	if logger != nil {
		logger.logWithContext(log.DebugLevel, msg, keyvals...)
	}
}

// Print outputs a message to stdout if not in silent/test mode, otherwise logs it.
// This replaces fmt.Printf calls throughout the codebase for consistent output handling.
func Print(msg string, args ...any) {
	formattedMsg := fmt.Sprintf(msg, args...)

	if logger == nil {
		// No logger initialized, print to stdout as fallback
		fmt.Print(formattedMsg)
		return
	}

	if logger.silentMode || logger.testMode {
		// In silent or test mode, suppress all output completely
		return
	}

	// Normal mode: print to stdout
	fmt.Print(formattedMsg)
}

// printDiagnostic writes a diagnostic to stderr, following the same silent
// and test-mode rules as Print.
func printDiagnostic(msg string) {
	if logger != nil && (logger.silentMode || logger.testMode) {
		return
	}
	_, _ = fmt.Fprint(os.Stderr, msg)
}

// Printf outputs a formatted message to stdout if not in silent/test mode, otherwise logs it.
func Printf(format string, args ...any) {
	Print(format, args...)
}

// Println outputs a message with newline to stdout if not in silent/test mode, otherwise logs it.
func Println(msg string, args ...any) {
	if len(args) > 0 {
		Print("%s", fmt.Sprintf(msg, args...)+"\n")
	} else {
		Print("%s", msg+"\n")
	}
}

// Success prints a success message with green checkmark (if colors supported).
func Success(msg string, args ...any) {
	formattedMsg := fmt.Sprintf("✅ "+msg, args...)
	Print("%s", formattedMsg+"\n")
}

// Warning prints a warning message with yellow warning icon.
func Warning(msg string, args ...any) {
	formattedMsg := fmt.Sprintf("⚠️  "+msg, args...)
	// Warnings are diagnostics: keep them off stdout so machine-readable
	// output such as --format json stays parseable.
	printDiagnostic(formattedMsg + "\n")
	// Also log as warning
	if logger != nil {
		logger.logWithContext(log.WarnLevel, strings.TrimSpace(formattedMsg))
	}
}

// ErrorMsg prints an error message with red cross icon.
func ErrorMsg(msg string, args ...any) {
	formattedMsg := fmt.Sprintf("❌ "+msg, args...)
	Print("%s", formattedMsg+"\n")
	// Also log as error
	if logger != nil {
		logger.logWithContext(log.ErrorLevel, strings.TrimSpace(formattedMsg))
	}
}

// gatherBasicSystemInfo collects comprehensive system information without importing platform package
func gatherBasicSystemInfo() string {
	var sb strings.Builder

	sb.WriteString("=== DEVEX SYSTEM INFORMATION ===\n")
	fmt.Fprintf(&sb, "Timestamp: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&sb, "CLI Version: %s\n", cliVersion)

	// Basic platform info
	sb.WriteString("=== PLATFORM ===\n")
	fmt.Fprintf(&sb, "Operating System: %s\n", runtime.GOOS)
	fmt.Fprintf(&sb, "Architecture: %s\n", runtime.GOARCH)

	// Detect Linux distribution
	if runtime.GOOS == "linux" {
		if distro := detectLinuxDistribution(); distro != "" {
			fmt.Fprintf(&sb, "Distribution: %s\n", distro)
		}
		if version := getOSVersion(); version != "" {
			fmt.Fprintf(&sb, "OS Version: %s\n", version)
		}
		if desktop := getDesktopEnvironment(); desktop != "" && desktop != "unknown" {
			fmt.Fprintf(&sb, "Desktop Environment: %s\n", desktop)
		}
	}

	// Runtime info
	sb.WriteString("=== RUNTIME ===\n")
	fmt.Fprintf(&sb, "Go Version: %s\n", runtime.Version())
	fmt.Fprintf(&sb, "CPU Count: %d\n", runtime.NumCPU())

	if username := os.Getenv("USER"); username != "" {
		fmt.Fprintf(&sb, "Username: %s\n", username)
	}
	if shell := os.Getenv("SHELL"); shell != "" {
		fmt.Fprintf(&sb, "Shell: %s\n", shell)
	}
	if home := os.Getenv("HOME"); home != "" {
		fmt.Fprintf(&sb, "Home Directory: %s\n", home)
	}
	if pwd, err := os.Getwd(); err == nil {
		fmt.Fprintf(&sb, "Working Directory: %s\n", pwd)
	}

	// Environment
	sb.WriteString("=== ENVIRONMENT ===\n")
	if path := os.Getenv("PATH"); path != "" {
		if len(path) > 500 {
			path = path[:500] + "... [truncated]"
		}
		fmt.Fprintf(&sb, "PATH: %s\n", path)
	}

	sb.WriteString("=== END SYSTEM INFORMATION ===\n")
	return sb.String()
}

// detectLinuxDistribution detects the Linux distribution
func detectLinuxDistribution() string {
	// Try to read /etc/os-release
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "ID=") {
				return strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
		}
	}

	// Fallback to checking specific files
	distributions := map[string]string{
		"/etc/ubuntu-release":  "ubuntu",
		"/etc/debian_version":  "debian",
		"/etc/redhat-release":  "rhel",
		"/etc/centos-release":  "centos",
		"/etc/fedora-release":  "fedora",
		"/etc/arch-release":    "arch",
		"/etc/manjaro-release": "manjaro",
		"/etc/SUSE-release":    "opensuse",
	}

	for file, distro := range distributions {
		if _, err := os.Stat(file); err == nil {
			return distro
		}
	}

	return "unknown"
}

// getOSVersion gets the OS version
func getOSVersion() string {
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "VERSION_ID=") {
				return strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}
	return ""
}

// getDesktopEnvironment detects the desktop environment
func getDesktopEnvironment() string {
	if os.Getenv("GNOME_DESKTOP_SESSION_ID") != "" ||
		strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "GNOME") {
		return "gnome"
	}

	if os.Getenv("KDE_FULL_SESSION") != "" ||
		strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "KDE") {
		return "kde"
	}

	if strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "XFCE") {
		return "xfce"
	}

	if strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "Unity") {
		return "unity"
	}

	if os.Getenv("DESKTOP_SESSION") == "cinnamon" {
		return "cinnamon"
	}

	return "unknown"
}
