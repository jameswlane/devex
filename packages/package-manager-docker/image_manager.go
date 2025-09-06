package main

import (
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleBuild builds Docker images
func (d *DockerInstaller) handleBuild(args []string) error {
	if len(args) == 0 {
		args = []string{"."}
	}
	cmdArgs := append([]string{"build"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}

// handlePull pulls Docker images
func (d *DockerInstaller) handlePull(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no image specified")
	}
	cmdArgs := append([]string{"pull"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}

// handlePush pushes Docker images
func (d *DockerInstaller) handlePush(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no image specified")
	}
	cmdArgs := append([]string{"push"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}

// handleImages lists Docker images
func (d *DockerInstaller) handleImages(args []string) error {
	return sdk.ExecCommand(false, "docker", "images")
}

// handleRmi removes Docker images
func (d *DockerInstaller) handleRmi(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no images specified")
	}
	cmdArgs := append([]string{"rmi"}, args...)
	return sdk.ExecCommand(false, "docker", cmdArgs...)
}