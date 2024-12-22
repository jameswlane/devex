package config

import (
	"os"
	"path/filepath"
)

// GetMigrationsPath resolves the migrations directory path based on the environment.
func GetMigrationsPath() (string, error) {
	// Check if an environment variable is set for the migrations path
	if envPath := os.Getenv("MIGRATIONS_PATH"); envPath != "" {
		return envPath, nil
	}

	// Default to the production path
	prodPath := filepath.Join(os.Getenv("HOME"), ".local/share/devex/migrations")
	if _, err := os.Stat(prodPath); err == nil {
		return prodPath, nil
	}

	// Fallback: Check for a local development path
	devPath := filepath.Join(".", "migrations")
	if _, err := os.Stat(devPath); err == nil {
		return devPath, nil
	}

	return "", os.ErrNotExist
}
