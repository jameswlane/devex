//go:build !windows

package tui

import "syscall"

// getPlatformSysProcAttr returns Unix-specific SysProcAttr configuration
func (ce *DefaultCommandExecutor) getPlatformSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}
