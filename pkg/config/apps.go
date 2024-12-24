package config

import (
	"fmt"

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
