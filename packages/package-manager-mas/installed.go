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

func (p *MasPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if err := validateAppIDs(args); err != nil {
		return err
	}
	if !p.IsAvailable() {
		return fmt.Errorf("%s", errMissingMas)
	}

	var missing []string
	for _, id := range args {
		// One query per ID: mas exits 1 for the whole call when any ID is
		// absent, so a batched query cannot say which ones are missing.
		var stderr bytes.Buffer
		err := p.runWithIO(ctx, "is-installed", os.Stdout, io.MultiWriter(os.Stderr, &stderr), "list", id)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if masAppAbsent(id, err, stderr.String()) {
				missing = append(missing, id)
				continue
			}
			return fmt.Errorf("failed to check installation status of App Store app %s: %w", id, err)
		}
	}

	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}

// masAppAbsent recognizes mas's answer for an app ID with no installed app:
// exit status 1 and "No installed apps with ADAM ID <id>" on stderr.
func masAppAbsent(id string, err error, stderr string) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.Contains(stderr, "No installed apps with ADAM ID "+id)
}

func isAuthorization(err error) bool {
	var auth *sdk.AuthorizationError
	return errors.As(err, &auth)
}
