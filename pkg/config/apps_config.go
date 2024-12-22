package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/viper"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/pkg/types"
)

// AppsConfig represents the list of apps in the YAML file
type AppsConfig struct {
	Apps []types.AppConfig `yaml:"apps"`
}

// In-memory cache for loaded configurations
var (
	configCache = make(map[string]any)
	cacheLock   sync.RWMutex
)

// LoadAppsConfig loads the apps configuration from a YAML file and validates it
func LoadAppsConfig() (AppsConfig, error) {
	var config AppsConfig
	err := viper.UnmarshalKey("apps", &config.Apps)
	if err != nil {
		return AppsConfig{}, fmt.Errorf("failed to unmarshal apps config: %v", err)
	}

	// Validate loaded apps
	for _, app := range config.Apps {
		if err := validateApp(app); err != nil {
			return AppsConfig{}, fmt.Errorf("invalid app configuration: %v", err)
		}
	}

	return config, nil
}

// validateApp checks that all required fields in an App struct are populated
func validateApp(app types.AppConfig) error {
	if app.Name == "" {
		return fmt.Errorf("app name is required")
	}
	if app.InstallMethod == "" {
		return fmt.Errorf("install method is required for app %s", app.Name)
	}
	if app.InstallCommand == "" {
		return fmt.Errorf("install command is required for app %s", app.Name)
	}
	return nil
}

// ListAppsByCategories lists all apps matching any of the specified categories
func (c *AppsConfig) ListAppsByCategories(categories []string) ([]types.AppConfig, error) {
	var appsInCategories []types.AppConfig
	var missingDeps []string

	for _, app := range c.Apps {
		// Check if app's category matches any of the given categories
		for _, category := range categories {
			if app.Category == category {
				appsInCategories = append(appsInCategories, app)

				// Validate dependencies
				for _, dep := range app.Dependencies {
					if dep == "" {
						missingDeps = append(missingDeps, fmt.Sprintf("App: %s, Missing Dependency: %s", app.Name, dep))
					}
				}
			}
		}
	}

	// Return error if missing dependencies are found
	if len(missingDeps) > 0 {
		return appsInCategories, fmt.Errorf("missing dependencies detected: %v", missingDeps)
	}

	return appsInCategories, nil
}

// loadYAMLWithCache loads a YAML file into the provided structure and caches the result
func loadYAMLWithCache(filePath string, out any) error {
	cacheLock.RLock()
	if cached, found := configCache[filePath]; found {
		cacheLock.RUnlock()
		*out.(*any) = cached
		return nil
	}
	cacheLock.RUnlock()

	// Load YAML from file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	err = yaml.Unmarshal(data, out)
	if err != nil {
		return fmt.Errorf("failed to parse YAML: %v", err)
	}

	// Cache the parsed configuration
	cacheLock.Lock()
	configCache[filePath] = out
	cacheLock.Unlock()

	return nil
}
