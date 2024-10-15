package testutils

import (
	"os/exec"
)

// MockExecCommand simulates a successful exec.Command
func MockExecCommand(command string, args ...string) *exec.Cmd {
	cmd := exec.Command("echo", append([]string{command}, args...)...)
	return cmd
}

// MockCommandWithError simulates an error from exec.Command
func MockCommandWithError(command string, args ...string) *exec.Cmd {
	return exec.Command("false")
}
