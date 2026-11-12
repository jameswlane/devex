package agentapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// PlanChangedMessage starts the error an install returns when its confirmed
// plan digest no longer matches. Remote callers recognize it in the output.
const PlanChangedMessage = "the installation plan changed after it was confirmed"

// PlanDigest returns the install plan and a digest that binds it to the full
// definitions it would execute: every planned application's complete
// configuration (install commands, hooks, dependencies, files) along with the
// plan's order, methods, effects and warnings. Any change to them changes the
// digest.
func PlanDigest(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest) (Plan, string, error) {
	plan, err := PlanInstall(ctx, s, r)
	if err != nil {
		return plan, "", err
	}
	_, merged, err := SelectApplications(ctx, s, r)
	if err != nil {
		return plan, "", APIError(err)
	}
	byName := map[string]types.CrossPlatformApp{}
	for _, app := range merged.GetAllApps() {
		byName[normalizeInstallLookup(app.Name)] = app
	}
	definitions := make([]types.CrossPlatformApp, 0, len(plan.DependencyOrder))
	for _, name := range plan.DependencyOrder {
		definitions = append(definitions, byName[normalizeInstallLookup(name)])
	}
	data, err := json.Marshal(struct {
		Plan        Plan
		Definitions []types.CrossPlatformApp
	}{plan, definitions})
	if err != nil {
		return plan, "", APIError(fmt.Errorf("encode the plan digest: %w", err))
	}
	sum := sha256.Sum256(data)
	return plan, "sha256:" + hex.EncodeToString(sum[:]), nil
}

// CheckCatalogNames reports names that match no catalog application on any
// platform. Unlike install selection it does not filter by the local
// platform, so a host can check names for another host's platform.
func CheckCatalogNames(settings config.CrossPlatformSettings, names []string) error {
	known := map[string]bool{}
	for _, app := range settings.GetAllApps() {
		known[normalizeInstallLookup(app.Name)] = true
		for _, c := range []types.OSConfig{app.Linux, app.MacOS, app.Windows, app.AllPlatforms} {
			if c.InstallCommand != "" {
				known[normalizeInstallLookup(c.InstallCommand)] = true
			}
		}
	}
	var unknown []string
	for _, name := range names {
		if !known[normalizeInstallLookup(name)] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("applications not found in the catalog: %s; run 'devex list available' to see application names", strings.Join(unknown, ", "))
	}
	return nil
}
