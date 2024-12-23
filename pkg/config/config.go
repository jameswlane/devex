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
	log.Info("Starting SetupConfig", "homeDir", homeDir)
	viper.SetConfigType("yaml")

	// Define config paths
	localConfigPath := filepath.Join(homeDir, ".devex/config/config.yaml")
	defaultConfigPath := filepath.Join(homeDir, ".local/share/devex/config/config.yaml")
	log.Info("Config paths defined", "localConfigPath", localConfigPath, "defaultConfigPath", defaultConfigPath)

	// Load the first available config
	if err := loadFirstAvailableConfig(localConfigPath, defaultConfigPath); err != nil {
		log.Warn("Failed to load configuration", "error", err)
	} else {
		log.Info("Configuration loaded successfully")
	}

	// Enable automatic environment variable binding
	viper.AutomaticEnv()
	log.Info("Automatic environment variable binding enabled")
}

// loadFirstAvailableConfig attempts to load configuration from one of the given paths
func loadFirstAvailableConfig(paths ...string) error {
	log.Info("Starting loadFirstAvailableConfig", "paths", paths)
	for _, path := range paths {
		log.Info("Checking config path", "path", path)
		if _, err := os.Stat(path); err == nil {
			log.Info("Config file found", "path", path)
			viper.SetConfigFile(path)
			if err := viper.ReadInConfig(); err == nil {
				log.Info("Successfully loaded config file", "path", path)
				return nil
			}
			log.Error("Error reading config file", "path", path, "error", err)
			return fmt.Errorf("error reading config file at %s: %v", path, err)
		} else {
			log.Warn("Config file not found", "path", path, "error", err)
		}
	}
	log.Error("No valid configuration file found")
	return fmt.Errorf("no valid configuration file found")
}

// GetDefaults retrieves default configurations for a given key using Viper
func GetDefaults(configName string) ([]string, error) {
	log.Info("Starting GetDefaults", "configName", configName)
	var defaults []string
	if err := viper.UnmarshalKey(configName, &defaults); err != nil {
		log.Error("Failed to unmarshal defaults", "configName", configName, "error", err)
		return nil, fmt.Errorf("failed to unmarshal defaults for %s: %v", configName, err)
	}
	log.Info("Defaults retrieved successfully", "configName", configName, "defaults", defaults)
	return defaults, nil
}
