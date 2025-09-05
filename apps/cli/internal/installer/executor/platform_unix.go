//go:build unix

package executor

import (
	"runtime"
	"syscall"
)

// getSysProcAttr returns platform-specific process attributes for Unix systems
func getSysProcAttr() *syscall.SysProcAttr {
	switch runtime.GOOS {
	case "linux", "darwin":
		return &syscall.SysProcAttr{
			// Create new process group to isolate from parent
			Setpgid: true,
			Pgid:    0,
		}
	default:
		return &syscall.SysProcAttr{}
	}
}
