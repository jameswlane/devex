package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var rpmPackageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.:-]*$`)

func (p *YumPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
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

// requireInstalledMatch checks the local RPM database for spec by name, then
// by provided capability, the way "yum remove" resolves it.
func requireInstalledMatch(ctx context.Context, spec string) error {
	if strings.HasPrefix(spec, "-") {
		return fmt.Errorf("cannot remove package %q: package specifications must not start with '-'", spec)
	}
	for _, selector := range [][]string{{"-q", "--", spec}, {"-q", "--whatprovides", "--", spec}} {
		_, err := sdk.ExecCommandOutputWithContext(ctx, "rpm", selector...)
		if err == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return fmt.Errorf("cannot remove package %q: RPM query failed: %w", spec, err)
		}
	}
	return fmt.Errorf("cannot remove package %q: no installed package matches", spec)
}
