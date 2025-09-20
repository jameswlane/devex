package utils

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// ExecAsUser executes a command as a specific user.
func ExecAsUser(command string, args ...string) (string, error) {
	log.Info("Preparing to execute command as user", "command", command, "args", strings.Join(args, " "))

	// Determine the target user
	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		targetUser = os.Getenv("USER")
	}
	if targetUser == "" {
		return "", fmt.Errorf("%w: unable to determine target user", ErrUserNotFound)
	}
	log.Info("Executing command as user", "user", targetUser, "command", command, "args", strings.Join(args, " "))

	// Construct the user-specific command
	fullCommand := fmt.Sprintf("sudo -u %s %s", targetUser, command)

	// Execute the command via CommandExec
	return CommandExec.RunShellCommand(fullCommand)
}

// ExecAsUserWithContext executes a command as a specific user with context support.
func ExecAsUserWithContext(ctx context.Context, command string, args ...string) (string, error) {
	log.Info("Preparing to execute command as user with context", "command", command, "args", strings.Join(args, " "))

	// Determine the target user
	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		targetUser = os.Getenv("USER")
	}
	if targetUser == "" {
		return "", fmt.Errorf("%w: unable to determine target user", ErrUserNotFound)
	}
	log.Info("Executing command as user with context", "user", targetUser, "command", command, "args", strings.Join(args, " "))

	// Construct the user-specific command
	fullCommand := fmt.Sprintf("sudo -u %s %s", targetUser, command)

	// Execute the command via CommandExec
	return CommandExec.RunCommand(ctx, "bash", "-c", fullCommand)
}
