package datastore

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func ApplySchemaUpdates(repo types.SchemaRepository, homeDir string) error {
	log.Info("Starting schema updates")

	// Construct the migrations directory path
	migrationsDir := filepath.Join(homeDir, ".local/share/devex/migrations")
	log.Info("Migrations directory path resolved", "path", migrationsDir)

	// Initialize the schema_migrations table if it does not exist
	initializeTableQuery := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	log.Info("Ensuring schema_migrations table exists")
	if err := repo.ApplyMigrations(initializeTableQuery); err != nil {
		log.Error("Failed to ensure schema_migrations table exists", err)
		return fmt.Errorf("failed to ensure schema_migrations table exists: %w", err)
	}

	// Retrieve the current schema version
	currentVersion, err := repo.GetVersion()
	if err != nil {
		log.Error("Failed to get current schema version", err)
		return fmt.Errorf("failed to get current schema version: %w", err)
	}
	log.Info("Current schema version retrieved", "version", currentVersion)

	// Read the migrations directory
	files, err := fs.ReadDir(migrationsDir)
	if err != nil {
		log.Error("Failed to read migrations directory", err)
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}
	log.Info("Migrations directory read successfully", "count", len(files))

	// Collect migration files
	var migrations []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), "_up.sql") {
			migrations = append(migrations, file.Name())
		}
	}
	log.Info("Migration files loaded", "files", migrations)

	// Sort migrations in ascending order
	sort.Strings(migrations)

	// Apply migrations in order
	for _, migration := range migrations {
		versionStr := strings.Split(migration, "_")[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			log.Error("Invalid migration file name", err, "file", migration)
			return fmt.Errorf("invalid migration file name: %s", migration)
		}

		if version > currentVersion {
			log.Info("Applying schema update", "version", version)
			query, err := fs.ReadFile(filepath.Join(migrationsDir, migration))
			if err != nil {
				log.Error("Failed to read migration file", err, "file", migration)
				return fmt.Errorf("failed to read migration file %s: %w", migration, err)
			}

			// Execute the migration query
			log.Info("Executing migration", "version", version)
			if err := repo.ApplyMigrations(string(query)); err != nil {
				log.Error("Failed to apply schema update", err, "version", version)
				return fmt.Errorf("failed to apply schema update for version %d: %w", version, err)
			}

			// Update the schema version
			log.Info("Updating schema version", "version", version)
			if err := repo.SetVersion(version); err != nil {
				log.Error("Failed to update schema version", err, "version", version)
				return fmt.Errorf("failed to update schema version to %d: %w", version, err)
			}
		}
	}

	log.Info("Database schema is up to date")
	return nil
}
