package datastore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/log"
)

func ApplySchemaUpdates(repo repository.SchemaRepository, homeDir string) error {
	log.Info("Starting schema updates")

	// Construct the migrations directory path
	migrationsDir := filepath.Join(homeDir, ".local/share/devex/migrations")
	log.Info("Migrations directory", "path", migrationsDir)

	// Initialize the schema_migrations table if it does not exist
	initializeTableQuery := `
  CREATE TABLE IF NOT EXISTS schema_migrations (
   version INTEGER PRIMARY KEY,
   applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
  );`
	log.Info("Ensuring schema_migrations table exists")
	if err := repo.ApplyMigrations(initializeTableQuery); err != nil {
		log.Error("Failed to ensure schema_migrations table exists", "error", err)
		return fmt.Errorf("failed to ensure schema_migrations table exists: %v", err)
	}

	// Initialize schema version if not present
	currentVersion, err := repo.GetVersion()
	if err != nil {
		log.Error("Failed to get current schema version", "error", err)
		return fmt.Errorf("failed to get current schema version: %v", err)
	}
	log.Info("Current schema version retrieved", "version", currentVersion)

	if currentVersion == 0 {
		log.Info("Initializing schema version to 0")
		if err := repo.SetVersion(0); err != nil {
			log.Error("Failed to initialize schema version to 0", "error", err)
			return fmt.Errorf("failed to initialize schema version to 0: %v", err)
		}
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Error("Failed to read migrations directory", "error", err)
		return fmt.Errorf("failed to read migrations directory: %v", err)
	}
	log.Info("Migrations directory read successfully")

	var migrations []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), "_up.sql") {
			migrations = append(migrations, file.Name())
		}
	}
	log.Info("Migration files loaded", "files", migrations)

	sort.Strings(migrations) // Ensure migrations are applied in order

	for _, migration := range migrations {
		versionStr := strings.Split(migration, "_")[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			log.Error("Invalid migration file name", "file", migration, "error", err)
			return fmt.Errorf("invalid migration file name: %s", migration)
		}

		if version > currentVersion {
			log.Info("Applying schema update", "version", version)
			query, err := os.ReadFile(filepath.Join(migrationsDir, migration))
			if err != nil {
				log.Error("Failed to read migration file", "file", migration, "error", err)
				return fmt.Errorf("failed to read migration file %s: %v", migration, err)
			}

			log.Info("Executing migration", "version", version)
			if err := repo.ApplyMigrations(string(query)); err != nil {
				log.Error("Failed to apply schema update", "version", version, "error", err)
				return fmt.Errorf("failed to apply schema update for version %d: %v", version, err)
			}

			log.Info("Updating schema version", "version", version)
			if err := repo.SetVersion(version); err != nil {
				log.Error("Failed to update schema version", "version", version, "error", err)
				return fmt.Errorf("failed to update schema version to %d: %v", version, err)
			}
		}
	}

	log.Info("Database schema is up to date")
	return nil
}
