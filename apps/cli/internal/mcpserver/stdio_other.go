//go:build !unix

package mcpserver

import (
	"errors"
	"os"
)

// ReserveStdout is unavailable on this platform, so mutating tools, which run
// installers that may write to stdout, cannot be served.
func ReserveStdout() (*os.File, error) {
	return nil, errors.New("--allow-mutations is supported on Linux and macOS only")
}
