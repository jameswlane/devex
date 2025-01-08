package utils

import "context"

type Interface interface {
	RunShellCommand(command string) (string, error)
	RunCommand(ctx context.Context, name string, args ...string) (string, error)
	DownloadFileWithContext(ctx context.Context, url, filepath string) error
}
