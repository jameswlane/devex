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
