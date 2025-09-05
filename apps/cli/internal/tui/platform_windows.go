//go:build windows

package tui

import (
	"syscall"
)

// getPlatformSysProcAttr returns platform-specific process attributes for Windows systems
func getPlatformSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		// Windows-specific security attributes could be added here
	}
}
