package agentapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Error carries stable machine semantics without leaking underlying config values.
type Error struct {
	Category  string `json:"category"`
	Retryable bool   `json:"retryable"`
	Message   string `json:"message"`
	cause     error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.cause }
func APIError(err error) *Error {
	if err == nil {
		return nil
	}
	var api *Error
	if errors.As(err, &api) {
		return api
	}
	category := "invalid_request"
	if errors.Is(err, context.Canceled) {
		category = "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		category = "deadline_exceeded"
	}
	return &Error{Category: category, Message: category, cause: err}
}

type PlanRequest struct {
	Names               []string
	Categories          []string
	Channel             string
	RecordedChannel     func(types.CrossPlatformApp) string
	Repo                types.Repository
	RefuseChannelSwitch bool
	// SkipChannels returns the selected applications with their default
	// channels, for callers such as maintenance that resolve each app's
	// channel themselves.
	SkipChannels bool
}

// PlanChannel describes one available install channel for an application on the
// current platform.
type PlanChannel struct {
	Name     string `json:"name" yaml:"name"`
	Official bool   `json:"official" yaml:"official"`
	Default  bool   `json:"default,omitempty" yaml:"default,omitempty"`
}

func availablePlanChannels(app types.CrossPlatformApp) []PlanChannel {
	all := app.Channels()
	if len(all) == 0 {
		return nil
	}
	out := make([]PlanChannel, 0, len(all))
	for _, ch := range all {
		if !ch.Available {
			continue
		}
		out = append(out, PlanChannel{
			Name:     ch.Name,
			Official: ch.Official,
			Default:  ch.Default,
		})
	}
	return out
}

func planChannelPreferences(r PlanRequest) channels.Preferences {
	prefs := channels.PreferencesFromViper()
	if ch := strings.TrimSpace(r.Channel); ch != "" {
		prefs.Channel = ch
	}
	return prefs
}

type Plan struct {
	Apps               []PlannedApp `json:"apps"`
	DependencyOrder    []string     `json:"dependency_order"`
	Warnings           []string     `json:"warnings"`
	OrganizationPolicy *PolicyRef   `json:"organization_policy,omitempty"`
}

// PolicyRef identifies the exact organization policy file used for a plan.
type PolicyRef struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version"`
	Source  string `json:"source"`
	SHA256  string `json:"sha256"`
	// ApprovalRequired means the plan installs only with a signed approval
	// of its digest (install --expect-plan <digest> --approval <file>).
	ApprovalRequired bool `json:"approval_required,omitempty"`
}
type PlannedApp struct {
	Name           string        `json:"name"`
	Method         string        `json:"method"`
	Channel        string        `json:"channel,omitempty"`
	Channels       []PlanChannel `json:"channels,omitempty"`
	Plugin         string        `json:"plugin"`
	Dependencies   []string      `json:"dependencies"`
	Effects        []string      `json:"effects"`
	Privileges     string        `json:"privileges"`
	UnknownEffects bool          `json:"unknown_effects"`
}

// SelectionResult holds the applications selected for installation together
// with the merged settings and any skip notices emitted during filtering.
type SelectionResult struct {
	Apps     []types.CrossPlatformApp
	Settings config.CrossPlatformSettings
	Warnings []string
}

// HeadlessSkipMessage formats the user-facing skip notice emitted when an
// application requiring a graphical desktop is skipped on a headless Linux host.
func HeadlessSkipMessage(appName string) string {
	return fmt.Sprintf("Skipped %s: needs a graphical desktop, and none was found (no DISPLAY/WAYLAND_DISPLAY session and no installed X11/Wayland sessions)", appName)
}

func filterHeadlessApplications(apps []types.CrossPlatformApp) ([]types.CrossPlatformApp, []string) {
	if types.CatalogOS() != "linux" || !platform.IsHeadless() {
		return apps, nil
	}
	filtered := make([]types.CrossPlatformApp, 0, len(apps))
	var warnings []string
	for _, app := range apps {
		if app.RequiresGraphicalDesktop() {
			warnings = append(warnings, HeadlessSkipMessage(app.Name))
			continue
		}
		filtered = append(filtered, app)
	}
	return filtered, warnings
}

// SelectApplicationsWithWarnings is the shared CLI install execution and
// planning selection policy: the requested applications filtered for the
// current OS and desktop environment, plus the sandbox prerequisites declared
// by the remaining applications.
func SelectApplicationsWithWarnings(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest) (SelectionResult, error) {
	apps, s, osWarnings, err := selectRequestedApplications(ctx, s, r, true)
	if err != nil {
		return SelectionResult{Settings: s}, err
	}
	filtered, headlessWarnings := filterHeadlessApplications(apps)
	// A graphical sandbox prerequisite is skipped on headless hosts too.
	withPrerequisites, err := expandSandboxPrerequisites(filtered, s, planChannelPreferences(r).WithoutExplicitChannel(), r.Repo)
	if err != nil {
		return SelectionResult{Settings: s}, &Error{Category: "invalid_request", Message: err.Error(), cause: err}
	}
	expanded, prereqWarnings := filterHeadlessApplications(withPrerequisites)
	if r.RefuseChannelSwitch && r.Repo != nil {
		if switchErr := channels.CheckRefusedChannelSwitches(r.Repo, expanded); switchErr != nil {
			return SelectionResult{Settings: s}, &Error{Category: "invalid_request", Message: switchErr.Error(), cause: switchErr}
		}
	}
	warnings := make([]string, 0, len(osWarnings)+len(headlessWarnings)+len(prereqWarnings))
	warnings = append(warnings, osWarnings...)
	warnings = append(warnings, headlessWarnings...)
	warnings = append(warnings, prereqWarnings...)
	return SelectionResult{
		Apps:     expanded,
		Settings: s,
		Warnings: warnings,
	}, nil
}

// SelectApplications is the shared CLI install execution and planning
// selection policy: the requested applications plus the sandbox prerequisites
// they declare for the current platform.
func SelectApplications(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest) ([]types.CrossPlatformApp, config.CrossPlatformSettings, error) {
	result, err := SelectApplicationsWithWarnings(ctx, s, r)
	return result.Apps, result.Settings, err
}

// SelectRequestedApplications resolves only the requested applications, for
// operations such as update and upgrade that must not add install prerequisites.
func SelectRequestedApplications(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest) ([]types.CrossPlatformApp, config.CrossPlatformSettings, error) {
	apps, merged, _, err := selectRequestedApplications(ctx, s, r, false)
	return apps, merged, err
}

func selectRequestedApplications(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest, skipUnsupportedOS bool) ([]types.CrossPlatformApp, config.CrossPlatformSettings, []string, error) {
	if err := ctx.Err(); err != nil {
		return nil, s, nil, err
	}
	if len(r.Names) > 0 && len(r.Categories) > 0 {
		return nil, s, nil, &Error{Category: "invalid_request", Message: "cannot combine applications and categories"}
	}
	configured, merged, present, err := ConfiguredApplications(ctx, s)
	if err != nil {
		return nil, s, nil, err
	}
	s = merged
	var apps []types.CrossPlatformApp
	var warnings []string
	switch {
	case len(r.Names) > 0:
		apps, warnings, err = resolveAppsByName(s, r.Names, skipUnsupportedOS)
	case len(r.Categories) > 0:
		apps, err = ResolveAppsByCategory(s, r.Categories)
	case present:
		apps = configured
	default:
		apps = s.GetDefaultApps()
	}
	if err != nil {
		return nil, s, nil, err
	}
	if len(apps) == 0 && len(warnings) == 0 && !present {
		return nil, s, nil, &Error{Category: "not_found", Message: "no applications matched the install request"}
	}
	if r.SkipChannels {
		return apps, s, warnings, nil
	}
	var resolved []types.CrossPlatformApp
	if r.RecordedChannel != nil {
		resolved, err = channels.ResolveAll(apps, planChannelPreferences(r), r.RecordedChannel)
	} else {
		var installedLookup func(types.CrossPlatformApp) string
		if r.Repo != nil {
			installedLookup = func(app types.CrossPlatformApp) string {
				return channels.InstalledChannelForApp(r.Repo, app)
			}
		}
		resolved, err = channels.ResolveAllForInstall(apps, planChannelPreferences(r), installedLookup)
	}
	if err != nil {
		return nil, s, nil, &Error{Category: "invalid_request", Message: err.Error(), cause: err}
	}
	return resolved, s, warnings, nil
}

// expandSandboxPrerequisites puts the current-platform sandbox_prerequisites
// of the selected apps first, in declaration order and deduplicated, followed by
// the remaining apps in their selected order. Prerequisites are filtered to
// catalog applications supported on this platform. The order depends only on
// the set of selected apps' declarations, so resolving the expanded names again
// (as MCP confirmation does) yields the same roots in the same order.
func expandSandboxPrerequisites(apps []types.CrossPlatformApp, settings config.CrossPlatformSettings, prereqPrefs channels.Preferences, repo types.Repository) ([]types.CrossPlatformApp, error) {
	if len(apps) == 0 {
		return apps, nil
	}
	catalog := make(map[string]types.CrossPlatformApp)
	for _, app := range settings.GetAllApps() {
		catalog[normalizeInstallLookup(app.Name)] = app
	}
	// A prerequisite the user also asked for keeps the channel chosen for it.
	selected := make(map[string]types.CrossPlatformApp, len(apps))
	for _, app := range apps {
		selected[normalizeInstallLookup(app.Name)] = app
	}
	var prerequisites []types.CrossPlatformApp
	var failures []error
	isPrerequisite := map[string]bool{}
	for _, app := range apps {
		appKey := normalizeInstallLookup(app.Name)
		for _, name := range app.GetOSConfig().SandboxPrerequisites {
			key := normalizeInstallLookup(name)
			if key == "" || key == appKey || isPrerequisite[key] {
				continue
			}
			prerequisite, ok := catalog[key]
			if !ok || !prerequisite.IsSupported() {
				continue
			}
			if chosen, requested := selected[key]; requested {
				prerequisite = chosen
			} else {
				resolved, _, err := channels.ResolveForInstall(prerequisite, prereqPrefs, channels.InstalledChannelForApp(repo, prerequisite))
				if err != nil {
					failures = append(failures, fmt.Errorf("sandbox prerequisite of %s: %w", app.Name, err))
					continue
				}
				prerequisite = resolved
			}
			isPrerequisite[key] = true
			prerequisites = append(prerequisites, prerequisite)
		}
	}
	if len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	if len(prerequisites) == 0 {
		return apps, nil
	}
	expanded := append([]types.CrossPlatformApp(nil), prerequisites...)
	emitted := map[string]bool{}
	for _, app := range apps {
		key := normalizeInstallLookup(app.Name)
		if isPrerequisite[key] || emitted[key] {
			continue
		}
		emitted[key] = true
		expanded = append(expanded, app)
	}
	return expanded, nil
}

// PlanInstall has no installer, executor, filesystem writer, or network dependency.
// Plugin and script effects are deliberately unknown until plugins support describe.
func PlanInstall(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest) (Plan, error) {
	result := Plan{Apps: []PlannedApp{}, DependencyOrder: []string{}, Warnings: []string{}}
	selection, err := SelectApplicationsWithWarnings(ctx, s, r)
	if err != nil {
		return result, APIError(err)
	}
	apps := selection.Apps
	s = selection.Settings
	if len(selection.Warnings) > 0 {
		result.Warnings = append(result.Warnings, selection.Warnings...)
	}
	if err = config.RequireValidConfig(ctx, s); err != nil {
		return result, APIError(err)
	}
	orgPolicy, err := security.LoadOrgPolicy(s.HomeDir)
	if err != nil {
		return result, &Error{Category: "policy_denied", Message: err.Error(), cause: err}
	}
	if orgPolicy != nil {
		result.OrganizationPolicy = &PolicyRef{Name: orgPolicy.Name, Version: orgPolicy.Version, Source: orgPolicy.Source, SHA256: orgPolicy.SHA256, ApprovalRequired: orgPolicy.ApprovalRequired()}
	}
	var violations []string
	byName := map[string]types.CrossPlatformApp{}
	aliases := map[string]string{}
	depPrefs := planChannelPreferences(r).WithoutExplicitChannel()
	// Resolution errors only matter for the dependencies the plan reaches.
	resolveErrs := map[string]error{}
	for _, app := range s.GetAllApps() {
		key := normalizeInstallLookup(app.Name)
		aliases[key] = key
		// A dependency may name the catalog's default command or the
		// selected channel's command; both resolve to the app.
		if command := app.GetOSConfig().InstallCommand; command != "" {
			aliases[normalizeInstallLookup(command)] = key
		}
		recorded := channels.InstalledChannelForApp(r.Repo, app)
		if selected, _, resolveErr := channels.ResolveForInstall(app, depPrefs, recorded); resolveErr == nil {
			app = selected
			if r.RefuseChannelSwitch {
				if switchErr := channels.CheckRefusedChannelSwitch(r.Repo, selected); switchErr != nil {
					resolveErrs[key] = switchErr
				}
			}
		} else {
			resolveErrs[key] = resolveErr
		}
		byName[key] = app
		if command := app.GetOSConfig().InstallCommand; command != "" {
			if _, taken := aliases[normalizeInstallLookup(command)]; !taken {
				aliases[normalizeInstallLookup(command)] = key
			}
		}
	}
	for _, app := range apps {
		byName[normalizeInstallLookup(app.Name)] = app
		delete(resolveErrs, normalizeInstallLookup(app.Name))
	}
	visited := map[string]bool{}
	var visit func(types.CrossPlatformApp) error
	visit = func(app types.CrossPlatformApp) error {
		if err := ctx.Err(); err != nil {
			return APIError(err)
		}
		key := normalizeInstallLookup(app.Name)
		if visited[key] {
			return nil
		}
		if resolveErr := resolveErrs[key]; resolveErr != nil {
			return &Error{Category: "invalid_request", Message: resolveErr.Error(), cause: resolveErr}
		}
		visited[key] = true
		if err := orgPolicy.CheckApp(app); err != nil {
			violations = append(violations, err.Error())
		}
		c := app.GetOSConfig()
		deps := append([]string{}, c.Dependencies...)
		for _, dep := range deps {
			if dependency, ok := byName[aliases[normalizeInstallLookup(dep)]]; ok {
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		effects := []string{"package_install"}
		if len(c.PreInstall) > 0 {
			effects = append(effects, "pre_install_hooks")
		}
		if len(c.PostInstall) > 0 {
			effects = append(effects, "post_install_hooks")
		}
		if len(c.ConfigFiles) > 0 {
			effects = append(effects, "configuration_files")
		}
		if len(c.Themes) > 0 {
			effects = append(effects, "themes")
		}
		if len(c.CleanupFiles) > 0 {
			effects = append(effects, "cleanup_files")
		}
		if len(c.AptSources) > 0 || c.BrewTap != "" {
			effects = append(effects, "package_sources")
		}
		for i, dep := range deps {
			if strings.Contains(dep, "://") || strings.ContainsAny(dep, " \t\n$=?#\\") {
				deps[i] = "[redacted]"
			}
		}
		result.Apps = append(result.Apps, PlannedApp{
			Name:           app.Name,
			Method:         c.InstallMethod,
			Channel:        app.SelectedChannel(),
			Channels:       availablePlanChannels(app),
			Plugin:         types.InstallPluginName(c.InstallMethod, c.InstallCommand),
			Dependencies:   deps,
			Effects:        effects,
			Privileges:     "unknown",
			UnknownEffects: true,
		})
		result.DependencyOrder = append(result.DependencyOrder, app.Name)
		if config.ClassifyInstallMethod(c.InstallMethod) != config.MethodSupported {
			result.Warnings = append(result.Warnings, "unsupported install method")
		}
		return nil
	}
	for _, app := range apps {
		if err := visit(app); err != nil {
			return result, APIError(err)
		}
	}
	if len(violations) > 0 {
		err := fmt.Errorf("organization policy denied installation:\n%s", strings.Join(violations, "\n"))
		return result, &Error{Category: "policy_denied", Message: err.Error(), cause: err}
	}
	if len(apps) > 0 {
		result.Warnings = append(result.Warnings, "Plugin effects and required privileges are unknown; no plugins or hooks were executed.")
	}
	return result, nil
}

type EffectiveApp struct {
	Name    string `json:"name"`
	Method  string `json:"method"`
	Default bool   `json:"default"`
}
type EffectiveConfig struct {
	Applications      []EffectiveApp `json:"applications"`
	Precedence        []string       `json:"precedence"`
	ApplicationSource string         `json:"application_source"`
}

// Effective returns only explicitly allowlisted fields; scripts, URLs, env and paths are absent.
func Effective(ctx context.Context, s config.CrossPlatformSettings) (EffectiveConfig, error) {
	result := EffectiveConfig{Applications: []EffectiveApp{}, Precedence: []string{"flags", "environment", "config", "defaults"}, ApplicationSource: "configuration_tiers"}
	if err := ctx.Err(); err != nil {
		return result, APIError(err)
	}
	_, merged, present, err := ConfiguredApplications(ctx, s)
	if err != nil {
		return result, APIError(err)
	}
	if present {
		result.ApplicationSource = "applications_override"
	}
	for _, app := range merged.GetAllApps() {
		if err := ctx.Err(); err != nil {
			return result, APIError(err)
		}
		c := app.GetOSConfig()
		result.Applications = append(result.Applications, EffectiveApp{Name: app.Name, Method: c.InstallMethod, Default: app.Default})
	}
	return result, nil
}
