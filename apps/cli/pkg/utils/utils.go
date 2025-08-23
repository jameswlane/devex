package utils

import (
	"context"
	"time"
)

type Interface interface {
	RunShellCommand(command string) (string, error)
	RunShellCommandWithTimeout(command string, timeout time.Duration) (string, error)
	RunCommand(ctx context.Context, name string, args ...string) (string, error)
	DownloadFileWithContext(ctx context.Context, url, filepath string) error
}
