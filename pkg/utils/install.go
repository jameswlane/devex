package utils

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/types"
)

func ProcessInstallCommands(commands []types.InstallCommand, repo repository.Repository, dryRun bool) error {
	for _, cmd := range commands {
		if cmd.Shell != "" {
			processedCommand := ReplacePlaceholders(cmd.Shell)
			log.Info("Executing shell command", "command", processedCommand)
			if !dryRun {
				err := ExecAsUser(processedCommand, dryRun)
				if err != nil {
					log.Error("Failed to execute shell command", "command", processedCommand, "error", err)
					return fmt.Errorf("failed to execute command: %v", err)
				}
			}
		}

		if cmd.UpdateShellConfig != "" {
			processedCommand := ReplacePlaceholders(cmd.UpdateShellConfig)
			log.Info("Updating shell config with command", "command", processedCommand)
			if !dryRun {
				err := UpdateShellConfig([]string{processedCommand})
				if err != nil {
					log.Error("Failed to update shell config", "command", processedCommand, "error", err)
					return fmt.Errorf("failed to update shell config: %v", err)
				}
			}
		}

		if cmd.Copy.Source != "" && cmd.Copy.Destination != "" {
			source := ReplacePlaceholders(cmd.Copy.Source)
			destination := ReplacePlaceholders(cmd.Copy.Destination)
			log.Info("Copying file", "source", source, "destination", destination)
			if !dryRun {
				err := CopyFile(source, destination)
				if err != nil {
					log.Error("Failed to copy file", "source", source, "destination", destination, "error", err)
					return fmt.Errorf("failed to copy file from %s to %s: %v", source, destination, err)
				}
			}
		}
	}
	log.Info("Processed all install commands successfully")
	return nil
}

func ReplacePlaceholders(input string) string {
	user := os.Getenv("USER")
	home := os.Getenv("HOME")

	placeholders := map[string]string{
		"%USER%": user,
		"%HOME%": home,
	}

	for placeholder, value := range placeholders {
		input = strings.ReplaceAll(input, placeholder, value)
	}

	log.Info("Replaced placeholders in command", "input", input)
	return input
}

func ExecAsUser(command string, dryRun bool) error {
	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		log.Error("Could not determine non-root user")
		return fmt.Errorf("could not determine non-root user, ensure you're using sudo")
	}

	wrappedCommand := fmt.Sprintf("sudo -u %s bash -c \"%s\"", targetUser, command)

	if dryRun {
		log.Info("[Dry Run] Command", "command", wrappedCommand)
		fmt.Println("[Dry Run] Command:", wrappedCommand)
		return nil
	}

	log.Info("Executing command as user", "command", wrappedCommand)
	cmd := exec.Command("bash", "-c", wrappedCommand)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Error("Failed to execute command as user", "command", wrappedCommand, "error", err)
	}
	return err
}

func UpdateShellConfig(commands []string) error {
	usr, err := user.Current()
	if err != nil {
		log.Error("Failed to get current user", "error", err)
		return fmt.Errorf("failed to get current user: %v", err)
	}

	shellFile := filepath.Join(usr.HomeDir, ".bashrc") // Default to .bashrc
	if strings.Contains(os.Getenv("SHELL"), "zsh") {
		shellFile = filepath.Join(usr.HomeDir, ".zshrc")
	}

	file, err := os.OpenFile(shellFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Error("Failed to open shell config file", "file", shellFile, "error", err)
		return fmt.Errorf("failed to open shell config file: %v", err)
	}
	defer file.Close()

	for _, command := range commands {
		_, err := file.WriteString(command + "\n")
		if err != nil {
			log.Error("Failed to write to shell config file", "file", shellFile, "command", command, "error", err)
			return fmt.Errorf("failed to write to shell config file: %v", err)
		}
	}

	log.Info("Updated shell config file successfully", "file", shellFile)
	return nil
}
