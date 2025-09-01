package repository

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type schemaRepository struct {
	db types.Database
}

func NewSchemaRepository(db types.Database) types.SchemaRepository {
	return &schemaRepository{db: db}
}

func (r *schemaRepository) GetVersion() (int, error) {
	var version int
	query := "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1"
	log.Info("Getting schema version", "query", query)

	result := r.db.QueryRow(query)
	err := result.Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			// No migrations have been applied yet, return version 0
			return 0, nil
		}
		return 0, fmt.Errorf("failed to scan result: %w", err)
	}
	return version, nil
}

func (r *schemaRepository) SetVersion(version int) error {
	query := `
		INSERT OR REPLACE INTO schema_migrations (version, applied_at)
		VALUES (?, CURRENT_TIMESTAMP)`
	log.Info("Setting schema version", "query", query, "version", version)

	err := r.db.Exec(query, version)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	log.Info("Schema version set successfully", "version", version)
	return nil
}

func (r *schemaRepository) ApplyMigrations(query string) error {
	log.Info("Applying migration query", "query", query)

	err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	log.Info("Migration query applied successfully")
	return nil
}

func (r *schemaRepository) RollbackMigrations(migrationsDir string, targetVersion int) error {
	log.Info("Rolling back migrations", "migrationsDir", migrationsDir, "targetVersion", targetVersion)

	currentVersion, err := r.GetVersion()
	if err != nil {
		log.Error("Failed to get current schema version", err)
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	if targetVersion >= currentVersion {
		log.Error("Invalid rollback target version", fmt.Errorf("invalid target version: %d", targetVersion), "targetVersion", targetVersion)
		return fmt.Errorf("target version (%d) must be less than current version (%d)", targetVersion, currentVersion)
	}

	migrations, err := loadMigrations(migrationsDir, "down")
	if err != nil {
		log.Error("Failed to load rollback migrations", err)
		return fmt.Errorf("failed to load rollback migrations: %w", err)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(migrations))) // Rollback in descending order
	for _, version := range migrations {
		if version > targetVersion && version <= currentVersion {
			log.Info("Rolling back migration", "version", version)
			if err := applyMigration(r.db, migrationsDir, version, "down"); err != nil {
				log.Error("Failed to rollback migration", err, "version", version)
				return fmt.Errorf("failed to rollback migration version %d: %w", version, err)
			}
			if err := r.SetVersion(version - 1); err != nil {
				log.Error("Failed to update schema version after rollback", err, "version", version-1)
				return fmt.Errorf("failed to update schema version to %d: %w", version-1, err)
			}
		}
	}

	log.Info("Migrations rolled back successfully")
	return nil
}

func loadMigrations(dir, direction string) ([]int, error) {
	log.Info("Loading migrations", "dir", dir, "direction", direction)

	files, err := fs.ReadDir(dir)
	if err != nil {
		log.Error("Failed to read migrations directory", err, "dir", dir)
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []int
	for _, file := range files {
		if strings.HasSuffix(file.Name(), fmt.Sprintf(".%s.sql", direction)) {
			version, err := strconv.Atoi(strings.Split(file.Name(), "_")[0])
			if err != nil {
				log.Error("Invalid migration file format", err, "file", file.Name())
				return nil, fmt.Errorf("invalid migration file format: %s", file.Name())
			}
			migrations = append(migrations, version)
		}
	}

	log.Info("Migrations loaded", "migrations", migrations)
	return migrations, nil
}

func applyMigration(db types.Database, dir string, version int, direction string) error {
	fileName := filepath.Join(dir, fmt.Sprintf("%03d_%s.sql", version, direction))
	log.Info("Applying migration", "fileName", fileName, "version", version, "direction", direction)

	query, err := fs.ReadFile(fileName)
	if err != nil {
		log.Error("Failed to read migration file", err, "fileName", fileName)
		return fmt.Errorf("failed to read migration file %s: %w", fileName, err)
	}

	err = db.Exec(string(query)) // Only handle the error
	if err != nil {
		log.Error("Failed to execute migration", err, "version", version, "direction", direction)
		return fmt.Errorf("failed to execute migration %d (%s): %w", version, direction, err)
	}

	log.Info("Migration applied successfully", "version", version, "direction", direction)
	return nil
}
