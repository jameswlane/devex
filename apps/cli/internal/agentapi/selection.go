package agentapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func ResolveAppsByName(settings config.CrossPlatformSettings, requested []string) ([]types.CrossPlatformApp, error) {
	apps := settings.GetAllApps()
	byName := make(map[string]types.CrossPlatformApp, len(apps))
	for _, app := range apps {
		if !app.IsSupported() {
			continue
		}
		byName[normalizeInstallLookup(app.Name)] = app
		osConfig := app.GetOSConfig()
		if osConfig.InstallCommand != "" {
			byName[normalizeInstallLookup(osConfig.InstallCommand)] = app
		}
	}

	resolved := make([]types.CrossPlatformApp, 0, len(requested))
	var unknown []string
	for _, name := range requested {
		app, ok := byName[normalizeInstallLookup(name)]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		resolved = append(resolved, app)
	}
	if len(unknown) > 0 {
		return nil, fmt.Errorf("applications not found: %s", strings.Join(unknown, ", "))
	}
	return dedupeApps(resolved), nil
}

func ResolveAppsByCategory(settings config.CrossPlatformSettings, categories []string) ([]types.CrossPlatformApp, error) {
	requested := make(map[string]bool, len(categories))
	requestedLabels := make(map[string]string, len(categories))
	for _, category := range categories {
		normalized := normalizeInstallLookup(category)
		requested[normalized] = true
		requestedLabels[normalized] = category
	}

	var resolved []types.CrossPlatformApp
	available := make(map[string]bool)
	matched := make(map[string]bool, len(requested))
	for _, app := range settings.GetAllApps() {
		if app.Category != "" {
			available[app.Category] = true
		}
		normalizedCategory := normalizeInstallLookup(app.Category)
		if requested[normalizedCategory] {
			matched[normalizedCategory] = true
		}
		if requested[normalizedCategory] && app.IsSupported() {
			resolved = append(resolved, app)
		}
	}
	if len(matched) != len(requested) {
		unknown := make([]string, 0, len(requested)-len(matched))
		for category := range requested {
			if !matched[category] {
				unknown = append(unknown, requestedLabels[category])
			}
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown categories: %s", strings.Join(unknown, ", "))
	}
	if len(resolved) == 0 {
		categoryNames := make([]string, 0, len(available))
		for category := range available {
			categoryNames = append(categoryNames, category)
		}
		sort.Strings(categoryNames)
		return nil, fmt.Errorf("no supported applications found for categories %s; available categories: %s", strings.Join(categories, ","), strings.Join(categoryNames, ", "))
	}
	return dedupeApps(resolved), nil
}

func dedupeApps(apps []types.CrossPlatformApp) []types.CrossPlatformApp {
	seen := make(map[string]bool, len(apps))
	deduped := make([]types.CrossPlatformApp, 0, len(apps))
	for _, app := range apps {
		key := normalizeInstallLookup(app.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, app)
	}
	return deduped
}

func normalizeInstallLookup(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
