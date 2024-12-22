package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// LoadCustomOrDefaultFile loads a custom file from ~/.devex if available, otherwise defaults to the provided path
func LoadCustomOrDefaultFile(defaultPath, assetType string) (string, error) {
	customPath := filepath.Join(os.Getenv("HOME"), ".devex", assetType, filepath.Base(defaultPath))

	// Check if the user has a custom file
	if _, err := os.Stat(customPath); err == nil {
		return customPath, nil
	}

	// Otherwise, fallback to the provided default path
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, nil
	}

	return "", fmt.Errorf("no valid configuration file found")
}

// LoadYAMLConfig loads a YAML file into the provided structure
func LoadYAMLConfig(filePath string, out any) error {
	return loadYAMLWithCache(filePath, out)
}
