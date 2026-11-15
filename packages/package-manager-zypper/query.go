package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// exitNotFound is ZYPPER_EXIT_INF_CAP_NOT_FOUND, which search returns when
// nothing matches.
const exitNotFound = 104

// handleIsInstalled checks each package or pattern with an exact,
// installed-only search of the local system. It reads no repository metadata,
// so it runs as the invoking user and needs no network access. Zypper exits
// 104 when nothing matches; any other failure is a failed query, not absence.
func (p *ZypperPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	sel, err := parseSelection(args)
	if err != nil {
		return err
	}
	for _, name := range sel.names {
		if !exactName.MatchString(name) {
			return fmt.Errorf("invalid %s name: %q", sel.kind, name)
		}
	}
	if !p.IsAvailable() {
		return fmt.Errorf("zypper is not available on this system; this plugin requires openSUSE or SUSE Linux")
	}
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{"is-installed"})
	var missing []string
	for _, name := range sel.names {
		var stderr bytes.Buffer
		err := sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: *privilege, Name: "zypper", Args: isInstalledArgs(sel.kind, name)},
			sdk.OperationIO{Stdout: io.Discard, Stderr: &stderr})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			continue
		}
		var auth *sdk.AuthorizationError
		if !errors.As(err, &auth) && sdk.ExitStatus(err) == exitNotFound {
			missing = append(missing, name)
			continue
		}
		if diagnostic := strings.TrimSpace(stderr.String()); diagnostic != "" {
			return fmt.Errorf("zypper query failed for %s %q: %s: %w", sel.kind, name, diagnostic, err)
		}
		return fmt.Errorf("zypper query failed for %s %q: %w", sel.kind, name, err)
	}
	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}

func isInstalledArgs(kind, name string) []string {
	return []string{"--non-interactive", "--quiet", "--no-refresh", "--disable-repositories",
		"search", "--installed-only", "--match-exact", "--type", kind, "--", name}
}
