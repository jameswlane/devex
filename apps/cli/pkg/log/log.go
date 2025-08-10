package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// Logger wraps the charmbracelet logger with additional context support.
type Logger struct {
	logger    *log.Logger
	context   map[string]any
	logFile   *os.File
	debugMode bool
}

var logger *Logger

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
		logger:    l,
		context:   make(map[string]any),
		debugMode: false,
	}
}

// InitDefaultLogger initializes the default logger with a specified writer.
func InitDefaultLogger(w io.Writer) {
	logger = New(w)
}

// InitFileLogger initializes a file-based logger with optional debug mode.
func InitFileLogger(debugMode bool) error {
	// Create logs directory
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/tmp" // Fallback for systems without HOME
	}

	logsDir := filepath.Join(homeDir, ".local", "share", "devex", "logs")
	if err := os.MkdirAll(logsDir, 0750); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	var writer io.Writer

	if debugMode {
		// Debug mode: log to both file and stderr
		timestamp := time.Now().Format("20060102-150405")
		logFile := filepath.Join(logsDir, fmt.Sprintf("devex-%s.log", timestamp))

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}

		// Write to both file and stderr in debug mode
		writer = io.MultiWriter(file, os.Stderr)

		logger = &Logger{
			logger:    log.New(writer),
			context:   make(map[string]any),
			logFile:   file,
			debugMode: true,
		}
	} else {
		// Normal mode: log only to file
		timestamp := time.Now().Format("20060102-150405")
		logFile := filepath.Join(logsDir, fmt.Sprintf("devex-%s.log", timestamp))

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return fmt.Errorf("failed to create log file: %w", err)
		}

		// Write initial header
		header := fmt.Sprintf("DevEx Log - Started: %s\nPlatform: %s %s\nMode: %s\n%s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			os.Getenv("USER"),
			os.Getenv("HOSTNAME"),
			getMode(debugMode),
			strings.Repeat("-", 50))
		_, _ = file.WriteString(header)

		writer = file

		logger = &Logger{
			logger:    log.New(writer),
			context:   make(map[string]any),
			logFile:   file,
			debugMode: false,
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
	mergedKeyvals := make([]any, 0, len(l.context)*2+len(keyvals))
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
