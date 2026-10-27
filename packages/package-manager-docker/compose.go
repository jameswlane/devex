package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// BuildDockerComposeArgs normalizes template-friendly compose flags into Docker CLI order.
func (d *DockerInstaller) BuildDockerComposeArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no compose command specified")
	}

	globalArgs := []string{"compose"}
	commandArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value, hasValue := splitDockerFlag(arg)
		takeValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			i++
			if i >= len(args) {
				return "", fmt.Errorf("flag %s requires a value", name)
			}
			return args[i], nil
		}

		switch name {
		case "--file", "-f":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateComposeFilePath(v); err != nil {
				return nil, err
			}
			globalArgs = append(globalArgs, "-f", v)
		case "--project", "--project-name", "-p":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateContainerName(v); err != nil {
				return nil, err
			}
			globalArgs = append(globalArgs, "-p", v)
		case "--detach":
			commandArgs = append(commandArgs, "-d")
		default:
			if strings.HasPrefix(arg, "-") {
				commandArgs = append(commandArgs, arg)
				continue
			}
			if err := d.ValidateCommandArg(arg); err != nil {
				return nil, err
			}
			commandArgs = append(commandArgs, arg)
		}
	}
	if len(commandArgs) == 0 {
		return nil, fmt.Errorf("no compose command specified")
	}
	return append(globalArgs, commandArgs...), nil
}

// handleCompose handles Docker Compose operations
func (d *DockerInstaller) handleCompose(ctx context.Context, args []string) error {
	composeArgs, err := d.BuildDockerComposeArgs(args)
	if err != nil {
		return err
	}

	// Check if docker compose or docker-compose is available
	composeCmd := "docker"
	fallbackArgs := composeArgs[1:]

	if !sdk.CommandExists("docker") {
		return fmt.Errorf("docker is not installed")
	}

	// Test if docker compose is available, fallback to docker-compose
	if err := sdk.ExecCommandWithContext(ctx, false, "docker", "compose", "version"); err != nil {
		if sdk.CommandExists("docker-compose") {
			composeCmd = "docker-compose"
			composeArgs = fallbackArgs
		} else {
			return fmt.Errorf("docker compose is not available")
		}
	}

	return sdk.ExecCommandWithContext(ctx, false, composeCmd, composeArgs...)
}
