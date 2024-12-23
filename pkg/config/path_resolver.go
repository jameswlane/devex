package config

import (
	"log"
	"os"
	"path/filepath"
)

// GetMigrationsPath resolves the migrations directory path based on the environment.
func GetMigrationsPath() (string, error) {
	// Check if an environment variable is set for the migrations path
	if envPath := os.Getenv("MIGRATIONS_PATH"); envPath != "" {
		log.Printf("Using migrations path from environment variable: %s", envPath)
		return envPath, nil
	}

	// Default to the production path
	prodPath := filepath.Join(os.Getenv("HOME"), ".local/share/devex/migrations")
	if _, err := os.Stat(prodPath); err == nil {
		log.Printf("Using production migrations path: %s", prodPath)
		return prodPath, nil
	} else {
		log.Printf("Production migrations path not found: %s", prodPath)
	}

	// Fallback: Check for a local development path
	devPath := filepath.Join(".", "migrations")
	if _, err := os.Stat(devPath); err == nil {
		log.Printf("Using local development migrations path: %s", devPath)
		return devPath, nil
	} else {
		log.Printf("Local development migrations path not found: %s", devPath)
	}

	log.Println("No migrations path found")
	return "", os.ErrNotExist
}
