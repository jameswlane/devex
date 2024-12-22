package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
)

// SetupConfig initializes the configuration for the application.
func SetupConfig(homeDir string) {
	viper.SetConfigType("yaml")

	// Define config paths
	localConfigPath := filepath.Join(homeDir, ".devex/config/config.yaml")
	defaultConfigPath := filepath.Join(homeDir, ".local/share/devex/config/config.yaml")

	// Load the first available config
	if err := loadFirstAvailableConfig(localConfigPath, defaultConfigPath); err != nil {
		log.Warn("Failed to load configuration", "error", err)
	}

	// Enable automatic environment variable binding
	viper.AutomaticEnv()
}

// loadFirstAvailableConfig attempts to load configuration from one of the given paths
func loadFirstAvailableConfig(paths ...string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			viper.SetConfigFile(path)
			if err := viper.ReadInConfig(); err == nil {
				log.Info("Successfully loaded config file", "path", path)
				return nil
			}
			return fmt.Errorf("error reading config file at %s: %v", path, err)
		}
	}
	return fmt.Errorf("no valid configuration file found")
}

// GetDefaults retrieves default configurations for a given key using Viper
func GetDefaults(configName string) ([]string, error) {
	var defaults []string
	if err := viper.UnmarshalKey(configName, &defaults); err != nil {
		return nil, fmt.Errorf("failed to unmarshal defaults for %s: %v", configName, err)
	}
	return defaults, nil
}
