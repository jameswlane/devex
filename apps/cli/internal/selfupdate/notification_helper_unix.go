//go:build !windows

package selfupdate

import (
	"os/exec"
	"syscall"
)

func detachNotificationHelper(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
