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
)

type SchemaRepository interface {
	GetVersion() (int, error)
	SetVersion(version int) error
	ApplyMigrations(migrationsDir string) error
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
	err := r.db.QueryRow(query).Scan(&version)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil // No version found
	} else if err != nil {
		return 0, fmt.Errorf("failed to fetch schema version: %v", err)
	}
	return version, nil
}

func (r *schemaRepository) SetVersion(version int) error {
	query := `
		INSERT INTO schema_migrations (version, applied_at)
		VALUES (?, CURRENT_TIMESTAMP)`
	_, err := r.db.Exec(query, version)
	if err != nil {
		return fmt.Errorf("failed to set schema version: %v", err)
	}
	return nil
}

func (r *schemaRepository) ApplyMigrations(migrationsDir string) error {
	currentVersion, err := r.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %v", err)
	}

	migrations, err := loadMigrations(migrationsDir, "up")
	if err != nil {
		return fmt.Errorf("failed to load migrations: %v", err)
	}

	sort.Ints(migrations) // Ensure migrations are applied in order
	for _, version := range migrations {
		if version > currentVersion {
			if err := applyMigration(r.db, migrationsDir, version, "up"); err != nil {
				return fmt.Errorf("failed to apply migration version %d: %v", version, err)
			}
			if err := r.SetVersion(version); err != nil {
				return fmt.Errorf("failed to update schema version to %d: %v", version, err)
			}
		}
	}

	return nil
}

func (r *schemaRepository) RollbackMigrations(migrationsDir string, targetVersion int) error {
	currentVersion, err := r.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %v", err)
	}

	if targetVersion >= currentVersion {
		return fmt.Errorf("target version (%d) must be less than current version (%d)", targetVersion, currentVersion)
	}

	migrations, err := loadMigrations(migrationsDir, "down")
	if err != nil {
		return fmt.Errorf("failed to load migrations: %v", err)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(migrations))) // Rollback in descending order
	for _, version := range migrations {
		if version > targetVersion && version <= currentVersion {
			if err := applyMigration(r.db, migrationsDir, version, "down"); err != nil {
				return fmt.Errorf("failed to rollback migration version %d: %v", version, err)
			}
			if err := r.SetVersion(version - 1); err != nil {
				return fmt.Errorf("failed to update schema version to %d: %v", version-1, err)
			}
		}
	}

	return nil
}

// loadMigrations loads migration file names (up or down) and parses version numbers
func loadMigrations(dir, direction string) ([]int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %v", err)
	}

	var migrations []int
	for _, file := range files {
		if strings.HasSuffix(file.Name(), fmt.Sprintf(".%s.sql", direction)) {
			version, err := strconv.Atoi(strings.Split(file.Name(), "_")[0])
			if err != nil {
				return nil, fmt.Errorf("invalid migration file format: %s", file.Name())
			}
			migrations = append(migrations, version)
		}
	}
	return migrations, nil
}

// applyMigration applies an individual migration (up or down)
func applyMigration(db *sql.DB, dir string, version int, direction string) error {
	fileName := filepath.Join(dir, fmt.Sprintf("%03d_%s.sql", version, direction))
	query, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %v", fileName, err)
	}

	_, err = db.Exec(string(query))
	if err != nil {
		return fmt.Errorf("failed to execute migration %d (%s): %v", version, direction, err)
	}

	fmt.Printf("Successfully applied migration %d (%s)\n", version, direction)
	return nil
}
