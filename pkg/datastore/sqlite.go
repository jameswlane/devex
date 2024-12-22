package datastore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	"github.com/jameswlane/devex/pkg/db"
)

type DB struct {
	*sql.DB
}

// DB returns the wrapped *db.DB instance
func (d *DB) GetDB() *db.DB {
	return &db.DB{DB: d.DB}
}

// InitDB initializes the SQLite database and returns the custom DB type
func InitDB(dbPath string) (*DB, error) {
	// Get the directory path from the dbPath
	dbDir := filepath.Dir(dbPath)

	// Check if the directory exists, if not, create it
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %v", err)
		}
	}

	// Open the database
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create tables
	tables := []string{
		`CREATE TABLE IF NOT EXISTS system_data (
            key TEXT PRIMARY KEY,
            value TEXT NOT NULL,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS installed_apps (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            app_name TEXT NOT NULL UNIQUE
        );`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
            version INTEGER PRIMARY KEY,
            applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
	}

	for _, query := range tables {
		if _, err := sqlDB.Exec(query); err != nil {
			return nil, fmt.Errorf("failed to create table: %v", err)
		}
	}

	// Initialize schema_migrations if not present
	initSchemaVersion := `
    INSERT INTO schema_migrations (version, applied_at)
    SELECT 0, CURRENT_TIMESTAMP
    WHERE NOT EXISTS (SELECT 1 FROM schema_migrations);`
	if _, err := sqlDB.Exec(initSchemaVersion); err != nil {
		return nil, fmt.Errorf("failed to initialize schema version: %v", err)
	}

	// Wrap the sql.DB in the custom DB type
	return &DB{sqlDB}, nil
}
