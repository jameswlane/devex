package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func configuredEditableApplications(settings config.CrossPlatformSettings) ([]types.AppConfig, error) {
	apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
	if err != nil {
		return nil, fmt.Errorf("read configured applications: %w", err)
	}
	result := make([]types.AppConfig, 0, len(apps))
	for _, app := range apps {
		legacy := app.ToLegacyAppConfig()
		// Removal affects every platform, so retain dependency constraints from each one.
		for _, platform := range []types.OSConfig{app.AllPlatforms, app.Linux, app.MacOS, app.Windows} {
			for _, dependency := range platform.Dependencies {
				present := false
				for _, existing := range legacy.Dependencies {
					if strings.EqualFold(existing, dependency) {
						present = true
					}
				}
				if !present {
					legacy.Dependencies = append(legacy.Dependencies, dependency)
				}
			}
		}
		result = append(result, legacy)
	}
	return result, nil
}

// migrateApplicationConfig preserves platform definitions when replacing a legacy file.
func migrateApplicationConfig(dir string) error {
	legacy := filepath.Join(dir, "applications.yaml")
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	apps, err := config.LoadApplicationConfigs(dir)
	if err != nil {
		return err
	}
	existing, err := (config.FileApplicationLoader{}).LoadApplications([]string{dir})
	if err != nil {
		return err
	}
	existingNames := make(map[string]bool)
	for _, app := range existing {
		existingNames[strings.ToLower(app.Name)] = true
	}
	for _, app := range apps {
		if existingNames[strings.ToLower(app.Name)] {
			continue
		}
		path, err := applicationConfigPath(dir, "applications", app.Category, app.Name)
		if err != nil {
			return err
		}
		if err := writeYAMLConfig(app, path, ""); err != nil {
			return err
		}
	}
	if err := os.Remove(legacy); err != nil {
		return fmt.Errorf("remove migrated application config %s: %w", legacy, err)
	}
	return nil
}

func deleteApplicationConfig(dir, name string) error {
	if err := migrateApplicationConfig(dir); err != nil {
		return err
	}
	files, err := config.ConfigurationFiles(dir)
	if err != nil {
		return err
	}
	found := false
	for _, relative := range files {
		if !strings.HasPrefix(filepath.ToSlash(relative), "applications/") {
			continue
		}
		path := filepath.Join(dir, relative)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var app types.CrossPlatformApp
		if err := yaml.Unmarshal(data, &app); err != nil {
			return fmt.Errorf("parse application %s: %w", path, err)
		}
		if strings.EqualFold(app.Name, name) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove application %s: %w", path, err)
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("application %q not found in configuration", name)
	}
	return nil
}
