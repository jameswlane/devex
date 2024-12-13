package logger

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Logger struct to hold logs
type Logger struct {
	logs []string
}

// InitLogger initializes and returns a Logger instance
func InitLogger() *Logger {
	return &Logger{
		logs: []string{},
	}
}

// LogInfo logs an informational message
func (l *Logger) LogInfo(msg string) {
	l.logs = append(l.logs, fmt.Sprintf("INFO: %s", msg))
}

// LogError logs an error message
func (l *Logger) LogError(msg string, err error) {
	l.logs = append(l.logs, fmt.Sprintf("ERROR: %s - %v", msg, err))
}

// ExecCommandWithLogging runs a command and logs its output
func (l *Logger) ExecCommandWithLogging(cmd *exec.Cmd) error {
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		l.LogError(fmt.Sprintf("Failed to run command: %s", cmd.String()), err)
		return err
	}

	// Log command output
	l.LogInfo(fmt.Sprintf("Command output: %s", out.String()))
	if stderr.Len() > 0 {
		l.LogInfo(fmt.Sprintf("Command error output: %s", stderr.String()))
	}

	return nil
}

// GetLogs returns all logs
func (l *Logger) GetLogs() []string {
	return l.logs
}
