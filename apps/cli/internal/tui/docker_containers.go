package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// dockerContainerName returns the --name of a `docker run` install command.
func dockerContainerName(command string) string {
	fields := strings.Fields(command)
	if len(fields) < 2 || fields[0] != "docker" || fields[1] != "run" {
		return ""
	}
	for i, field := range fields {
		if field == "--name" && i+1 < len(fields) {
			return fields[i+1]
		}
		if name, ok := strings.CutPrefix(field, "--name="); ok {
			return name
		}
	}
	return ""
}

// dockerNeedsSudo reports whether the current process cannot reach the Docker
// daemon itself. Right after setup installs Docker and adds the user to the
// docker group, the running session has not picked up the new group yet, so
// containers are created with sudo until the user logs in again.
func dockerNeedsSudo(ctx context.Context) bool {
	return exec.CommandContext(ctx, "docker", "info").Run() != nil //nolint:gosec // fixed argv
}

// dockerContainerExists reports whether a container with name already exists.
func dockerContainerExists(ctx context.Context, name string, sudo bool) bool {
	argv := []string{"docker", "container", "inspect", name}
	if sudo {
		argv = append([]string{"sudo", "-n"}, argv...)
	}
	return exec.CommandContext(ctx, argv[0], argv[1:]...).Run() == nil //nolint:gosec // validated container name, no shell
}

// executeDockerInstall creates a database container. An existing container with
// the same name is left untouched, so rerunning setup never fails on
// "name already in use" or replaces the user's container.
func (si *StreamingInstaller) executeDockerInstall(ctx context.Context, app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	si.sendLog("INFO", fmt.Sprintf("Starting Docker installation for %s", app.Name))
	command := strings.TrimSpace(osConfig.InstallCommand)
	name := dockerContainerName(command)
	if name == "" {
		return si.executeCommandStream(ctx, command)
	}
	sudo := dockerNeedsSudo(ctx)
	if dockerContainerExists(ctx, name, sudo) {
		si.sendLog("INFO", fmt.Sprintf("Container %s already exists; leaving it unchanged", name))
		return nil
	}
	if sudo {
		si.sendLog("INFO", "This session cannot reach the Docker daemon yet; creating the container with sudo. Log out and back in to use docker without sudo.")
		command = "sudo " + command
	}
	return si.executeCommandStream(ctx, command)
}
