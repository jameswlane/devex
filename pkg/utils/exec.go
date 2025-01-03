package utils

import (
	"bytes"
	"os/exec"
)

type CommandExecutor interface {
	RunCommand(name string, args ...string) (string, error)
}

type OSCommandExecutor struct{}

func (OSCommandExecutor) RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

var CommandExec CommandExecutor = OSCommandExecutor{}

func RunCommand(name string, args ...string) (string, error) {
	return CommandExec.RunCommand(name, args...)
}
