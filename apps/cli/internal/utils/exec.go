package utils

import (
	"context"
	"os/exec"
	"time"
)

type OSCommandExecutor struct{}

var CommandExec Interface = &OSCommandExecutor{} // Changed to a pointer

func (OSCommandExecutor) RunShellCommand(command string) (string, error) {
	// Use a 30-second timeout for shell commands to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (OSCommandExecutor) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (OSCommandExecutor) DownloadFileWithContext(ctx context.Context, url, filepath string) error {
	return DownloadFileWithContext(ctx, url, filepath)
}

func (OSCommandExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	return cmd, nil
}

func (OSCommandExecutor) ValidateCommand(command string) error {
	// Basic validation - let the real security validation happen elsewhere
	return nil
}
