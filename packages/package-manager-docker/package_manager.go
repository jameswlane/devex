package main

import (
	"fmt"
	"os/exec"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// DockerInstaller implements Docker container management and Docker Engine installation
type DockerInstaller struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// Execute handles command execution
func (d *DockerInstaller) Execute(command string, args []string) error {
	switch command {
	case "ensure-installed":
		return d.handleEnsureInstalled(args)
	case "status":
		return d.handleStatus(args)
	}

	// For all other commands, ensure Docker is available
	d.EnsureAvailable()

	switch command {
	case "install":
		return d.handleInstall(args)
	case "remove":
		return d.handleRemove(args)
	case "start":
		return d.handleStart(args)
	case "stop":
		return d.handleStop(args)
	case "restart":
		return d.handleRestart(args)
	case "list":
		return d.handleList(args)
	case "logs":
		return d.handleLogs(args)
	case "exec":
		return d.handleExec(args)
	case "build":
		return d.handleBuild(args)
	case "pull":
		return d.handlePull(args)
	case "push":
		return d.handlePush(args)
	case "images":
		return d.handleImages(args)
	case "rmi":
		return d.handleRmi(args)
	case "compose":
		return d.handleCompose(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// isDockerAvailable checks if Docker is available
func (d *DockerInstaller) isDockerAvailable() bool {
	return sdk.CommandExists("docker")
}

// isDockerDaemonRunning checks if Docker daemon is running
func (d *DockerInstaller) isDockerDaemonRunning() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// handleStatus checks Docker daemon status
func (d *DockerInstaller) handleStatus(args []string) error {
	if !d.isDockerAvailable() {
		d.logger.ErrorMsg("Docker is not installed")
		return fmt.Errorf("Docker is not installed")
	}

	if d.isDockerDaemonRunning() {
		d.logger.Success("Docker daemon is running")
		
		// Show Docker version
		if err := sdk.ExecCommand(false, "docker", "version"); err != nil {
			d.logger.Warning("Failed to get Docker version: %v", err)
		}
		return nil
	}

	d.logger.ErrorMsg("Docker daemon is not running")
	return fmt.Errorf("Docker daemon is not running")
}