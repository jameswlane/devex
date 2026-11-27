package agentapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// UnsupportedOSSkipMessage formats the user-facing skip notice emitted when an
// application has no install definition for the current operating system.
func UnsupportedOSSkipMessage(appName, currentOS string, supportedPlatforms []string) string {
	if len(supportedPlatforms) > 0 {
		return fmt.Sprintf("Skipped %s: not supported on %s (supported platforms: %s)", appName, currentOS, strings.Join(supportedPlatforms, ", "))
	}
	return fmt.Sprintf("Skipped %s: not supported on %s", appName, currentOS)
}

func ResolveAppsByName(settings config.CrossPlatformSettings, requested []string) ([]types.CrossPlatformApp, error) {
	resolved, skipped, err := resolveAppsByName(settings, requested, false)
	if err != nil {
		return nil, err
	}
	if len(skipped) > 0 {
		return nil, &Error{Category: "unsupported_platform", Message: strings.Join(skipped, "; ")}
	}
	return resolved, nil
}

func resolveAppsByName(settings config.CrossPlatformSettings, requested []string, skipUnsupportedOS bool) ([]types.CrossPlatformApp, []string, error) {
	apps := settings.GetAllApps()
	byName := make(map[string]types.CrossPlatformApp, len(apps))
	unsupported := make(map[string]types.CrossPlatformApp, len(apps))
	for _, app := range apps {
		if !app.IsSupported() {
			unsupported[normalizeInstallLookup(app.Name)] = app
			osConfig := app.GetOSConfig()
			if osConfig.InstallCommand != "" {
				unsupported[normalizeInstallLookup(osConfig.InstallCommand)] = app
			}
			continue
		}
		byName[normalizeInstallLookup(app.Name)] = app
		osConfig := app.GetOSConfig()
		if osConfig.InstallCommand != "" {
			byName[normalizeInstallLookup(osConfig.InstallCommand)] = app
		}
	}
	// Kebab-case names (claude-code) match catalog names too, after exact
	// names and install commands.
	bySlug := make(map[string]types.CrossPlatformApp, len(apps))
	unsupportedBySlug := make(map[string]types.CrossPlatformApp, len(apps))
	for _, app := range apps {
		slug := types.AppSlug(app.Name)
		if slug == "" {
			continue
		}
		target := bySlug
		if !app.IsSupported() {
			target = unsupportedBySlug
		}
		if _, exists := target[slug]; !exists {
			target[slug] = app
		}
	}

	resolved := make([]types.CrossPlatformApp, 0, len(requested))
	var unknown []string
	var archUnsupported []string
	var osUnsupported []string
	var skipWarnings []string
	seenSkipped := make(map[string]bool)
	for _, name := range requested {
		key := normalizeInstallLookup(name)
		app, ok := byName[key]
		if !ok {
			if _, exists := unsupported[key]; !exists {
				slug := types.AppSlug(name)
				if app, ok = bySlug[slug]; !ok {
					if candidate, found := unsupportedBySlug[slug]; found {
						unsupported[key] = candidate
					}
				}
			}
		}
		if !ok {
			if candidate, exists := unsupported[key]; exists {
				cfg := candidate.GetOSConfig()
				if types.CatalogOS() == "linux" && cfg.InstallMethod != "" && !cfg.IsArchitectureSupported(types.CatalogArch()) {
					archUnsupported = append(archUnsupported, fmt.Sprintf("%s is not supported on %s/%s (supported architectures: %s)", candidate.Name, types.CatalogOS(), types.CatalogArch(), strings.Join(cfg.SupportedArchitectures(), ", ")))
					continue
				}
				if cfg.InstallMethod == "" {
					platforms := GetSupportedPlatforms(candidate)
					switch {
					case skipUnsupportedOS:
						if !seenSkipped[normalizeInstallLookup(candidate.Name)] {
							seenSkipped[normalizeInstallLookup(candidate.Name)] = true
							skipWarnings = append(skipWarnings, UnsupportedOSSkipMessage(candidate.Name, types.CatalogOS(), platforms))
						}
					case len(platforms) > 0:
						osUnsupported = append(osUnsupported, fmt.Sprintf("%s is not supported on %s (supported platforms: %s)", candidate.Name, types.CatalogOS(), strings.Join(platforms, ", ")))
					default:
						osUnsupported = append(osUnsupported, fmt.Sprintf("%s is not supported on %s", candidate.Name, types.CatalogOS()))
					}
					continue
				}
			}
			unknown = append(unknown, name)
			continue
		}
		resolved = append(resolved, app)
	}
	platformErrors := make([]string, 0, len(archUnsupported)+len(osUnsupported))
	platformErrors = append(platformErrors, archUnsupported...)
	platformErrors = append(platformErrors, osUnsupported...)
	if len(platformErrors) > 0 && len(unknown) == 0 {
		return nil, nil, &Error{Category: "unsupported_platform", Message: strings.Join(platformErrors, "; ")}
	}
	if len(unknown) > 0 {
		if len(platformErrors) > 0 {
			return nil, nil, &Error{Category: "invalid_request", Message: fmt.Sprintf("%s; applications not found: %s", strings.Join(platformErrors, "; "), strings.Join(unknown, ", "))}
		}
		return nil, nil, fmt.Errorf("applications not found: %s", strings.Join(unknown, ", "))
	}
	return dedupeApps(resolved), skipWarnings, nil
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
