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
					return fmt.Errorf("failed to copy file from %s to %s: %v", source, destination, err)
				}
			}
		}
	}
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

	return input
}

func ExecAsUser(command string, dryRun bool) error {
	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		return fmt.Errorf("could not determine non-root user, ensure you're using sudo")
	}

	wrappedCommand := fmt.Sprintf("sudo -u %s bash -c \"%s\"", targetUser, command)

	if dryRun {
		fmt.Println("[Dry Run] Command:", wrappedCommand)
		return nil
	}

	cmd := exec.Command("bash", "-c", wrappedCommand)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func UpdateShellConfig(commands []string) error {
	usr, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %v", err)
	}

	shellFile := filepath.Join(usr.HomeDir, ".bashrc") // Default to .bashrc
	if strings.Contains(os.Getenv("SHELL"), "zsh") {
		shellFile = filepath.Join(usr.HomeDir, ".zshrc")
	}

	file, err := os.OpenFile(shellFile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open shell config file: %v", err)
	}
	defer file.Close()

	for _, command := range commands {
		_, err := file.WriteString(command + "\n")
		if err != nil {
			return fmt.Errorf("failed to write to shell config file: %v", err)
		}
	}

	return nil
}
