package bootstrap

import (
	"github.com/jameswlane/devex/apps/cli/internal/log"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// SDKLoggerAdapter adapts the CLI's log package to implement the SDK's Logger interface
// This allows SDK operations to log to the CLI's log file without polluting stdout during TUI operations
type SDKLoggerAdapter struct {
	silent bool // When true, only logs to file (no stdout)
}

// NewSDKLoggerAdapter creates a new logger adapter
func NewSDKLoggerAdapter(silent bool) sdk.Logger {
	return &SDKLoggerAdapter{
		silent: silent,
	}
}

// Printf implements sdk.Logger interface
func (l *SDKLoggerAdapter) Printf(format string, args ...any) {
	if !l.silent {
		log.Printf(format, args...)
	}
}

// Println implements sdk.Logger interface
func (l *SDKLoggerAdapter) Println(msg string, args ...any) {
	if !l.silent {
		if len(args) > 0 {
			log.Printf(msg+"\n", args...)
		} else {
			log.Println("%s", msg)
		}
	}
}

// Success implements sdk.Logger interface
func (l *SDKLoggerAdapter) Success(msg string, args ...any) {
	log.Info(msg, args...)
}

// Warning implements sdk.Logger interface
func (l *SDKLoggerAdapter) Warning(msg string, args ...any) {
	log.Warning(msg, args...)
}

// ErrorMsg implements sdk.Logger interface
func (l *SDKLoggerAdapter) ErrorMsg(msg string, args ...any) {
	log.Error(msg, nil, args...)
}

// Info implements sdk.Logger interface
func (l *SDKLoggerAdapter) Info(msg string, keyvals ...any) {
	log.Info(msg, keyvals...)
}

// Warn implements sdk.Logger interface
func (l *SDKLoggerAdapter) Warn(msg string, keyvals ...any) {
	log.Warn(msg, keyvals...)
}

// Error implements sdk.Logger interface
func (l *SDKLoggerAdapter) Error(msg string, err error, keyvals ...any) {
	log.Error(msg, err, keyvals...)
}

// Debug implements sdk.Logger interface
func (l *SDKLoggerAdapter) Debug(msg string, keyvals ...any) {
	log.Debug(msg, keyvals...)
}
