package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

type DockerRunOptions struct {
	Image      string
	Name       string
	Ports      []string
	Env        []string
	Volumes    []string
	Restart    string
	Detach     bool
	Command    []string
	ExtraFlags []string
}

// BuildDockerRunArgs converts typed template/plugin args into a safe docker run invocation.
func (d *DockerInstaller) BuildDockerRunArgs(args []string) ([]string, error) {
	options, err := d.parseDockerRunOptions(args)
	if err != nil {
		return nil, err
	}

	runArgs := []string{"run"}
	if options.Detach {
		runArgs = append(runArgs, "-d")
	}
	if options.Name != "" {
		runArgs = append(runArgs, "--name", options.Name)
	}
	for _, port := range options.Ports {
		runArgs = append(runArgs, "-p", port)
	}
	for _, env := range options.Env {
		runArgs = append(runArgs, "-e", env)
	}
	for _, volume := range options.Volumes {
		runArgs = append(runArgs, "-v", volume)
	}
	if options.Restart != "" {
		runArgs = append(runArgs, "--restart", options.Restart)
	}
	runArgs = append(runArgs, options.ExtraFlags...)
	runArgs = append(runArgs, options.Image)
	runArgs = append(runArgs, options.Command...)

	return runArgs, nil
}

// handleInstall handles container installation/running
func (d *DockerInstaller) handleInstall(ctx context.Context, args []string) error {
	runCmd, err := d.BuildDockerRunArgs(args)
	if err != nil {
		return err
	}

	d.logger.Printf("Running container from image: %s\n", runCmd[len(runCmd)-1])
	if err := sdk.ExecCommandWithContext(ctx, false, "docker", runCmd...); err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	d.logger.Success("Container started successfully")
	return nil
}

// handleRemove removes containers
func (d *DockerInstaller) handleRemove(ctx context.Context, args []string) error {
	removeArgs, containers, err := d.BuildDockerRemoveArgs(args)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return fmt.Errorf("no containers specified")
	}

	d.logger.Printf("Removing containers: %s\n", strings.Join(containers, ", "))

	for _, container := range containers {
		// Stop container first
		if err := sdk.ExecCommandWithContext(ctx, false, "docker", "stop", container); err != nil {
			d.logger.Warning("Failed to stop container %s: %v", container, err)
		}
	}
	if err := sdk.ExecCommandWithContext(ctx, false, "docker", removeArgs...); err != nil {
		return fmt.Errorf("failed to remove containers: %w", err)
	}

	d.logger.Success("Containers removed successfully")
	return nil
}

// handleStart starts stopped containers
func (d *DockerInstaller) handleStart(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}
	if err := d.ValidateContainerArgs(args); err != nil {
		return err
	}
	cmdArgs := append([]string{"start"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handleStop stops running containers
func (d *DockerInstaller) handleStop(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}
	if err := d.ValidateContainerArgs(args); err != nil {
		return err
	}
	cmdArgs := append([]string{"stop"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handleRestart restarts containers
func (d *DockerInstaller) handleRestart(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}
	if err := d.ValidateContainerArgs(args); err != nil {
		return err
	}
	cmdArgs := append([]string{"restart"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handleList lists containers
func (d *DockerInstaller) handleList(ctx context.Context, args []string) error {
	listArgs := []string{"ps"}
	for _, arg := range args {
		if arg == "--all" || arg == "-a" {
			listArgs = append(listArgs, "-a")
			break
		}
	}
	return sdk.ExecCommandWithContext(ctx, false, "docker", listArgs...)
}

// handleLogs shows container logs
func (d *DockerInstaller) handleLogs(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no container specified")
	}
	if err := d.ValidateContainerName(args[0]); err != nil {
		return err
	}
	cmdArgs := append([]string{"logs"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handleExec executes commands in containers
func (d *DockerInstaller) handleExec(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("exec requires container and command")
	}
	if err := d.ValidateContainerName(args[0]); err != nil {
		return err
	}
	cmdArgs := append([]string{"exec", "-it"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

func (d *DockerInstaller) parseDockerRunOptions(args []string) (DockerRunOptions, error) {
	if len(args) == 0 {
		return DockerRunOptions{}, fmt.Errorf("no image specified")
	}

	options := DockerRunOptions{Detach: true}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			options.Command = append(options.Command, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			if options.Image == "" {
				if err := d.ValidateImageName(arg); err != nil {
					return DockerRunOptions{}, err
				}
				options.Image = arg
				continue
			}
			options.Command = append(options.Command, arg)
			continue
		}

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
		case "--name":
			v, err := takeValue()
			if err != nil {
				return DockerRunOptions{}, err
			}
			if err := d.ValidateContainerName(v); err != nil {
				return DockerRunOptions{}, err
			}
			options.Name = v
		case "--port", "-p":
			v, err := takeValue()
			if err != nil {
				return DockerRunOptions{}, err
			}
			if err := d.ValidatePortMapping(v); err != nil {
				return DockerRunOptions{}, err
			}
			options.Ports = append(options.Ports, v)
		case "--env", "-e":
			v, err := takeValue()
			if err != nil {
				return DockerRunOptions{}, err
			}
			if err := d.ValidateEnvVar(v); err != nil {
				return DockerRunOptions{}, err
			}
			options.Env = append(options.Env, v)
		case "--volume", "-v":
			v, err := takeValue()
			if err != nil {
				return DockerRunOptions{}, err
			}
			if err := d.ValidateVolumeMount(v); err != nil {
				return DockerRunOptions{}, err
			}
			options.Volumes = append(options.Volumes, v)
		case "--restart":
			v, err := takeValue()
			if err != nil {
				return DockerRunOptions{}, err
			}
			if err := d.ValidateRestartPolicy(v); err != nil {
				return DockerRunOptions{}, err
			}
			options.Restart = v
		case "--detach", "-d":
			options.Detach = !hasValue || value != "false"
		default:
			return DockerRunOptions{}, fmt.Errorf("unsupported docker install flag: %s", name)
		}
	}
	if options.Image == "" {
		return DockerRunOptions{}, fmt.Errorf("no image specified")
	}
	if err := d.ValidateCommand(options.Command); err != nil {
		return DockerRunOptions{}, err
	}
	return options, nil
}

func (d *DockerInstaller) BuildDockerRemoveArgs(args []string) ([]string, []string, error) {
	removeArgs := []string{"rm"}
	containers := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--force", "-f":
			removeArgs = append(removeArgs, "-f")
		case "--volumes", "-v":
			removeArgs = append(removeArgs, "-v")
		default:
			if strings.HasPrefix(arg, "-") {
				return nil, nil, fmt.Errorf("unsupported docker remove flag: %s", arg)
			}
			if err := d.ValidateContainerName(arg); err != nil {
				return nil, nil, err
			}
			containers = append(containers, arg)
		}
	}
	removeArgs = append(removeArgs, containers...)
	return removeArgs, containers, nil
}

func splitDockerFlag(arg string) (name string, value string, hasValue bool) {
	if before, after, ok := strings.Cut(arg, "="); ok {
		return before, after, true
	}
	return arg, "", false
}
