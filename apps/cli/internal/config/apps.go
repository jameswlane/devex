package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/common"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// platformCache provides cached platform detection to avoid repeated system calls
var (
	platformCache      *platform.DetectionResult
	platformCacheMutex sync.RWMutex
	platformCacheOnce  sync.Once
)

// getCachedPlatform returns the platform information, caching it for the session
func getCachedPlatform() platform.DetectionResult {
	platformCacheOnce.Do(func() {
		log.Debug("Detecting platform for the first time - caching result for session")
		detected := platform.DetectPlatform()
		platformCache = &detected
	})

	platformCacheMutex.RLock()
	defer platformCacheMutex.RUnlock()
	return *platformCache
}

// GetAppList retrieves the list of apps from the settings.
func GetAppList(settings CrossPlatformSettings) ([]types.AppConfig, error) {
	log.Info("Retrieving app list")
	return settings.GetApplications(), nil
}

// ValidateApp checks the validity of an individual app.
func ValidateApp(app types.AppConfig) error {
	log.Info("Validating app configuration", "app", app.Name)

	if err := common.ValidateAppConfig(app.Name, app.InstallMethod); err != nil {
		return err
	}

	log.Info("App validated successfully", "app", app.Name)
	return nil
}

// ListAppsByCategory filters apps by categories using optimized data structures.
func ListAppsByCategory(settings CrossPlatformSettings, categories []string) ([]types.AppConfig, error) {
	log.Info("Filtering apps by categories", "categories", categories)

	apps := settings.GetApplications()

	// Use map for O(1) category lookup instead of O(n) loops
	categorySet := make(map[string]bool, len(categories))
	for _, category := range categories {
		categorySet[category] = true
	}

	// Pre-allocate slice with reasonable capacity based on app count
	filteredApps := make([]types.AppConfig, 0, len(apps)/2)
	for _, app := range apps {
		if categorySet[app.Category] {
			filteredApps = append(filteredApps, app)
		}
	}

	log.Info("Filtered apps by categories", "count", len(filteredApps))
	return filteredApps, nil
}

// convertToAppConfig converts a CrossPlatformApp to AppConfig format for backward compatibility
func convertToAppConfig(app types.CrossPlatformApp) *types.AppConfig {
	osConfig := app.GetOSConfig()
	return &types.AppConfig{
		BaseConfig: types.BaseConfig{
			Name:        app.Name,
			Description: app.Description,
			Category:    app.Category,
		},
		Default:          app.Default,
		InstallMethod:    osConfig.InstallMethod,
		InstallCommand:   osConfig.InstallCommand,
		UninstallCommand: osConfig.UninstallCommand,
		Dependencies:     osConfig.Dependencies,
		PreInstall:       osConfig.PreInstall,
		PostInstall:      osConfig.PostInstall,
		ConfigFiles:      osConfig.ConfigFiles,
		AptSources:       osConfig.AptSources,
		CleanupFiles:     osConfig.CleanupFiles,
		Conflicts:        osConfig.Conflicts,
		DownloadURL:      osConfig.DownloadURL,
		InstallDir:       osConfig.Destination,
	}
}

// FindAppByName retrieves an app by its name from the cross-platform settings.
func FindAppByName(settings CrossPlatformSettings, name string) (*types.AppConfig, error) {
	log.Info("Finding app by name", "name", name)

	for _, app := range settings.GetAllApps() {
		if app.Name == name {
			log.Info("App found", "name", name)
			return convertToAppConfig(app), nil
		}
	}

	log.Error("App not found", fmt.Errorf("app name: %s", name))
	return nil, fmt.Errorf("app not found: %s", name)
}

// GetAppInfo retrieves the AppConfig for a given install_command or name.
func GetAppInfo(identifier string) (*types.AppConfig, error) {
	log.Info("Retrieving app information", "identifier", identifier)

	// List of sections to search
	sections := []string{"apps", "databases", "optional_apps", "programming_languages"}

	for _, section := range sections {
		sectionData := viper.Get(section)
		apps, ok := sectionData.([]any)
		if !ok {
			log.Warn("Section is not a slice", "section", section)
			continue
		}
		for _, app := range apps {
			candidate, ok := app.(map[string]any)
			if !ok {
				log.Warn("App is not a map", "app", app)
				continue
			}
			if candidate["install_command"] == identifier || candidate["name"] == identifier {
				log.Info("App configuration found", "identifier", identifier)

				name, ok := candidate["name"].(string)
				if !ok {
					log.Warn("App name is not a string", "name", candidate["name"])
					continue
				}

				description, ok := candidate["description"].(string)
				if !ok {
					log.Warn("App description is not a string", "description", candidate["description"])
					continue
				}

				installMethod, ok := candidate["install_method"].(string)
				if !ok {
					log.Warn("Install method is not a string", "install_method", candidate["install_method"])
					continue
				}

				installCommand, ok := candidate["install_command"].(string)
				if !ok {
					log.Warn("Install command is not a string", "install_command", candidate["install_command"])
					continue
				}

				downloadURL, ok := candidate["download_url"].(string)
				if !ok {
					downloadURL = ""
				}

				return &types.AppConfig{
					BaseConfig: types.BaseConfig{
						Name:        name,
						Description: description,
					},
					InstallMethod:  installMethod,
					InstallCommand: installCommand,
					DownloadURL:    downloadURL,
					Dependencies:   ResolvePlatformDependencies(candidate),
					PostInstall:    toInstallCommandSlice(candidate["post_install"]),
				}, nil
			}
		}
	}

	// log.Error("No app configuration found", ,"identifier", identifier)
	return nil, fmt.Errorf("no app configuration found for identifier: %s", identifier)
}

func ToStringSlice(input any) []string {
	if input == nil {
		return nil
	}

	log.Info("Converting to string slice")
	items, ok := input.([]any)
	if !ok {
		log.Warn("Conversion failed: input is not a slice", "input", input)
		return nil
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		if str, ok := item.(string); ok {
			result = append(result, str)
		} else {
			log.Warn("Item is not a string, skipping", "item", item)
		}
	}

	log.Info("Converted to string slice", "count", len(result))
	return result
}

func toInstallCommandSlice(input any) []types.InstallCommand {
	if input == nil {
		return nil
	}

	log.Info("Converting to install command slice")
	items, ok := input.([]any)
	if !ok {
		log.Warn("Conversion failed: input is not a slice", "input", input)
		return nil
	}

	result := make([]types.InstallCommand, 0, len(items))
	for _, item := range items {
		cmd, ok := item.(map[string]any)
		if !ok {
			log.Warn("Item is not a map, skipping", "item", item)
			continue
		}

		command, ok := cmd["command"].(string)
		if !ok {
			log.Warn("Command is not a string, skipping", "command", cmd["command"])
			continue
		}

		shell, ok := cmd["shell"].(string)
		if !ok {
			log.Warn("Shell is not a string, skipping", "shell", cmd["shell"])
			continue
		}

		result = append(result, types.InstallCommand{
			Command: command,
			Shell:   shell,
		})
	}

	log.Info("Converted to install command slice", "count", len(result))
	return result
}

// ResolvePlatformDependencies resolves dependencies for the current platform from platform_requirements
func ResolvePlatformDependencies(candidate map[string]any) []string {
	// First check for legacy dependencies field (backward compatibility)
	if legacyDeps := ToStringSlice(candidate["dependencies"]); len(legacyDeps) > 0 {
		log.Info("Using legacy dependencies field", "count", len(legacyDeps))
		return legacyDeps
	}

	// Get current platform (cached for performance)
	currentPlatform := getCachedPlatform()
	log.Info("Resolving platform-specific dependencies", "os", currentPlatform.OS, "distribution", currentPlatform.Distribution)

	// Check platform_requirements for OS-specific dependencies
	platformReqs, ok := candidate["platform_requirements"].([]any)
	if !ok {
		log.Debug("No platform_requirements found")
		return nil
	}

	for _, req := range platformReqs {
		requirement, ok := req.(map[string]any)
		if !ok {
			log.Warn("Platform requirement is not a map", "requirement", req)
			continue
		}

		reqOS, ok := requirement["os"].(string)
		if !ok {
			log.Warn("Platform requirement OS is not a string", "os", requirement["os"])
			continue
		}

		// Check if this requirement matches our current platform
		if MatchesPlatform(reqOS, &currentPlatform) {
			deps := ToStringSlice(requirement["dependencies"])
			if len(deps) > 0 {
				log.Info("Found platform-specific dependencies", "os", reqOS, "count", len(deps))
				return deps
			}
		}
	}

	log.Debug("No platform-specific dependencies found for current platform")
	return nil
}

// MatchesPlatform checks if a platform requirement matches the current platform
func MatchesPlatform(reqOS string, currentPlatform *platform.DetectionResult) bool {
	if currentPlatform == nil {
		log.Warn("Current platform is nil, cannot match platform requirements")
		return false
	}

	// Direct OS match
	if reqOS == currentPlatform.OS {
		return true
	}

	// Linux distribution match
	if currentPlatform.OS == "linux" && reqOS == currentPlatform.Distribution {
		return true
	}

	return false
}
