package agentapi

import (
	"context"
	"fmt"
	"strings"

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
	failed := false
	for _, app := range apps {
		if err := ctx.Err(); err != nil {
			return results, APIError(err)
		}
		if resolved, e := s.GetApplicationByName(app.Name); e == nil && app.InstallMethod == "" {
			app = *resolved
		}
		if r.Category != "" && !strings.EqualFold(app.Category, r.Category) {
			continue
		}
		result := status.AppStatus{Name: app.Name, InstallMethod: app.InstallMethod, Status: "unknown", Issues: []string{}}
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
