package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleIsInstalled queries each identifier separately: winget list --id
// --exact exits with NO_APPLICATIONS_FOUND for a package that is absent.
func (p *WingetPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	req, err := parseRequest(args)
	if err != nil {
		return err
	}
	if !p.IsAvailable() {
		return fmt.Errorf("%s", errMissingWinget)
	}

	var missing []string
	for _, id := range req.ids {
		argv := append([]string{"list", "--id", id, "--exact"}, req.scopeArgs()...)
		argv = append(argv, sourceAgreement...)
		// A scope only filters the query, so it always runs at user scope.
		err := p.run(ctx, sdk.ScopeUser, argv...)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			continue
		}
		if code, ok := hresult(err); ok && code == hrNoApplicationsFound {
			missing = append(missing, id)
			continue
		}
		return fmt.Errorf("failed to check installation status of winget package %s: %w", id, err)
	}

	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}
