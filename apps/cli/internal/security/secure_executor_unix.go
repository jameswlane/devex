//go:build unix

package security

import (
	"os/exec"
	"syscall"
)

// setPlatformSpecificAttrs sets Unix-specific process attributes
func setPlatformSpecificAttrs(cmd *exec.Cmd) {
	// Set process group for better signal handling on Unix systems
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}
