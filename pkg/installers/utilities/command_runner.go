package utilities

import (
	"fmt"
	"os"
	"os/exec"
)

func RunCommand(command string) error {
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CopyFile(source, destination string) error {
	input, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("failed to read source file: %v", err)
	}
	if err := os.WriteFile(destination, input, 0o644); err != nil {
		return fmt.Errorf("failed to write destination file: %v", err)
	}
	return nil
}
