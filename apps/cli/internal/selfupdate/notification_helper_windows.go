package selfupdate

import (
	"os/exec"
	"syscall"
)

func detachNotificationHelper(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008} // DETACHED_PROCESS
}
