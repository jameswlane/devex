package utils

import (
	"context"
	"os/exec"
)

type OSCommandExecutor struct{}

var CommandExec Interface = &OSCommandExecutor{} // Changed to a pointer

func (OSCommandExecutor) RunShellCommand(command string) (string, error) {
	ctx := context.Background()
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
