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
		// rpm prints "package X is not installed" as a translated notice on
		// stdout, so pin the C locale and inspect stdout to recognise it. A
		// failed database open prints the same notice, plus "error:" on stderr.
		var stdout, stderr bytes.Buffer
		err := p.runWithIO(ctx, "is-installed", io.MultiWriter(os.Stdout, &stdout), io.MultiWriter(os.Stderr, &stderr), []string{"LC_ALL=C"}, "--query", pkg)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if rpmPackageAbsent(pkg, err, stdout.String(), stderr.String()) {
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

func rpmPackageAbsent(pkg string, err error, stdout, stderr string) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.Contains(stdout, "package "+pkg+" is not installed") && !strings.Contains(stderr, "error:")
}
