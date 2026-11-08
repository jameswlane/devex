package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var rpmPackageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.:-]*$`)

func (p *DnfPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	// Validate the whole request before executing any query.
	for _, pkg := range args {
		if !rpmPackageName.MatchString(pkg) {
			return fmt.Errorf("invalid RPM package name %q", pkg)
		}
	}
	var missing []string
	for _, pkg := range args {
		_, err := sdk.ExecCommandOutputWithContext(ctx, "rpm", "-q", "--", pkg)
		if err == nil {
			continue
		}
		var exitErr *exec.ExitError
		// RPM uses status 1 for both missing packages and some database
		// failures. Diagnostic stderr means the query failed, not absence.
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && len(exitErr.Stderr) == 0 {
			missing = append(missing, pkg)
			continue
		}
		return fmt.Errorf("RPM query failed for %q: %w", pkg, err)
	}
	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}
