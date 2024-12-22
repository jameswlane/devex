package utils

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
)

func RunShellCommand(command string) error {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute shell command: %v - %s", err, string(output))
	}
	log.Info("Executed shell command", "command", command)
	return nil
}

func ExtendSudoSession() error {
	if err := exec.Command("sudo", "-v").Run(); err != nil {
		return fmt.Errorf("failed to start sudo session: %v", err)
	}

	go func() {
		for {
			time.Sleep(60 * time.Second)
			if err := exec.Command("sudo", "-n", "true").Run(); err != nil {
				break
			}
		}
	}()
	return nil
}
