package agentapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/commands/status"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type StatusRequest struct {
	Names    []string
	Category string
}
type StatusReader interface {
	ListApps() ([]types.AppConfig, error)
	GetApp(string) (*types.AppConfig, error)
}
type StatusProbe func(context.Context, types.AppConfig) (bool, error)

// Status checks installation only. It never repairs, runs lifecycle health hooks,
// creates backup managers, or initializes/downloads plugins.
func Status(ctx context.Context, s config.CrossPlatformSettings, repo StatusReader, r StatusRequest, probe StatusProbe) ([]status.AppStatus, error) {
	results := []status.AppStatus{}
	if err := ctx.Err(); err != nil {
		return results, APIError(err)
	}
	_, s, _, err := ConfiguredApplications(ctx, s)
	if err != nil {
		return results, APIError(err)
	}
	apps := []types.AppConfig{}
	unknown := 0
	if len(r.Names) > 0 {
		for _, input := range r.Names {
			for _, name := range strings.Split(input, ",") {
				name = strings.TrimSpace(name)
				var app *types.AppConfig
				if repo != nil {
					app, _ = repo.GetApp(name)
				}
				if app == nil {
					app, _ = s.GetApplicationByName(name)
				}
				if app == nil {
					unknown++
					continue
				}
				apps = append(apps, *app)
			}
		}
	} else if repo != nil {
		apps, err = repo.ListApps()
		if err != nil {
			return results, &Error{Category: "unavailable", Retryable: true, Message: "cannot read tracked applications", cause: err}
		}
	}
	var receiptGetter channels.ReceiptGetter
	if getter, ok := repo.(channels.ReceiptGetter); ok {
		receiptGetter = getter
	}
	catalogByName := map[string]types.CrossPlatformApp{}
	for _, entry := range s.GetAllApps() {
		catalogByName[normalizeInstallLookup(entry.Name)] = entry
		if cmd := strings.TrimSpace(entry.GetOSConfig().InstallCommand); cmd != "" {
			key := normalizeInstallLookup(cmd)
			if _, exists := catalogByName[key]; !exists {
				catalogByName[key] = entry
			}
		}
	}
	resolveStatusApp := func(app types.AppConfig) types.AppConfig {
		// Recorded definition or receipt always wins over configured channel
		// preferences so status probes the channel that was actually installed.
		if receiptGetter != nil {
			if data, readErr := receiptGetter.Get("installed-app-definition:" + normalizeInstallLookup(app.Name)); readErr == nil && data != "" {
				var saved types.CrossPlatformApp
				if json.Unmarshal([]byte(data), &saved) == nil && saved.Name != "" {
					if rec := channels.RecordedChannelForApp(receiptGetter, saved); rec != "" {
						if switched, _, err := channels.Resolve(saved, channels.Preferences{}, rec); err == nil {
							saved = switched
						}
					}
					return saved.ToLegacyAppConfig()
				}
			}
		}
		if catalogApp, ok := catalogByName[normalizeInstallLookup(app.Name)]; ok {
			if rec := channels.RecordedChannelForApp(receiptGetter, catalogApp); rec != "" {
				if switched, _, err := channels.Resolve(catalogApp, channels.Preferences{}, rec); err == nil {
					return switched.ToLegacyAppConfig()
				}
			}
			if app.InstallMethod == "" {
				if chosen, _, err := channels.Resolve(catalogApp, channels.PreferencesFromViper(), ""); err == nil {
					return chosen.ToLegacyAppConfig()
				}
				return catalogApp.ToLegacyAppConfig()
			}
		}
		if resolved, e := s.GetApplicationByName(app.Name); e == nil && app.InstallMethod == "" {
			return *resolved
		}
		return app
	}
	failed := false
	for _, app := range apps {
		if err := ctx.Err(); err != nil {
			return results, APIError(err)
		}
		app = resolveStatusApp(app)
		if r.Category != "" && !strings.EqualFold(app.Category, r.Category) {
			continue
		}
		result := status.AppStatus{Name: app.Name, InstallMethod: app.InstallMethod, Channel: app.Channel, Status: "unknown", Issues: []string{}}
		if app.InstallMethod == "" {
			result.Issues = append(result.Issues, "Install method unknown: the application is not in the DevEx configuration")
		}
		if probe != nil && app.InstallMethod != "" {
			installed, e := probe(ctx, app)
			if ctx.Err() != nil {
				return results, APIError(ctx.Err())
			}
			if e != nil {
				category := APIError(e).Category
				if category != "unsupported" {
					result.Status = "error"
					failed = true
				}
				result.Issues = append(result.Issues, category)
			} else {
				result.Installed = installed
				if installed {
					result.Status = "healthy"
				} else {
					result.Status = "not_installed"
					failed = true
				}
			}
		}
		results = append(results, result)
	}
	if unknown > 0 {
		return results, &Error{Category: "not_found", Message: fmt.Sprintf("%d requested application(s) are not known to DevEx", unknown)}
	}
	if failed {
		return results, &Error{Category: "partial_failure", Message: "some applications are missing or could not be checked"}
	}
	return results, nil
}
