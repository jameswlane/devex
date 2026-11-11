//go:build unix

package mcpserver

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// ReserveStdout keeps the process's original stdout for MCP messages and
// points file descriptor 1 at stderr. Anything else that writes to stdout
// afterwards, including installers, plugins and their child processes, then
// writes to stderr and cannot corrupt the MCP stream.
func ReserveStdout() (*os.File, error) {
	saved, err := unix.Dup(1)
	if err != nil {
		return nil, fmt.Errorf("reserve stdout for MCP: %w", err)
	}
	unix.CloseOnExec(saved)
	if err := unix.Dup2(2, 1); err != nil {
		_ = unix.Close(saved)
		return nil, fmt.Errorf("redirect stdout to stderr: %w", err)
	}
	return os.NewFile(uintptr(saved), "mcp-stdout"), nil
}
