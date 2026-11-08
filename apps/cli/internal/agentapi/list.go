package agentapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// GetInstalledApps retrieves and processes the list of installed applications
func GetInstalledApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) ([]InstalledApp, error) {
	// Get cached installed apps
	installedAppsCache := GetInstalledAppsCache(repo)

	// Get all available apps for cross-referencing
	allApps := settings.GetAllApps()
	known := make(map[string]bool, len(allApps))
	for _, app := range allApps {
		known[app.Name] = true
	}
	var installedApps []InstalledApp
	for name := range installedAppsCache {
		if known[name] {
			continue
		}
		data, err := repo.Get("installed-app-definition:" + normalizeInstallLookup(name))
		if err != nil || data == "" {
			installedApps = append(installedApps, InstalledApp{
				Name:              name,
				Description:       "No application definition available",
				Category:          "unknown",
				Method:            "unknown",
				Status:            "Installed",
				MissingDefinition: true,
			})
			continue
		}
		var app types.CrossPlatformApp
		if err := json.Unmarshal([]byte(data), &app); err != nil {
			return nil, fmt.Errorf("invalid installed definition for %s: %w", name, err)
		}
		allApps = append(allApps, app)
	}

	// Convert to InstalledApp format with method detection
	for _, app := range allApps {
		if installedAppsCache[app.Name] {
			// Determine installation method
			method := DetectInstallationMethod(app)

			installedApp := InstalledApp{
				Name:        app.Name,
				Description: app.Description,
				Category:    app.Category,
				Method:      method,
				Status:      "Installed",
			}
			installedApps = append(installedApps, installedApp)
		}
	}

	// Apply filters
	filteredApps := FilterInstalledApps(installedApps, options)
	return filteredApps, nil
}

// GetAvailableApps retrieves and processes the list of available applications
func GetAvailableApps(repo types.Repository, settings config.CrossPlatformSettings, options ListCommandOptions) []AvailableApp {
	allApps := settings.GetAllApps()
	installedAppsCache := GetInstalledAppsCache(repo)

	availableApps := make([]AvailableApp, 0, len(allApps))

	for _, app := range allApps {
		// Applications without a method for this platform are not available here.
		if !app.IsSupported() {
			continue
		}
		// Skip already installed apps unless verbose mode
		if installedAppsCache[app.Name] && !options.Verbose {
			continue
		}

		platforms := GetSupportedPlatforms(app)
		platformStr := strings.Join(platforms, ", ")
		if platformStr == "" {
			platformStr = "Universal"
		}

		// Determine preferred installation method
		method := GetPreferredInstallationMethod(app)

		availableApp := AvailableApp{
			Name:        app.Name,
			Description: app.Description,
			Category:    app.Category,
			Platform:    platformStr,
			Method:      method,
			Recommended: app.Default, // Use Default field instead of Recommended
		}
		availableApps = append(availableApps, availableApp)
	}

	// Apply filters
	filteredApps := FilterAvailableApps(availableApps, options)
	return filteredApps
}

// GetInstalledAppsCache creates a cache of installed applications for efficient lookup
func GetInstalledAppsCache(repo types.Repository) map[string]bool {
	installedApps := make(map[string]bool)

	dbApps, err := repo.ListApps()
	if err != nil {
		log.Warn("Failed to retrieve installed apps from database: %v", err)
		return installedApps
	}

	for _, app := range dbApps {
		installedApps[app.Name] = true
	}

	return installedApps
}

// GetCategoryInfo processes and returns information about categories
func GetCategoryInfo(settings config.CrossPlatformSettings) []CategoryInfo {
	allApps := settings.GetAllApps()
	categories := make(map[string][]AvailableApp)

	// Group apps by category and deduplicate platforms using set
	for _, app := range allApps {
		category := app.Category
		if category == "" {
			category = "Other"
		}

		platforms := GetSupportedPlatforms(app)
		platformStr := strings.Join(platforms, ", ")
		if platformStr == "" {
			platformStr = "Universal"
		}

		availableApp := AvailableApp{
			Name:        app.Name,
			Description: app.Description,
			Category:    category,
			Platform:    platformStr,
			Method:      GetPreferredInstallationMethod(app),
			Recommended: app.Default, // Use Default field instead of Recommended
		}
		categories[category] = append(categories[category], availableApp)

		// Note: Platform deduplication will be handled later when converting to CategoryInfo
	}

	// Convert to CategoryInfo slice
	categoryInfos := make([]CategoryInfo, 0, len(categories))
	for category, apps := range categories {
		// Collect unique platforms for this category
		platformSet := make(map[string]struct{})
		for _, app := range apps {
			appPlatforms := strings.Split(app.Platform, ", ")
			for _, platform := range appPlatforms {
				if platform != "" {
					platformSet[platform] = struct{}{}
				}
			}
		}

		// Convert platform set to sorted slice
		platforms := make([]string, 0, len(platformSet))
		for platform := range platformSet {
			platforms = append(platforms, platform)
		}
		sort.Strings(platforms)

		categoryInfo := CategoryInfo{
			Category:    category,
			Description: GetCategoryDescription(category),
			Count:       len(apps),
			Platforms:   platforms,
		}
		categoryInfos = append(categoryInfos, categoryInfo)
	}

	// Sort by category name for consistent output
	sort.Slice(categoryInfos, func(i, j int) bool {
		return categoryInfos[i].Category < categoryInfos[j].Category
	})

	return categoryInfos
}

// FilterInstalledApps applies filtering options to installed apps
func FilterInstalledApps(apps []InstalledApp, options ListCommandOptions) []InstalledApp {
	if len(apps) == 0 {
		return apps
	}

	filtered := make([]InstalledApp, 0, len(apps))

	// Pre-compute search term in lowercase for performance
	var searchLower string
	if options.Search != "" {
		searchLower = strings.ToLower(options.Search)
	}

	for _, app := range apps {
		// Apply category filter
		if options.Category != "" && !MatchesListCategory(app.Category, options.Category) {
			continue
		}

		// Apply method filter
		if options.Method != "" && !strings.EqualFold(app.Method, options.Method) {
			continue
		}

		// Apply search filter (case-insensitive)
		if searchLower != "" {
			nameMatch := strings.Contains(strings.ToLower(app.Name), searchLower)
			descMatch := strings.Contains(strings.ToLower(app.Description), searchLower)
			if !nameMatch && !descMatch {
				continue
			}
		}

		filtered = append(filtered, app)
	}

	// Sort by name for consistent output
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	return filtered
}

// FilterAvailableApps applies filtering options to available apps
func FilterAvailableApps(apps []AvailableApp, options ListCommandOptions) []AvailableApp {
	if len(apps) == 0 {
		return apps
	}

	filtered := make([]AvailableApp, 0, len(apps))

	// Pre-compute search term in lowercase for performance
	var searchLower string
	if options.Search != "" {
		searchLower = strings.ToLower(options.Search)
	}

	for _, app := range apps {
		// Apply category filter
		if options.Category != "" && !MatchesListCategory(app.Category, options.Category) {
			continue
		}

		// Apply method filter
		if options.Method != "" && !strings.EqualFold(app.Method, options.Method) {
			continue
		}

		// Apply recommended filter
		if options.Recommended && !app.Recommended {
			continue
		}

		// Apply search filter (case-insensitive)
		if searchLower != "" {
			nameMatch := strings.Contains(strings.ToLower(app.Name), searchLower)
			descMatch := strings.Contains(strings.ToLower(app.Description), searchLower)
			if !nameMatch && !descMatch {
				continue
			}
		}

		filtered = append(filtered, app)
	}

	// Sort by name for consistent output
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	return filtered
}

// GroupAppsByCategory groups available apps by their category
func GroupAppsByCategory(apps []AvailableApp) map[string][]AvailableApp {
	categories := make(map[string][]AvailableApp)

	for _, app := range apps {
		category := app.Category
		if category == "" {
			category = "Other"
		}
		categories[category] = append(categories[category], app)
	}

	return categories
}

// GetSortedCategories returns category names sorted alphabetically
func GetSortedCategories(categories map[string][]AvailableApp) []string {
	sortedCategories := make([]string, 0, len(categories))
	for category := range categories {
		sortedCategories = append(sortedCategories, category)
	}
	sort.Strings(sortedCategories)
	return sortedCategories
}

// DetectInstallationMethod determines how an app was installed
func DetectInstallationMethod(app types.CrossPlatformApp) string {
	// For now, return the preferred method as the detected method
	// TODO: Implement actual detection logic based on system state
	method := GetPreferredInstallationMethod(app)
	if method != "manual" {
		return method
	}

	// Default fallback
	return "system"
}

// GetPreferredInstallationMethod determines the preferred installation method for an app
func GetPreferredInstallationMethod(app types.CrossPlatformApp) string {
	// Check all platforms first (for cross-platform tools)
	if app.AllPlatforms.InstallMethod != "" {
		return app.AllPlatforms.InstallMethod
	}

	// Check current platform specific method
	config := app.GetOSConfig()
	if config.InstallMethod != "" {
		return config.InstallMethod
	}

	return "manual"
}

// MatchesListCategory accepts category labels and their short, case-insensitive slugs.
func MatchesListCategory(category, filter string) bool {
	categorySlug := configurationPathPart(category)
	filterSlug := configurationPathPart(filter)
	if categorySlug == filterSlug {
		return true
	}
	for _, suffix := range []string{"-tools", "-apps"} {
		if strings.TrimSuffix(categorySlug, suffix) == filterSlug {
			return true
		}
	}
	return false
}

func configurationPathPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	lastDash := false
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-'
		if valid {
			result.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash && result.Len() > 0 {
			result.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(result.String(), "-")
}
