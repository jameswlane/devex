package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleCompose handles Docker Compose operations
func (d *DockerInstaller) handleCompose(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no compose command specified")
	}

	// Check if docker compose or docker-compose is available
	composeCmd := "docker"
	composeArgs := append([]string{"compose"}, args...)

	if !sdk.CommandExists("docker") {
		return fmt.Errorf("docker is not installed")
	}

	// Test if docker compose is available, fallback to docker-compose
	if err := sdk.ExecCommandWithContext(ctx, false, "docker", "compose", "version"); err != nil {
		if sdk.CommandExists("docker-compose") {
			composeCmd = "docker-compose"
			composeArgs = args
		} else {
			return fmt.Errorf("docker compose is not available")
		}
	}

	return sdk.ExecCommandWithContext(ctx, false, composeCmd, composeArgs...)
}
