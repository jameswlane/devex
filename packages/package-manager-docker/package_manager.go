// Package main implements the Docker package manager plugin for the DevEx CLI.
// This plugin provides comprehensive Docker container and image management capabilities
// along with automated Docker Engine installation and configuration.
//
// The plugin supports:
//   - Docker Engine installation on multiple Linux distributions
//   - Container lifecycle management (create, start, stop, remove)
//   - Image management (pull, push, build, remove)
//   - Docker Compose integration
//   - User group management for Docker access
//   - System service management
//
// Security features include input validation, trusted registry enforcement,
// and proper error handling with context cancellation support.
package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// DockerInstaller implements Docker container management and Docker Engine installation
type DockerInstaller struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// Execute handles command execution
func (d *DockerInstaller) Execute(command string, args []string) error {
	ctx := context.Background()

	switch command {
	case "ensure-installed":
		return d.handleEnsureInstalled(ctx, args)
	case "status":
		return d.handleStatus(ctx, args)
	}

	// For all other commands, ensure Docker is available
	d.EnsureAvailable()

	switch command {
	case "install":
		return d.handleInstall(ctx, args)
	case "remove":
		return d.handleRemove(ctx, args)
	case "start":
		return d.handleStart(ctx, args)
	case "stop":
		return d.handleStop(ctx, args)
	case "restart":
		return d.handleRestart(ctx, args)
	case "list":
		return d.handleList(ctx, args)
	case "logs":
		return d.handleLogs(ctx, args)
	case "exec":
		return d.handleExec(ctx, args)
	case "build":
		return d.handleBuild(ctx, args)
	case "pull":
		return d.handlePull(ctx, args)
	case "push":
		return d.handlePush(ctx, args)
	case "images":
		return d.handleImages(ctx, args)
	case "rmi":
		return d.handleRmi(ctx, args)
	case "compose":
		return d.handleCompose(ctx, args)
	default:
		return fmt.Errorf("unknown command: '%s'", command)
	}
}

// isDockerAvailable checks if Docker is available
func (d *DockerInstaller) isDockerAvailable() bool {
	return sdk.CommandExists("docker")
}

// isDockerDaemonRunning checks if Docker daemon is running
func (d *DockerInstaller) isDockerDaemonRunning() bool {
	err := d.ExecManagerCommand("search", false, "info")
	return err == nil
}

// handleStatus checks Docker daemon status
func (d *DockerInstaller) handleStatus(ctx context.Context, args []string) error {
	if !d.isDockerAvailable() {
		d.logger.ErrorMsg("Docker is not installed")
		return fmt.Errorf("docker is not installed on this system")
	}

	if d.isDockerDaemonRunning() {
		d.logger.Success("Docker daemon is running")

		// Show Docker version
		if err := d.ExecManagerCommand("search", false, "version"); err != nil {
			d.logger.Warning("Failed to get Docker version: %v", err)
		}
		return nil
	}

	d.logger.ErrorMsg("Docker daemon is not running")
	return fmt.Errorf("docker daemon is not running")
}
