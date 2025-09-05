//go:build windows

package executor

import (
	"syscall"
)

// getSysProcAttr returns platform-specific process attributes for Windows systems
func getSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		// Windows-specific security attributes could be added here
	}
}
