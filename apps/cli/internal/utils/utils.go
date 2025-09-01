package utils

import (
	"context"
	"os/exec"
)

type Interface interface {
	RunShellCommand(command string) (string, error)
	RunCommand(ctx context.Context, name string, args ...string) (string, error)
	DownloadFileWithContext(ctx context.Context, url, filepath string) error
	ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error)
	ValidateCommand(command string) error
}
