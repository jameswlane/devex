package log

import (
	"io"

	"github.com/charmbracelet/log"
)

// Logger wraps the charmbracelet logger with additional context support.
type Logger struct {
	logger  *log.Logger
	context map[string]any
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
		logger:  l,
		context: make(map[string]any),
	}
}

// InitDefaultLogger initializes the default logger with a specified writer.
func InitDefaultLogger(w io.Writer) {
	logger = New(w)
}

// SetLevel dynamically updates the logging level.
func SetLevel(level log.Level) {
	logger.logger.SetLevel(level)
}

// WithContext adds contextual metadata to the logger.
func WithContext(ctx map[string]any) {
	for k, v := range ctx {
		logger.context[k] = v
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
	logger.logWithContext(log.InfoLevel, msg, keyvals...)
}

// Warn logs a warning message with the current context.
func Warn(msg string, keyvals ...any) {
	logger.logWithContext(log.WarnLevel, msg, keyvals...)
}

// Error logs an error message with the current context.
func Error(msg string, err error, keyvals ...any) {
	keyvals = append(keyvals, err)
	logger.logWithContext(log.ErrorLevel, msg, keyvals...)
}

// Fatal logs a fatal error and exits the application.
func Fatal(msg string, err error, keyvals ...any) {
	keyvals = append(keyvals, err)
	logger.logWithContext(log.FatalLevel, msg, keyvals...)
}
