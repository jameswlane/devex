//go:build !windows

package tui

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// getPlatformSysProcAttr returns Unix-specific SysProcAttr configuration
func (ce *DefaultCommandExecutor) getPlatformSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func configureCommandCancellation(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		return
	}
	cmd.WaitDelay = time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
