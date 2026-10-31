package commands

import (
	"errors"
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

// writeNewApplicationConfig reserves the path so a filename collision cannot replace an application.
func writeNewApplicationConfig(app types.CrossPlatformApp, path string) error {
	data, err := yaml.Marshal(app)
	if err != nil {
		return fmt.Errorf("marshal application %q: %w", app.Name, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("create configuration directory for application %q at %s: %w", app.Name, path, err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return fmt.Errorf("cannot save application %q: configuration path %s already exists; rename the existing file or change the application's name or category, then retry", app.Name, path)
	}
	if err != nil {
		return fmt.Errorf("create application %q at %s: %w", app.Name, path, err)
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		// A failed write must not leave an incomplete definition that a retry could load.
		return fmt.Errorf("write application %q at %s: %w", app.Name, path, errors.Join(err, os.Remove(path)))
	}
	return nil
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
		if err := writeNewApplicationConfig(app, path); err != nil {
			return fmt.Errorf("migrate applications from %s (legacy file retained): %w", legacy, err)
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
