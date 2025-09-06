package main

import (
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleCompose handles Docker Compose operations
func (d *DockerInstaller) handleCompose(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no compose command specified")
	}

	// Check if docker compose or docker-compose is available
	composeCmd := "docker"
	composeArgs := append([]string{"compose"}, args...)
	
	if !sdk.CommandExists("docker") {
		return fmt.Errorf("Docker is not installed")
	}

	// Test if docker compose is available, fallback to docker-compose
	if err := sdk.ExecCommand(false, "docker", "compose", "version"); err != nil {
		if sdk.CommandExists("docker-compose") {
			composeCmd = "docker-compose"
			composeArgs = args
		} else {
			return fmt.Errorf("Docker Compose is not available")
		}
	}

	return sdk.ExecCommand(false, composeCmd, composeArgs...)
}