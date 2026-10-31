package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func validateCurlpipeRemoval(app types.AppConfig) error {
	if app.InstallMethod == "curlpipe" && strings.TrimSpace(app.UninstallCommand) == "" {
		return &sdk.ManualRemovalRequiredError{Applications: []string{app.Name}}
	}
	return nil
}

func uninstallConfiguredApplication(ctx context.Context, app types.AppConfig, installer types.BaseInstaller, repo types.Repository) error {
	if err := validateCurlpipeRemoval(app); err != nil {
		return err
	}
	if app.InstallMethod == "curlpipe" {
		// Catalog uninstall commands are shell programs, not package names. Keep
		// quoting, flags and environment expansion intact, without adding sudo.
		cmd := exec.CommandContext(ctx, "sh", "-c", app.UninstallCommand)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("uninstall command for %s failed: %w", app.Name, err)
		}
		return nil
	}
	command := strings.TrimSpace(app.UninstallCommand)
	if command == "" {
		command = strings.TrimSpace(app.InstallCommand)
	}
	return installer.Uninstall(command, repo)
}
