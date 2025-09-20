package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// LoadConfigs loads and merges all configuration files from the default and override directories.
func LoadConfigs(homeDir string, files []string) (*viper.Viper, error) {
	v := viper.New()

	// Default configuration path
	defaultConfigPath := filepath.Join(homeDir, ".local/share/devex/config")
	overrideConfigPath := filepath.Join(homeDir, ".devex/config")

	// Load default configurations
	for _, file := range files {
		defaultPath := filepath.Join(defaultConfigPath, file)
		if err := mergeConfigFile(v, defaultPath); err != nil {
			log.Warn("Failed to load default config; skipping", "file", file, "error", err)
		}
	}

	// Apply overrides
	for _, file := range files {
		overridePath := filepath.Join(overrideConfigPath, file)
		if _, err := fs.Stat(overridePath); err == nil {
			log.Info("Applying override", "file", overridePath)
			if err := mergeConfigFile(v, overridePath); err != nil {
				log.Warn("Failed to apply override; skipping", "file", overridePath, "error", err)
			}
		}
	}

	// Bind environment variables
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	return v, nil
}

// mergeConfigFile reads a YAML file and merges it into the Viper instance.
func mergeConfigFile(v *viper.Viper, path string) error {
	data, err := fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn("Config file not found", "path", path)
			return nil
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	subViper := viper.New()
	subViper.SetConfigType("yaml")
	if err := subViper.ReadConfig(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("failed to parse YAML file %s: %w", path, err)
	}

	for k, value := range subViper.AllSettings() { // Rename loop variable
		v.Set(k, value) // Use the viper instance for setting values
	}

	return nil
}
