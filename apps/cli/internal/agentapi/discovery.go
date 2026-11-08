package agentapi

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type ListRequest struct {
	Kind    string
	Options ListCommandOptions
}
type ListResult struct {
	Installed  []InstalledApp
	Available  []AvailableApp
	Categories []CategoryInfo
}
type snapshotRepository struct {
	types.Repository
	apps []types.AppConfig
}

func (r snapshotRepository) ListApps() ([]types.AppConfig, error) { return r.apps, nil }
func List(ctx context.Context, repo types.Repository, s config.CrossPlatformSettings, r ListRequest) (ListResult, error) {
	result := ListResult{Installed: []InstalledApp{}, Available: []AvailableApp{}, Categories: []CategoryInfo{}}
	if err := ctx.Err(); err != nil {
		return result, APIError(err)
	}
	_, s, _, err := ConfiguredApplications(ctx, s)
	if err != nil {
		return result, APIError(err)
	}
	if r.Kind == "categories" {
		result.Categories = GetCategoryInfo(s)
		return result, nil
	}
	if r.Kind != "installed" && r.Kind != "available" {
		return result, &Error{Category: "invalid_request", Message: "unknown list kind"}
	}
	apps := []types.AppConfig{}
	if repo != nil {
		apps, err = repo.ListApps()
		if err != nil {
			return result, &Error{Category: "unavailable", Retryable: true, Message: "cannot read tracked applications", cause: err}
		}
	}
	if err := ctx.Err(); err != nil {
		return result, APIError(err)
	}
	snapshot := snapshotRepository{Repository: repo, apps: apps}
	if r.Kind == "installed" {
		result.Installed, err = GetInstalledApps(snapshot, s, r.Options)
		if result.Installed == nil {
			result.Installed = []InstalledApp{}
		}
	} else {
		result.Available = GetAvailableApps(snapshot, s, r.Options)
	}
	if err != nil {
		return result, APIError(err)
	}
	return result, nil
}
