package utils

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// ShellValidator provides validation for shell commands
type ShellValidator struct {
	// Patterns that indicate potentially dangerous commands
	dangerousPatterns []*regexp.Regexp
}

// NewShellValidator creates a new shell validator with default security patterns
func NewShellValidator() *ShellValidator {
	return &ShellValidator{
		dangerousPatterns: []*regexp.Regexp{
			// Destructive file operations on critical paths
			regexp.MustCompile(`\brm\s+(-[rfRi]*\s+)*(/|/home|/usr|/var|/etc|/boot|/sys|/proc)(\s|$)`),
			regexp.MustCompile(`\bdd\s+.*\bof=/dev/(sd[a-z]|hd[a-z]|nvme\d+n\d+|loop\d+)\b`),
			regexp.MustCompile(`\bmkfs(\.[a-zA-Z0-9]+)?\s+/dev/`),

			// Fork bombs and resource exhaustion
			regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`),
			regexp.MustCompile(`\bwhile\s+true\s*;\s*do\s+:`),

			// Command injection attempts
			regexp.MustCompile(`[;&|]\s*rm\s+-rf\s+/`),
			regexp.MustCompile(`\$\(.*rm\s+-rf.*\)`),
			regexp.MustCompile("`.*rm\\s+-rf.*`"),
		},
	}
}

// ValidateCommand checks if a command is safe to execute
func (v *ShellValidator) ValidateCommand(command string) error {
	// Check for dangerous patterns
	for _, pattern := range v.dangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains potentially dangerous pattern")
		}
	}

	return nil
}

// ValidatePackageName validates that a package name is safe for shell use
func ValidatePackageName(name string) error {
	if name == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	// Trim whitespace and check if empty after trimming
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("package name contains invalid characters")
	}

	// Check for shell metacharacters that could lead to command injection
	if strings.ContainsAny(name, ";&|`$()[]{}*?<>\n\r\t\"'\\") {
		return fmt.Errorf("package name contains invalid characters")
	}

	// Check for path traversal attempts and problematic names
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		return fmt.Errorf("package name contains path traversal characters")
	}

	// Check for problematic edge cases
	if name == "." || name == "-" || name == "--" {
		return fmt.Errorf("package name contains invalid characters")
	}

	// Limit length to prevent buffer issues
	if len(name) > 255 {
		return fmt.Errorf("package name too long")
	}

	return nil
}

// IsCommandAvailable checks if a command is available in the system PATH
func IsCommandAvailable(ctx context.Context, command string) bool {
	if err := ValidatePackageName(command); err != nil {
		return false
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "command", "-v", command)
	return cmd.Run() == nil
}

// ValidateUsername validates that a username is safe for shell use
func ValidateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	// Check for shell metacharacters
	if strings.ContainsAny(username, ";&|`$()[]{}*?<>\n\r\t\"'\\") {
		return fmt.Errorf("username contains invalid characters")
	}

	// Check for path components
	if strings.Contains(username, "/") || strings.Contains(username, "..") {
		return fmt.Errorf("username contains path characters")
	}

	// Limit length
	if len(username) > 32 {
		return fmt.Errorf("username too long")
	}

	return nil
}

// SafeCommandExecutor provides secure command execution
type SafeCommandExecutor struct {
	validator *ShellValidator
	executor  Interface
}

// NewSafeCommandExecutor creates a new safe command executor
func NewSafeCommandExecutor(executor Interface) *SafeCommandExecutor {
	return &SafeCommandExecutor{
		validator: NewShellValidator(),
		executor:  executor,
	}
}

// RunShellCommandSafe validates and executes a shell command
func (s *SafeCommandExecutor) RunShellCommandSafe(command string) (string, error) {
	if err := s.validator.ValidateCommand(command); err != nil {
		return "", fmt.Errorf("command validation failed: %w", err)
	}

	return s.executor.RunShellCommand(command)
}

// RunCommandWithArgs executes a command with properly escaped arguments
// This is the preferred method for running commands with user input
func (s *SafeCommandExecutor) RunCommandWithArgs(ctx context.Context, name string, args ...string) (string, error) {
	// This method doesn't use shell interpretation, so it's inherently safer
	return s.executor.RunCommand(ctx, name, args...)
}

// RunPackageCommand safely executes a package manager command
func RunPackageCommand(ctx context.Context, packageManager string, operation string, packages []string) (string, error) {
	// Validate all package names
	for _, pkg := range packages {
		if err := ValidatePackageName(pkg); err != nil {
			return "", fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}
	}

	// Build command arguments without shell interpretation
	var args []string

	switch packageManager {
	case "apt", "apt-get":
		args = append(args, operation, "-y")
		args = append(args, packages...)
	case "dnf", "yum":
		args = append(args, operation, "-y")
		args = append(args, packages...)
	case "pacman":
		switch operation {
		case "install":
			args = append(args, "-S", "--noconfirm")
		case "remove":
			args = append(args, "-R", "--noconfirm")
		}
		args = append(args, packages...)
	case "zypper":
		args = append(args, operation, "-y")
		args = append(args, packages...)
	default:
		return "", fmt.Errorf("unsupported package manager: %s", packageManager)
	}

	// Execute with sudo if needed
	cmd := exec.CommandContext(ctx, "sudo", append([]string{packageManager}, args...)...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// CheckPackageInstalled safely checks if a package is installed
func CheckPackageInstalled(ctx context.Context, packageName string) (bool, error) {
	// Validate package name
	if err := ValidatePackageName(packageName); err != nil {
		return false, fmt.Errorf("invalid package name: %w", err)
	}

	// Use dpkg-query with proper argument separation (no shell interpretation)
	cmd := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Status}", packageName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// dpkg-query returns non-zero if package is not found
		// This is expected behavior, not an error
		return false, nil
	}

	// Check if the package is installed and configured properly
	return strings.Contains(string(output), "install ok installed"), nil
}

// WaitForService waits for a service to become ready
func WaitForService(ctx context.Context, serviceName string, timeout time.Duration) error {
	// Validate service name
	if err := ValidatePackageName(serviceName); err != nil {
		return fmt.Errorf("invalid service name: %w", err)
	}

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check service status
		cmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
		output, _ := cmd.CombinedOutput()

		if strings.TrimSpace(string(output)) == "active" {
			return nil
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			// Continue checking
		}
	}

	return fmt.Errorf("service %s did not become ready within %v", serviceName, timeout)
}

// WaitForDockerDaemon waits for Docker daemon to become ready
func WaitForDockerDaemon(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// Check Docker daemon status using version command
		cmd := exec.CommandContext(ctx, "docker", "version", "--format", "{{.Server.Version}}")
		if output, err := cmd.CombinedOutput(); err == nil && len(output) > 0 {
			return nil
		}

		// Try with sudo
		sudoCmd := exec.CommandContext(ctx, "sudo", "docker", "version", "--format", "{{.Server.Version}}")
		if output, err := sudoCmd.CombinedOutput(); err == nil && len(output) > 0 {
			return nil
		}

		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
			// Continue checking
		}
	}

	return fmt.Errorf("docker daemon did not become ready within %v", timeout)
}
