//go:build !unix

package mcpserver

import (
	"errors"
	"os"
)

// Stdio is unavailable on this platform.
type Stdio struct {
	Stdin  *os.File
	Stdout *os.File
}

// Close does nothing.
func (s *Stdio) Close() error { return nil }

// ReserveStdout is unavailable on this platform, so mutating tools, which run
// installers that may write to stdout, cannot be served.
func ReserveStdout() (*Stdio, error) {
	return nil, errors.New("--allow-mutations is supported on Linux and macOS only")
}
