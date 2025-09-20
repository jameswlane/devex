//go:build unix

package tui

import (
	"runtime"
	"syscall"
)

// getPlatformSysProcAttr returns platform-specific process attributes for Unix systems
func getPlatformSysProcAttr() *syscall.SysProcAttr {
	switch runtime.GOOS {
	case "linux", "darwin":
		return &syscall.SysProcAttr{
			// Create new process group to isolate from parent
			Setpgid: true,
			Pgid:    0,
		}
	case "windows":
		return &syscall.SysProcAttr{
			// Windows-specific security attributes could be added here
		}
	default:
		return &syscall.SysProcAttr{}
	}
}
