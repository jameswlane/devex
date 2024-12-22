package datastore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jameswlane/devex/pkg/datastore/repository"
)

const migrationsDir = "migrations"

func ApplySchemaUpdates(repo repository.SchemaRepository) error {
	// Initialize the schema_migrations table if it does not exist
	initializeTableQuery := `
    CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );`
	if err := repo.ApplyMigrations(initializeTableQuery); err != nil {
		return fmt.Errorf("failed to ensure schema_migrations table exists: %v", err)
	}

	// Initialize schema version if not present
	if currentVersion, err := repo.GetVersion(); err == nil && currentVersion == 0 {
		if err := repo.SetVersion(0); err != nil {
			return fmt.Errorf("failed to initialize schema version to 0: %v", err)
		}
	}

	currentVersion, err := repo.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %v", err)
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %v", err)
	}

	var migrations []string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			migrations = append(migrations, file.Name())
		}
	}

	sort.Strings(migrations) // Ensure migrations are applied in order

	for _, migration := range migrations {
		version, err := strconv.Atoi(strings.TrimSuffix(migration, ".sql"))
		if err != nil {
			return fmt.Errorf("invalid migration file name: %s", migration)
		}

		if version > currentVersion {
			query, err := os.ReadFile(filepath.Join(migrationsDir, migration))
			if err != nil {
				return fmt.Errorf("failed to read migration file %s: %v", migration, err)
			}

			fmt.Printf("Applying schema update to version %d...\n", version)
			if err := repo.ApplyMigrations(string(query)); err != nil {
				return fmt.Errorf("failed to apply schema update for version %d: %v", version, err)
			}

			if err := repo.SetVersion(version); err != nil {
				return fmt.Errorf("failed to update schema version to %d: %v", version, err)
			}
		}
	}

	fmt.Println("Database schema is up to date.")
	return nil
}
