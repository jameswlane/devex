package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleBuild builds Docker images
func (d *DockerInstaller) handleBuild(ctx context.Context, args []string) error {
	if len(args) == 0 {
		args = []string{"."}
	}
	cmdArgs := append([]string{"build"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handlePull pulls Docker images
func (d *DockerInstaller) handlePull(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no image specified")
	}
	cmdArgs := append([]string{"pull"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handlePush pushes Docker images
func (d *DockerInstaller) handlePush(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no image specified")
	}
	cmdArgs := append([]string{"push"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handleImages lists Docker images
func (d *DockerInstaller) handleImages(ctx context.Context, args []string) error {
	return sdk.ExecCommandWithContext(ctx, false, "docker", "images")
}

// handleRmi removes Docker images
func (d *DockerInstaller) handleRmi(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no images specified")
	}
	cmdArgs := append([]string{"rmi"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}
