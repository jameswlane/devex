//go:build windows

package tui

import "syscall"

// getPlatformSysProcAttr returns Windows-specific SysProcAttr configuration
func (ce *DefaultCommandExecutor) getPlatformSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		// Windows doesn't support Setpgid - use different process management
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
