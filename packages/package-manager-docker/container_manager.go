package main

import (
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleInstall handles container installation/running
func (d *DockerInstaller) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no image specified")
	}

	image := args[0]
	d.logger.Printf("Running container from image: %s\n", image)

	// Build docker run command
	runCmd := []string{"run", "-d"}
	
	// Add image
	runCmd = append(runCmd, image)

	if err := sdk.ExecCommand(false, "docker", runCmd...); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	d.logger.Success("Container started successfully")
	return nil
}

// handleRemove removes containers
func (d *DockerInstaller) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}

	d.logger.Printf("Removing containers: %s\n", strings.Join(args, ", "))

	for _, container := range args {
		// Stop container first
		if err := sdk.ExecCommand(false, "docker", "stop", container); err != nil {
			d.logger.Warning("Failed to stop container %s: %v", container, err)
		}

		// Remove container
		if err := sdk.ExecCommand(false, "docker", "rm", container); err != nil {
			return fmt.Errorf("failed to remove container %s: %w", container, err)
		}
	}

	d.logger.Success("Containers removed successfully")
	return nil
}

// handleStart starts stopped containers
func (d *DockerInstaller) handleStart(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}
	cmdArgs := append([]string{"start"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}

// handleStop stops running containers
func (d *DockerInstaller) handleStop(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}
	cmdArgs := append([]string{"stop"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}

// handleRestart restarts containers
func (d *DockerInstaller) handleRestart(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}
	cmdArgs := append([]string{"restart"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}

// handleList lists containers
func (d *DockerInstaller) handleList(args []string) error {
	listArgs := []string{"ps"}
	for _, arg := range args {
		if arg == "--all" || arg == "-a" {
			listArgs = append(listArgs, "-a")
			break
		}
	}
	return sdk.ExecCommand(false, "docker", listArgs...)
}

// handleLogs shows container logs
func (d *DockerInstaller) handleLogs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no container specified")
	}
	cmdArgs := append([]string{"logs"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}

// handleExec executes commands in containers
func (d *DockerInstaller) handleExec(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("exec requires container and command")
	}
	cmdArgs := append([]string{"exec", "-it"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}