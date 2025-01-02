package log

import (
	"io"
	"os"

	"github.com/charmbracelet/log"
)

type Logger struct {
	log.Logger
}

var logger = log.New(os.Stderr)

// ConfigureLogger sets the global logger's level and output.
func ConfigureLogger(level log.Level, output io.Writer) {
	logger.SetLevel(level)
	logger.SetOutput(output)
}

// SetTimeFormat updates the timestamp format used in logs.
func SetTimeFormat(format string) {
	logger.SetTimeFormat(format)
}

// Logging methods
func Error(msg any, keyvals ...any) {
	logger.Error(msg, keyvals...)
}

func Debug(msg any, keyvals ...any) {
	logger.Debug(msg, keyvals...)
}

func Info(msg any, keyvals ...any) {
	logger.Info(msg, keyvals...)
}

func Warn(msg any, keyvals ...any) {
	logger.Warn(msg, keyvals...)
}

func Fatal(msg any, keyvals ...any) {
	logger.Fatal(msg, keyvals...)
}

func Print(msg any, keyvals ...any) {
	logger.Print(msg, keyvals...)
}

// With creates a sub-logger with additional context.
func With(keyvals ...any) *Logger {
	return &Logger{Logger: *logger.With(keyvals...)}
}

// UseTestLogger sets the logger to discard all output (for tests).
func UseTestLogger() {
	ConfigureLogger(log.DebugLevel, io.Discard)
}
