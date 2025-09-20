package sdk_test

import "github.com/jameswlane/devex/packages/plugin-sdk"

// SilentLogger is a test logger that suppresses all output
type SilentLogger struct{}

func NewSilentLogger() sdk.Logger {
	return &SilentLogger{}
}

func (l *SilentLogger) Printf(format string, args ...any)        {}
func (l *SilentLogger) Println(msg string, args ...any)           {}
func (l *SilentLogger) Success(msg string, args ...any)           {}
func (l *SilentLogger) Warning(msg string, args ...any)           {}
func (l *SilentLogger) ErrorMsg(msg string, args ...any)          {}
func (l *SilentLogger) Info(msg string, keyvals ...any)           {}
func (l *SilentLogger) Warn(msg string, keyvals ...any)           {}
func (l *SilentLogger) Error(msg string, err error, keyvals ...any) {}
func (l *SilentLogger) Debug(msg string, keyvals ...any)          {}
