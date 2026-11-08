package agentapi

import (
	"context"
	"errors"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
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
	Names      []string
	Categories []string
}
type Plan struct {
	Apps            []PlannedApp `json:"apps"`
	DependencyOrder []string     `json:"dependency_order"`
	Warnings        []string     `json:"warnings"`
}
type PlannedApp struct {
	Name           string   `json:"name"`
	Method         string   `json:"method"`
	Plugin         string   `json:"plugin"`
	Dependencies   []string `json:"dependencies"`
	Effects        []string `json:"effects"`
	Privileges     string   `json:"privileges"`
	UnknownEffects bool     `json:"unknown_effects"`
}

// SelectApplications is the shared CLI execution and planning selection policy.
func SelectApplications(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest) ([]types.CrossPlatformApp, config.CrossPlatformSettings, error) {
	if err := ctx.Err(); err != nil {
		return nil, s, err
	}
	if len(r.Names) > 0 && len(r.Categories) > 0 {
		return nil, s, &Error{Category: "invalid_request", Message: "cannot combine applications and categories"}
	}
	configured, merged, present, err := ConfiguredApplications(ctx, s)
	if err != nil {
		return nil, s, err
	}
	s = merged
	var apps []types.CrossPlatformApp
	switch {
	case len(r.Names) > 0:
		apps, err = ResolveAppsByName(s, r.Names)
	case len(r.Categories) > 0:
		apps, err = ResolveAppsByCategory(s, r.Categories)
	case present:
		apps = configured
	default:
		apps = s.GetDefaultApps()
	}
	if err != nil {
		return nil, s, err
	}
	if len(apps) == 0 && !present {
		return nil, s, &Error{Category: "not_found", Message: "no applications matched the install request"}
	}
	return apps, s, nil
}

// PlanInstall has no installer, executor, filesystem writer, or network dependency.
// Plugin and script effects are deliberately unknown until plugins support describe.
func PlanInstall(ctx context.Context, s config.CrossPlatformSettings, r PlanRequest) (Plan, error) {
	result := Plan{Apps: []PlannedApp{}, DependencyOrder: []string{}, Warnings: []string{}}
	apps, s, err := SelectApplications(ctx, s, r)
	if err != nil {
		return result, APIError(err)
	}
	if err = config.RequireValidConfig(ctx, s); err != nil {
		return result, APIError(err)
	}
	byName := map[string]types.CrossPlatformApp{}
	aliases := map[string]string{}
	for _, app := range s.GetAllApps() {
		byName[normalizeInstallLookup(app.Name)] = app
		aliases[normalizeInstallLookup(app.Name)] = normalizeInstallLookup(app.Name)
		c := app.GetOSConfig()
		if c.InstallCommand != "" {
			aliases[normalizeInstallLookup(c.InstallCommand)] = normalizeInstallLookup(app.Name)
		}
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
		visited[key] = true
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
		result.Apps = append(result.Apps, PlannedApp{Name: app.Name, Method: c.InstallMethod, Plugin: "package-manager-" + c.InstallMethod, Dependencies: deps, Effects: effects, Privileges: "unknown", UnknownEffects: true})
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
