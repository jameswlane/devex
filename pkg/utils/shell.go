package utils

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/log"
)

func RunShellCommand(command string) error {
	log.Info("Starting RunShellCommand", "command", command)
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to execute shell command", "command", command, "error", err, "output", string(output))
		return fmt.Errorf("failed to execute shell command: %v - %s", err, string(output))
	}
	log.Info("Executed shell command successfully", "command", command, "output", string(output))
	return nil
}

func ExtendSudoSession() error {
	log.Info("Starting ExtendSudoSession")
	if err := exec.Command("sudo", "-v").Run(); err != nil {
		log.Error("Failed to start sudo session", "error", err)
		return fmt.Errorf("failed to start sudo session: %v", err)
	}

	go func() {
		for {
			time.Sleep(60 * time.Second)
			if err := exec.Command("sudo", "-n", "true").Run(); err != nil {
				log.Warn("Failed to extend sudo session", "error", err)
				break
			}
			log.Info("Extended sudo session")
		}
	}()
	log.Info("ExtendSudoSession initialized successfully")
	return nil
}
