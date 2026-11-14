package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

func (p *RpmPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}
	if !p.IsAvailable() {
		return fmt.Errorf("rpm is not available on this system; this plugin requires an RPM-based distribution")
	}

	var missing []string
	for _, pkg := range args {
		var stderr bytes.Buffer
		err := p.runWithStderr(ctx, "is-installed", io.MultiWriter(os.Stderr, &stderr), "--query", pkg)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if rpmPackageAbsent(pkg, err, stderr.String()) {
				missing = append(missing, pkg)
				continue
			}
			return fmt.Errorf("failed to check installation status of %s: %w", pkg, err)
		}
	}

	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}

func rpmPackageAbsent(pkg string, err error, stderr string) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.Contains(stderr, "package "+pkg+" is not installed")
}
