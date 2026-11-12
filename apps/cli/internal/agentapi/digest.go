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

// ExecutionPolicy is the effective configuration that changes what an
// install does besides the applications themselves.
type ExecutionPolicy struct {
	RollbackOnFailure  bool   `json:"rollback_on_failure"`
	InstallConcurrency int    `json:"install_concurrency"`
	Offline            bool   `json:"offline"`
	NoCache            bool   `json:"no_cache"`
	SkipPluginDownload bool   `json:"skip_plugin_download"`
	AptListsMaxAge     string `json:"apt_lists_max_age"`
}

// ExecutionSnapshot is a fully resolved install: the selected root
// applications whose lifecycles run, the settings they run with, the plan
// shown for them and the execution policy. Digest covers all of it, so a
// confirmed digest authorizes exactly this execution and nothing else.
type ExecutionSnapshot struct {
	Roots    []types.CrossPlatformApp
	Settings config.CrossPlatformSettings
	Plan     Plan
	Policy   ExecutionPolicy
	Digest   string
}

// RootNames returns the names of the selected root applications.
func (e ExecutionSnapshot) RootNames() []string {
	names := make([]string, 0, len(e.Roots))
	for _, app := range e.Roots {
		names = append(names, app.Name)
	}
	return names
}

// ResolveExecution resolves an install request once into the snapshot that
// both its preview digest and its execution use. The digest binds the
// selected roots (with their complete definitions, in execution order), the
// plan's order, methods, effects and warnings, the complete definition of
// every planned application including catalog dependencies (install
// commands, hooks, alternatives, files), and the execution policy. Any
// change to them changes the digest.
func ResolveExecution(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest, policy ExecutionPolicy) (ExecutionSnapshot, error) {
	plan, err := PlanInstall(ctx, s, r)
	if err != nil {
		return ExecutionSnapshot{Plan: plan}, err
	}
	roots, merged, err := SelectApplications(ctx, s, r)
	if err != nil {
		return ExecutionSnapshot{Plan: plan}, APIError(err)
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
		Roots       []types.CrossPlatformApp
		Plan        Plan
		Definitions []types.CrossPlatformApp
		Policy      ExecutionPolicy
	}{roots, plan, definitions, policy})
	if err != nil {
		return ExecutionSnapshot{Plan: plan}, APIError(fmt.Errorf("encode the plan digest: %w", err))
	}
	sum := sha256.Sum256(data)
	return ExecutionSnapshot{Roots: roots, Settings: merged, Plan: plan, Policy: policy, Digest: "sha256:" + hex.EncodeToString(sum[:])}, nil
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
