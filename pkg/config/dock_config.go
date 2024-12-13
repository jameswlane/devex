package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// DockConfig represents the structure of the dock.yaml config
type DockConfig struct {
	Favorites []string `yaml:"favorites"`
}

// LoadDockConfig loads the dock configuration from a custom or default path
func LoadDockConfig(defaultPath string) (DockConfig, error) {
	// Define the custom path (e.g., ~/.devex/config/dock.yaml)
	customPath := filepath.Join(os.Getenv("HOME"), ".devex/config/dock.yaml")

	// Use the helper function to load custom or default config
	data, err := LoadCustomOrDefault(defaultPath, customPath)
	if err != nil {
		return DockConfig{}, fmt.Errorf("failed to load dock config: %v", err)
	}

	var config DockConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return DockConfig{}, fmt.Errorf("failed to unmarshal dock config: %v", err)
	}

	return config, nil
}
