// Package stream handles output streaming, logging, and terminal interaction for installers
package stream

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// Program interface allows for testing and loose coupling
type Program interface {
	Send(tea.Msg)
}

// Config holds configuration for streaming operations
type Config struct {
	MaxLogLines         int
	InputTimeout        time.Duration
	InitializationDelay time.Duration
}

// DefaultConfig returns default streaming configuration
func DefaultConfig() Config {
	return Config{
		MaxLogLines:         1000,
		InputTimeout:        30 * time.Second,
		InitializationDelay: 500 * time.Millisecond,
	}
}

// Manager handles output streaming and logging
type Manager struct {
	program  Program
	config   Config
	logMutex sync.Mutex
	ctx      context.Context
}

// New creates a new stream manager
func New(program Program, ctx context.Context) *Manager {
	return &Manager{
		program: program,
		config:  DefaultConfig(),
		ctx:     ctx,
	}
}

// NewWithConfig creates a stream manager with custom configuration
func NewWithConfig(program Program, ctx context.Context, config Config) *Manager {
	return &Manager{
		program: program,
		config:  config,
		ctx:     ctx,
	}
}

// StreamOutput streams command output to the TUI with proper error handling
func (m *Manager) StreamOutput(reader io.Reader, source string) {
	scanner := m.createScanner(reader)
	var currentLine string

	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-m.ctx.Done():
			m.Log("INFO", fmt.Sprintf("%s stream cancelled", source))
			return
		default:
		}

		line := scanner.Text()
		line = CleanTerminalOutput(line)

		// Skip empty lines and apt database reading progress
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Filter out progress messages that cause display issues
		if m.isProgressLine(line) {
			if m.isProgressComplete(line) {
				currentLine = line
			} else {
				currentLine = line
				continue
			}
		}

		// Send the cleaned line
		if currentLine != "" {
			m.Log(source, currentLine)
			currentLine = ""
		} else {
			m.Log(source, line)
		}
	}

	// Send any remaining line
	if currentLine != "" {
		m.Log(source, currentLine)
	}

	// Check for scanner errors (but ignore closed pipe errors)
	if err := scanner.Err(); err != nil && !strings.Contains(err.Error(), "file already closed") {
		m.Log("ERROR", fmt.Sprintf("Scanner error in %s: %v", source, err))
	}
}

// MonitorForInput monitors stderr for password prompts and requests user input
func (m *Manager) MonitorForInput(stderr io.Reader, stdin io.WriteCloser, inputHandler InputHandler) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for common password prompts
		if m.isPasswordPrompt(line) {
			// Skip if no program (for testing)
			if m.program == nil {
				return
			}

			response := inputHandler.RequestInput(m.ctx, line, m.config.InputTimeout)
			if response != "" {
				if _, err := stdin.Write([]byte(response + "\n")); err != nil {
					m.Log("ERROR", fmt.Sprintf("Failed to write input: %v", err))
				}
			}
		}
	}
}

// Log sends a log message to both the TUI and persistent log file
func (m *Manager) Log(level, message string) {
	m.logMutex.Lock()
	defer m.logMutex.Unlock()

	// Always write to persistent log file first
	m.writeToLogFile(level, message)

	// Skip TUI sending when program is nil (during testing)
	if m.program == nil {
		return
	}

	// Add panic protection for program.Send calls
	defer func() {
		if r := recover(); r != nil {
			log.Error(fmt.Sprintf("TUI unavailable, message: %s", message), nil, "source", "stream_panic")
		}
	}()

	// Check if context is cancelled before sending to TUI
	select {
	case <-m.ctx.Done():
		log.Info("Context cancelled while sending to TUI", "level", level, "message", message)
		return
	default:
		m.sendToTUI(level, message)
	}
}

// CleanTerminalOutput removes ANSI escape sequences and control characters
func CleanTerminalOutput(s string) string {
	// Remove ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	s = ansiRegex.ReplaceAllString(s, "")

	// Remove cursor positioning sequences
	cursorRegex := regexp.MustCompile(`\x1b\[[0-9]*[ABCD]`)
	s = cursorRegex.ReplaceAllString(s, "")

	// Remove additional control sequences
	ctrlRegex := regexp.MustCompile(`\x1b[()][0-9A-Z]`)
	s = ctrlRegex.ReplaceAllString(s, "")

	// Remove carriage returns
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

// createScanner creates a scanner that handles both newlines and carriage returns
func (m *Manager) createScanner(reader io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(reader)

	// Custom split function that handles both newlines and carriage returns
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}

		// Look for \n or \r
		for i := 0; i < len(data); i++ {
			if data[i] == '\n' {
				return i + 1, data[:i], nil
			}
			if data[i] == '\r' {
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

	return scanner
}

// isProgressLine checks if a line is a progress indicator
func (m *Manager) isProgressLine(line string) bool {
	progressIndicators := []string{
		"Reading database",
		"Scanning processes",
		"Scanning candidates",
		"Scanning linux images",
		"Readin database", // Handle partial lines
	}

	for _, indicator := range progressIndicators {
		if strings.Contains(line, indicator) {
			return true
		}
	}
	return false
}

// isProgressComplete checks if a progress line is complete
func (m *Manager) isProgressComplete(line string) bool {
	return strings.Contains(line, "done") ||
		strings.Contains(line, "100%") ||
		strings.Contains(line, "... done")
}

// isPasswordPrompt checks if a line is a password prompt
func (m *Manager) isPasswordPrompt(line string) bool {
	lowerLine := strings.ToLower(line)
	return strings.Contains(lowerLine, "password") &&
		(strings.Contains(lowerLine, "sudo") ||
			strings.Contains(lowerLine, "enter") ||
			strings.Contains(lowerLine, ":"))
}

// writeToLogFile writes to the persistent log file
func (m *Manager) writeToLogFile(level, message string) {
	switch strings.ToUpper(level) {
	case "ERROR", "STDERR":
		log.Error(message, nil, "source", "installer_stream")
	case "WARN", "WARNING", "CAUTION":
		log.Warn(message, "source", "installer_stream")
	case "DEBUG":
		log.Debug(message, "source", "installer_stream")
	case "INFO", "STDOUT":
		log.Info(message, "source", "installer_stream")
	case "CRITICAL":
		log.Error(message, nil, "source", "installer_stream", "critical", true)
	default:
		log.Info(fmt.Sprintf("[%s] %s", level, message), "source", "installer_stream")
	}
}

// sendToTUI sends a message to the TUI program
func (m *Manager) sendToTUI(level, message string) {
	// This would send to the TUI program
	// The actual message type would depend on your TUI implementation
	// For now, we'll just use a generic interface
	if m.program != nil {
		m.program.Send(LogMessage{
			Level:     level,
			Message:   message,
			Timestamp: time.Now(),
		})
	}
}

// LogMessage represents a log message for the TUI
type LogMessage struct {
	Level     string
	Message   string
	Timestamp time.Time
}

// InputHandler interface for handling user input requests
type InputHandler interface {
	RequestInput(ctx context.Context, prompt string, timeout time.Duration) string
}
