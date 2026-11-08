//go:build unix

package sdk

import (
	"os/exec"
	"syscall"
	"time"
)

// Give SDK plugin entry points time to forward cancellation to native work.
func configureCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second
}
