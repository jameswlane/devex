package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// dockerSocketPermissionDenied matches the docker CLI error for a user who
// can't open the daemon socket, such as a user added to the docker group in
// this login session.
func dockerSocketPermissionDenied(output string) bool {
	// Older clients say "the Docker daemon socket", newer ones "the docker API".
	return strings.Contains(strings.ToLower(output), "permission denied while trying to connect to the docker")
}

// dockerNeedsSudo reports whether docker commands must run through sudo.
// usermod -aG docker doesn't change the groups of a running session, so on
// the first install the user can't reach the daemon socket until they log in
// again. The plugin then uses the same elevation that ensure-installed used.
func (d *DockerInstaller) dockerNeedsSudo(ctx context.Context) bool {
	if d.dockerSudo != nil {
		return *d.dockerSudo
	}
	if sdk.NewPrivilegeResolver().Elevated {
		needs := false
		d.dockerSudo = &needs
		return needs
	}
	output, err := sdk.NewCommandContext(ctx, "docker", "info").CombinedOutput()
	needs := err != nil && dockerSocketPermissionDenied(string(output))
	if err != nil && !needs {
		// Docker isn't installed or its daemon isn't up yet; probe again on
		// the next command instead of caching the answer.
		return false
	}
	if needs {
		d.logger.Warning("This session can't reach the Docker socket yet, so DevEx runs docker with sudo. Log out and back in to use docker without sudo.")
	}
	d.dockerSudo = &needs
	return needs
}

// runDocker runs a docker command, elevated when this session can't reach the
// Docker socket.
func (d *DockerInstaller) runDocker(ctx context.Context, args ...string) error {
	useSudo := d.dockerNeedsSudo(ctx)
	err := sdk.ExecCommandWithContext(ctx, useSudo, "docker", args...)
	if err != nil && useSudo {
		return fmt.Errorf("%w; this session isn't in the docker group yet, so log out and back in, then run the same command again", err)
	}
	return err
}
