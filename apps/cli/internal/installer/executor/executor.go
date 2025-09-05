// Package executor provides command execution functionality with security validation
package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Executor defines the interface for command execution
type Executor interface {
	// Execute executes a command with the given context and returns the command handle
	Execute(ctx context.Context, command string) (*exec.Cmd, error)
	// Validate validates a command for security before execution
	Validate(command string) error
}

// Default implements Executor using the standard approach
type Default struct{}

// New creates a new default command executor
func New() *Default {
	return &Default{}
}

// Execute implements Executor.Execute
func (e *Default) Execute(ctx context.Context, command string) (*exec.Cmd, error) {
	// Validate command first
	if err := e.Validate(command); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Parse and execute using safest method
	executable, args, needsShell := ParseCommand(command)

	var cmd *exec.Cmd
	if needsShell {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	} else {
		cmd = exec.CommandContext(ctx, executable, args...)
	}

	cmd.SysProcAttr = getPlatformSysProcAttr()

	return cmd, nil
}

// Validate implements Executor.Validate using the security package
func (e *Default) Validate(command string) error {
	validator := security.NewCommandValidator(security.SecurityLevelModerate)
	return validator.ValidateCommand(command)
}

// Secure implements Executor using pattern-based validation
type Secure struct {
	validator *security.CommandValidator
}

// NewSecure creates a new secure command executor
func NewSecure(level security.SecurityLevel, apps []types.CrossPlatformApp) *Secure {
	return &Secure{
		validator: security.NewCommandValidator(level),
	}
}

// Execute implements Executor.Execute for Secure
func (s *Secure) Execute(ctx context.Context, command string) (*exec.Cmd, error) {
	// Validate command using pattern-based approach
	if err := s.validator.ValidateCommand(command); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Parse and execute using safest method
	executable, args, needsShell := ParseCommand(command)

	var cmd *exec.Cmd
	if needsShell {
		cmd = exec.CommandContext(ctx, "bash", "-c", command)
	} else {
		cmd = exec.CommandContext(ctx, executable, args...)
	}

	cmd.SysProcAttr = getPlatformSysProcAttr()

	return cmd, nil
}

// Validate implements Executor.Validate for Secure
func (s *Secure) Validate(command string) error {
	return s.validator.ValidateCommand(command)
}

// ParseCommand safely parses a command string into executable parts
// Returns (executable, args, needsShell) where needsShell indicates if shell execution is required
func ParseCommand(command string) (string, []string, bool) {
	// Trim whitespace
	command = strings.TrimSpace(command)
	if command == "" {
		return "", nil, false
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

	return parts[0], parts[1:], false
}

// getPlatformSysProcAttr returns platform-specific security attributes
func getPlatformSysProcAttr() *syscall.SysProcAttr {
	return getSysProcAttr()
}
