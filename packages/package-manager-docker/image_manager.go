package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleBuild builds Docker images
func (d *DockerInstaller) handleBuild(ctx context.Context, args []string) error {
	cmdArgs, err := d.BuildDockerBuildArgs(args)
	if err != nil {
		return err
	}
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

func (d *DockerInstaller) BuildDockerBuildArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		args = []string{"."}
	}

	buildArgs := []string{"build"}
	contexts := 0
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
		case "--tag", "-t":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateImageName(v); err != nil {
				return nil, err
			}
			buildArgs = append(buildArgs, "-t", v)
		case "--file", "-f":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateDockerBuildPath(v); err != nil {
				return nil, err
			}
			buildArgs = append(buildArgs, "-f", v)
		case "--build-arg":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateEnvVar(v); err != nil {
				return nil, err
			}
			buildArgs = append(buildArgs, "--build-arg", v)
		case "--target":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateCommandArg(v); err != nil {
				return nil, err
			}
			buildArgs = append(buildArgs, "--target", v)
		case "--no-cache", "--pull":
			if hasValue {
				return nil, fmt.Errorf("flag %s does not accept a value", name)
			}
			buildArgs = append(buildArgs, name)
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return nil, fmt.Errorf("unsupported docker build flag: %s", name)
			}
			if err := d.ValidateDockerBuildPath(arg); err != nil {
				return nil, err
			}
			contexts++
			if contexts > 1 {
				return nil, fmt.Errorf("docker build accepts only one build context")
			}
			buildArgs = append(buildArgs, arg)
		}
	}
	if contexts == 0 {
		return nil, fmt.Errorf("docker build context is required")
	}
	return buildArgs, nil
}

// handlePull pulls Docker images
func (d *DockerInstaller) handlePull(ctx context.Context, args []string) error {
	cmdArgs, err := d.BuildDockerPullArgs(args)
	if err != nil {
		return err
	}
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

func (d *DockerInstaller) BuildDockerPullArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no image specified")
	}
	for _, image := range args {
		if err := d.ValidateImageName(image); err != nil {
			return nil, err
		}
	}
	return append([]string{"pull"}, args...), nil
}

// handlePush pushes Docker images
func (d *DockerInstaller) handlePush(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no image specified")
	}
	for _, image := range args {
		if err := d.ValidateImageName(image); err != nil {
			return err
		}
	}
	cmdArgs := append([]string{"push"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

// handleImages lists Docker images
func (d *DockerInstaller) handleImages(ctx context.Context, args []string) error {
	cmdArgs, err := d.BuildDockerImagesArgs(args)
	if err != nil {
		return err
	}
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}

func (d *DockerInstaller) BuildDockerImagesArgs(args []string) ([]string, error) {
	imageArgs := []string{"images"}
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
		case "--filter", "-f":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateDockerFilter(v); err != nil {
				return nil, err
			}
			imageArgs = append(imageArgs, "-f", v)
		case "--format":
			v, err := takeValue()
			if err != nil {
				return nil, err
			}
			if err := d.ValidateDockerFormat(v); err != nil {
				return nil, err
			}
			imageArgs = append(imageArgs, "--format", v)
		case "--all", "-a", "--digests", "--no-trunc", "--quiet", "-q":
			if hasValue {
				return nil, fmt.Errorf("flag %s does not accept a value", name)
			}
			imageArgs = append(imageArgs, name)
		default:
			if len(arg) > 0 && arg[0] == '-' {
				return nil, fmt.Errorf("unsupported docker images flag: %s", name)
			}
			if err := d.ValidateImageReferencePattern(arg); err != nil {
				return nil, err
			}
			imageArgs = append(imageArgs, arg)
		}
	}
	return imageArgs, nil
}

// handleRmi removes Docker images
func (d *DockerInstaller) handleRmi(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no images specified")
	}
	for _, image := range args {
		if err := d.ValidateImageName(image); err != nil {
			return err
		}
	}
	cmdArgs := append([]string{"rmi"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "docker", cmdArgs...)
}
