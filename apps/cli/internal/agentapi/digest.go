package agentapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/execinputs"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
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

// ExecutionTrust is the effective plugin trust configuration of an install.
// It is bound into the digest but not shown, since it names local paths and URLs.
type ExecutionTrust struct {
	PluginRegistryURL     string
	PluginPublicKey       string
	PluginPublicKeyDigest string
}

// ExecutionSnapshot is a fully resolved install: the selected root
// applications whose lifecycles run, the settings they run with (including
// the resolved command-security policy), the plan shown for them, the
// execution policy, the plugin trust configuration and the local files the
// roots read while installing. Digest covers all of it, so a confirmed digest
// authorizes exactly this execution and nothing else.
type ExecutionSnapshot struct {
	Roots    []types.CrossPlatformApp
	Settings config.CrossPlatformSettings
	Plan     Plan
	Policy   ExecutionPolicy
	Trust    ExecutionTrust
	Security *security.SecurityConfig
	Inputs   execinputs.Inputs
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
// commands, hooks, alternatives, files), the execution policy and plugin
// trust, the effective command-security policy (embedded settings or
// security.yaml), and the content of every config_files and copy-hook source
// plus the presence of every config_files marker the roots read. Any change
// to them changes the digest. Settings.ResolvedSecurity is set so execution
// uses the resolved security policy instead of loading it again.
func ResolveExecution(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest, policy ExecutionPolicy, trust ExecutionTrust) (ExecutionSnapshot, error) {
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
	securityConfig, err := merged.LoadSecurityConfigForSettings()
	if err != nil {
		return ExecutionSnapshot{Plan: plan}, APIError(fmt.Errorf("load the command security policy: %w", err))
	}
	merged.ResolvedSecurity = securityConfig
	inputs, err := rootInputs(roots)
	if err != nil {
		return ExecutionSnapshot{Plan: plan}, APIError(err)
	}
	data, err := json.Marshal(struct {
		Roots       []types.CrossPlatformApp
		Plan        Plan
		Definitions []types.CrossPlatformApp
		Policy      ExecutionPolicy
		Trust       ExecutionTrust
		Security    *security.SecurityConfig
		Inputs      execinputs.Inputs
	}{roots, plan, definitions, policy, trust, securityConfig, inputs})
	if err != nil {
		return ExecutionSnapshot{Plan: plan}, APIError(fmt.Errorf("encode the plan digest: %w", err))
	}
	sum := sha256.Sum256(data)
	return ExecutionSnapshot{Roots: roots, Settings: merged, Plan: plan, Policy: policy, Trust: trust, Security: securityConfig, Inputs: inputs, Digest: "sha256:" + hex.EncodeToString(sum[:])}, nil
}

// rootInputs fingerprints the local files the roots read while installing:
// config_files sources and markers, and copy-hook sources, expanded the same
// way execution expands them, for the selected platform configuration and
// its alternatives.
func rootInputs(roots []types.CrossPlatformApp) (execinputs.Inputs, error) {
	inputs := execinputs.Inputs{Files: map[string]string{}, Markers: map[string]bool{}}
	var identity *sdk.UserIdentity
	for _, app := range roots {
		selected := app.GetOSConfig()
		configs := append([]types.OSConfig{selected}, selected.Alternatives...)
		for _, c := range configs {
			for _, file := range c.ConfigFiles {
				if source, err := configfiles.Expand(file.Source); err == nil {
					inputs.Files[source] = execinputs.Fingerprint(source)
				}
				for _, marker := range file.RequiresAny {
					if path, err := configfiles.Expand(marker); err == nil {
						inputs.Markers[path] = execinputs.MarkerPresent(path)
					}
				}
			}
			for _, hook := range append(append([]types.InstallCommand{}, c.PreInstall...), c.PostInstall...) {
				if hook.Copy == nil {
					continue
				}
				if identity == nil {
					plan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "install-hooks"})
					if err != nil {
						return inputs, fmt.Errorf("resolve the install user for copy hooks: %w", err)
					}
					identity = plan.IntendedUser
				}
				source := utils.ExpandUserPlaceholders(hook.Copy.Source, *identity)
				inputs.Files[source] = execinputs.Fingerprint(source)
			}
		}
	}
	return inputs, nil
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
