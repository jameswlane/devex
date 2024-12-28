package config

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/types"
)

// GetAppList retrieves the list of apps from the settings.
func GetAppList(settings Settings) ([]types.AppConfig, error) {
	return settings.Apps, nil
}

// ValidateApp checks the validity of an individual app.
func ValidateApp(app types.AppConfig) error {
	if app.Name == "" {
		return fmt.Errorf("app name is required")
	}
	if app.InstallMethod == "" {
		return fmt.Errorf("install method is required for app %s", app.Name)
	}
	// TODO: curlpipe doesn't have a install command
	//if app.InstallCommand == "" {
	//	return fmt.Errorf("install command is required for app %s", app.Name)
	//}
	return nil
}

// ListAppsByCategory filters apps by categories.
func ListAppsByCategory(settings Settings, categories []string) ([]types.AppConfig, error) {
	var filteredApps []types.AppConfig
	for _, app := range settings.Apps {
		for _, category := range categories {
			if app.Category == category {
				filteredApps = append(filteredApps, app)
			}
		}
	}
	return filteredApps, nil
}

// FindAppByName retrieves an app by its name from the settings.
func FindAppByName(settings Settings, name string) (*types.AppConfig, error) {
	for _, app := range settings.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, fmt.Errorf("app not found: %s", name)
}

// GetAppInfo retrieves the AppConfig for a given install_command or name.
func GetAppInfo(identifier string) (*types.AppConfig, error) {
	// List of sections to search
	sections := []string{"apps", "databases", "optional_apps", "programming_languages"}

	for _, section := range sections {
		for _, app := range viper.Get(section).([]any) {
			candidate := app.(map[string]any)
			if candidate["install_command"] == identifier || candidate["name"] == identifier {
				return &types.AppConfig{
					Name:           candidate["name"].(string),
					Description:    candidate["description"].(string),
					InstallMethod:  candidate["install_method"].(string),
					InstallCommand: candidate["install_command"].(string),
					DownloadURL:    candidate["download_url"].(string),
					Dependencies:   toStringSlice(candidate["dependencies"]),
					PostInstall:    toInstallCommandSlice(candidate["post_install"]),
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no app configuration found for identifier: %s", identifier)
}

// Helper to convert interface{} to []string
func toStringSlice(input any) []string {
	if input == nil {
		return nil
	}
	result := []string{}
	for _, item := range input.([]any) {
		result = append(result, item.(string))
	}
	return result
}

// Helper to convert interface{} to []types.InstallCommand
func toInstallCommandSlice(input any) []types.InstallCommand {
	if input == nil {
		return nil
	}
	result := []types.InstallCommand{}
	for _, item := range input.([]any) {
		cmd := item.(map[string]any)
		result = append(result, types.InstallCommand{
			Command: cmd["command"].(string),
			Shell:   cmd["shell"].(string),
		})
	}
	return result
}
