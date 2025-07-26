package config

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// GetAppList retrieves the list of apps from the settings.
func GetAppList(settings Settings) ([]types.AppConfig, error) {
	log.Info("Retrieving app list")
	return settings.Apps, nil
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

// ListAppsByCategory filters apps by categories.
func ListAppsByCategory(settings Settings, categories []string) ([]types.AppConfig, error) {
	log.Info("Filtering apps by categories", "categories", categories)

	var filteredApps []types.AppConfig
	for _, app := range settings.Apps {
		for _, category := range categories {
			if app.Category == category {
				filteredApps = append(filteredApps, app)
			}
		}
	}

	log.Info("Filtered apps by categories", "count", len(filteredApps))
	return filteredApps, nil
}

// FindAppByName retrieves an app by its name from the cross-platform settings.
func FindAppByName(settings CrossPlatformSettings, name string) (*types.AppConfig, error) {
	log.Info("Finding app by name", "name", name)

	for _, app := range settings.GetAllApps() {
		if app.Name == name {
			log.Info("App found", "name", name)

			// Convert to AppConfig format for compatibility
			osConfig := app.GetOSConfig()
			appConfig := &types.AppConfig{
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
			return appConfig, nil
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
		for _, app := range viper.Get(section).([]any) {
			candidate := app.(map[string]any)
			if candidate["install_command"] == identifier || candidate["name"] == identifier {
				log.Info("App configuration found", "identifier", identifier)
				return &types.AppConfig{
					BaseConfig: types.BaseConfig{
						Name:        candidate["name"].(string),
						Description: candidate["description"].(string),
					},
					InstallMethod:  candidate["install_method"].(string),
					InstallCommand: candidate["install_command"].(string),
					DownloadURL:    candidate["download_url"].(string),
					Dependencies:   ToStringSlice(candidate["dependencies"]),
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

	result := make([]string, len(items))
	for i, item := range items {
		result[i] = item.(string)
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

	result := make([]types.InstallCommand, len(items))
	for i, item := range items {
		cmd := item.(map[string]any)
		result[i] = types.InstallCommand{
			Command: cmd["command"].(string),
			Shell:   cmd["shell"].(string),
		}
	}

	log.Info("Converted to install command slice", "count", len(result))
	return result
}
