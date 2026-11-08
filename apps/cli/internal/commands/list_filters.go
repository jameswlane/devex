package commands

import (
	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func getInstalledApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) ([]InstalledApp, error) {
	return agentapi.GetInstalledApps(repo, settings, options)
}
func getAvailableApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) []AvailableApp {
	return agentapi.GetAvailableApps(repo, settings, options)
}
func getInstalledAppsCache(repo types.Repository) map[string]bool {
	return agentapi.GetInstalledAppsCache(repo)
}
func getCategoryInfo(settings config.CrossPlatformSettings) []CategoryInfo {
	return agentapi.GetCategoryInfo(settings)
}
func filterInstalledApps(apps []InstalledApp, options ListCommandOptions) []InstalledApp {
	return agentapi.FilterInstalledApps(apps, options)
}
func filterAvailableApps(apps []AvailableApp, options ListCommandOptions) []AvailableApp {
	return agentapi.FilterAvailableApps(apps, options)
}
func groupAppsByCategory(apps []AvailableApp) map[string][]AvailableApp {
	return agentapi.GroupAppsByCategory(apps)
}
func getSortedCategories(categories map[string][]AvailableApp) []string {
	return agentapi.GetSortedCategories(categories)
}
func detectInstallationMethod(app types.CrossPlatformApp) string {
	return agentapi.DetectInstallationMethod(app)
}
func getPreferredInstallationMethod(app types.CrossPlatformApp) string {
	return agentapi.GetPreferredInstallationMethod(app)
}
func matchesListCategory(category, filter string) bool {
	return agentapi.MatchesListCategory(category, filter)
}
