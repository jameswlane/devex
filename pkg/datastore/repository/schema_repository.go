package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jameswlane/devex/pkg/log"
)

type SchemaRepository interface {
	GetVersion() (int, error)
	SetVersion(version int) error
	ApplyMigrations(query string) error
	RollbackMigrations(migrationsDir string, targetVersion int) error
}

type schemaRepository struct {
	db *sql.DB
}

func NewSchemaRepository(db *sql.DB) SchemaRepository {
	return &schemaRepository{db: db}
}

func (r *schemaRepository) GetVersion() (int, error) {
	var version int
	query := "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1"
	log.Info("Executing query to get schema version", "query", query)
	err := r.db.QueryRow(query).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		log.Info("No schema version found, returning 0")
		return 0, nil // No version found
	} else if err != nil {
		log.Error("Failed to fetch schema version", "error", err)
		return 0, fmt.Errorf("failed to fetch schema version: %v", err)
	}
	log.Info("Schema version retrieved", "version", version)
	return version, nil
}

func (r *schemaRepository) SetVersion(version int) error {
	query := `
		INSERT OR REPLACE INTO schema_migrations (version, applied_at)
		VALUES (?, CURRENT_TIMESTAMP)`
	log.Info("Executing query to set schema version", "query", query, "version", version)
	_, err := r.db.Exec(query, version)
	if err != nil {
		log.Error("Failed to set schema version", "error", err)
		return fmt.Errorf("failed to set schema version: %v", err)
	}
	log.Info("Schema version set successfully", "version", version)
	return nil
}

func (r *schemaRepository) ApplyMigrations(query string) error {
	log.Info("Applying migration", "query", query)
	_, err := r.db.Exec(query)
	if err != nil {
		log.Error("Failed to apply migration", "error", err)
		return fmt.Errorf("failed to apply migration: %v", err)
	}
	log.Info("Migration applied successfully")
	return nil
}

func (r *schemaRepository) RollbackMigrations(migrationsDir string, targetVersion int) error {
	log.Info("Starting to rollback migrations", "migrationsDir", migrationsDir, "targetVersion", targetVersion)
	currentVersion, err := r.GetVersion()
	if err != nil {
		log.Error("Failed to get current schema version", "error", err)
		return fmt.Errorf("failed to get current schema version: %v", err)
	}

	if targetVersion >= currentVersion {
		log.Error("Target version must be less than current version", "targetVersion", targetVersion, "currentVersion", currentVersion)
		return fmt.Errorf("target version (%d) must be less than current version (%d)", targetVersion, currentVersion)
	}

	migrations, err := loadMigrations(migrationsDir, "down")
	if err != nil {
		log.Error("Failed to load migrations", "error", err)
		return fmt.Errorf("failed to load migrations: %v", err)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(migrations))) // Rollback in descending order
	for _, version := range migrations {
		if version > targetVersion && version <= currentVersion {
			log.Info("Rolling back migration", "version", version)
			if err := applyMigration(r.db, migrationsDir, version, "down"); err != nil {
				log.Error("Failed to rollback migration", "version", version, "error", err)
				return fmt.Errorf("failed to rollback migration version %d: %v", version, err)
			}
			if err := r.SetVersion(version - 1); err != nil {
				log.Error("Failed to update schema version", "version", version-1, "error", err)
				return fmt.Errorf("failed to update schema version to %d: %v", version-1, err)
			}
		}
	}

	log.Info("Migrations rolled back successfully")
	return nil
}

func loadMigrations(dir, direction string) ([]int, error) {
	log.Info("Loading migrations", "dir", dir, "direction", direction)
	files, err := os.ReadDir(dir)
	if err != nil {
		log.Error("Failed to read migrations directory", "error", err)
		return nil, fmt.Errorf("failed to read migrations directory: %v", err)
	}

	var migrations []int
	for _, file := range files {
		if strings.HasSuffix(file.Name(), fmt.Sprintf(".%s.sql", direction)) {
			version, err := strconv.Atoi(strings.Split(file.Name(), "_")[0])
			if err != nil {
				log.Error("Invalid migration file format", "file", file.Name(), "error", err)
				return nil, fmt.Errorf("invalid migration file format: %s", file.Name())
			}
			migrations = append(migrations, version)
		}
	}
	log.Info("Migrations loaded", "migrations", migrations)
	return migrations, nil
}

func applyMigration(db *sql.DB, dir string, version int, direction string) error {
	fileName := filepath.Join(dir, fmt.Sprintf("%03d_%s.sql", version, direction))
	log.Info("Applying migration", "fileName", fileName, "version", version, "direction", direction)
	query, err := os.ReadFile(fileName)
	if err != nil {
		log.Error("Failed to read migration file", "fileName", fileName, "error", err)
		return fmt.Errorf("failed to read migration file %s: %v", fileName, err)
	}

	_, err = db.Exec(string(query))
	if err != nil {
		log.Error("Failed to execute migration", "version", version, "direction", direction, "error", err)
		return fmt.Errorf("failed to execute migration %d (%s): %v", version, direction, err)
	}

	log.Info("Successfully applied migration", "version", version, "direction", direction)
	return nil
}
