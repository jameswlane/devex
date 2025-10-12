package setup

import (
	"os"
	"strconv"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// getAvailableThemeNames returns a list of unique theme names from application configurations
func getAvailableThemeNames(settings config.CrossPlatformSettings) []string {
	log.Debug("Loading available themes from application configurations")

	// Get all applications from all categories in settings
	var allApps []interface{}

	// Collect applications from all categories using GetAllApps
	allConfigApps := settings.GetAllApps()
	appCategories := [][]types.CrossPlatformApp{
		allConfigApps, // Use the unified list from GetAllApps
	}

	// Convert CrossPlatformApp slice to interface{} slice for GetAvailableThemes
	for _, category := range appCategories {
		for _, app := range category {
			// Get themes from the appropriate OS config
			osConfig := app.GetOSConfig()
			if len(osConfig.Themes) > 0 {
				appWithThemes := map[string]interface{}{
					"name":   app.Name,
					"themes": convertThemesToInterface(osConfig.Themes),
				}
				allApps = append(allApps, appWithThemes)
			}
		}
	}

	// Get unique themes from all applications
	// TODO: Use desktop-themes plugin for theme management
	var themeNames []string
	themeSet := make(map[string]bool)

	// Extract theme names from collected apps
	for _, appInterface := range allApps {
		appMap, ok := appInterface.(map[string]interface{})
		if !ok {
			continue
		}

		themesInterface, exists := appMap["themes"]
		if !exists {
			continue
		}

		themes, ok := themesInterface.([]interface{})
		if !ok {
			continue
		}

		for _, themeInterface := range themes {
			themeMap, ok := themeInterface.(map[string]interface{})
			if !ok {
				continue
			}

			themeName, exists := themeMap["name"]
			if !exists {
				continue
			}

			name, ok := themeName.(string)
			if !ok || themeSet[name] {
				continue
			}

			themeNames = append(themeNames, name)
			themeSet[name] = true
		}
	}

	log.Debug("Loaded themes from configurations", "count", len(themeNames), "themes", themeNames)

	// Fallback to default themes if none found in configurations
	// This fallback is used when:
	// - Configuration files are missing or corrupted
	// - No desktop environment themes are defined
	// - Theme loading from config files fails
	if len(themeNames) == 0 {
		log.Warn("No themes found in application configurations, using fallback themes")
		return FallbackThemes
	}

	return themeNames
}

// convertThemesToInterface converts []types.Theme to []interface{} for GetAvailableThemes
func convertThemesToInterface(themes []types.Theme) []interface{} {
	result := make([]interface{}, len(themes))
	for i, theme := range themes {
		result[i] = map[string]interface{}{
			"name":             theme.Name,
			"theme_color":      theme.ThemeColor,
			"theme_background": theme.ThemeBackground,
		}
	}
	return result
}

// getProgrammingLanguageNames extracts programming language names from environment configuration
func getProgrammingLanguageNames(settings config.CrossPlatformSettings) []string {
	if len(settings.ProgrammingLanguages) == 0 {
		log.Warn("No programming languages found in environment configuration, using fallback")
		// Fallback to default languages if none found in configuration
		return []string{
			"Node.js",
			"Python",
			"Go",
			"Ruby",
			"Java",
			"Rust",
		}
	}

	// Performance optimization: Pre-allocate slice with known capacity to avoid reallocations
	languageNames := make([]string, 0, len(settings.ProgrammingLanguages))
	for _, lang := range settings.ProgrammingLanguages {
		languageNames = append(languageNames, lang.Name)
	}

	log.Debug("Loaded programming languages from environment configuration", "count", len(languageNames), "languages", languageNames)
	return languageNames
}

// getDesktopAppNames returns a list of desktop application names from settings
func getDesktopAppNames(settings config.CrossPlatformSettings) []string {
	// For now, return applications from DesktopApps category if it exists
	// In the future, this should be filtered by desktop environment
	allApps := settings.GetAllApps()
	// Pre-allocate slice with known capacity to avoid reallocations
	desktopAppNames := make([]string, 0, len(allApps))

	// Filter apps that are typically desktop applications
	// You can refine this logic based on your application categories
	for _, app := range allApps {
		// Add all apps that have GUI or desktop-related categories
		// This is a simple heuristic - adjust based on your needs
		desktopAppNames = append(desktopAppNames, app.Name)
	}

	log.Debug("Loaded desktop applications", "count", len(desktopAppNames))
	return desktopAppNames
}

// getPluginTimeout returns the plugin installation timeout from environment or default
func getPluginTimeout() time.Duration {
	// Check for custom timeout in environment (in seconds)
	if timeoutStr := os.Getenv("DEVEX_PLUGIN_TIMEOUT"); timeoutStr != "" {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			timeout := time.Duration(seconds) * time.Second
			log.Debug("Using custom plugin timeout from environment", "timeout", timeout)
			return timeout
		}
	}

	// Default timeout
	log.Debug("Using default plugin timeout", "timeout", PluginInstallTimeout)
	return PluginInstallTimeout
}
