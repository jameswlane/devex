package datastore

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

// InitDB initializes the SQLite database and returns the custom DB type
func InitDB(dbPath string) (*DB, error) {
	// Get the directory path from the dbPath
	dbDir := filepath.Dir(dbPath)

	// Check if the directory exists, if not, create it
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		err := os.MkdirAll(dbDir, 0o755)
		if err != nil {
			return nil, fmt.Errorf("failed to create database directory: %v", err)
		}
	}

	// Open the database
	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create table if it doesn't exist
	createTableQuery := `
    CREATE TABLE IF NOT EXISTS installed_apps (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        app_name TEXT NOT NULL UNIQUE
    );`
	_, err = sqlDB.Exec(createTableQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %v", err)
	}

	// Wrap the sql.DB in the custom DB type
	return &DB{sqlDB}, nil
}

// AddInstalledApp adds an app to the installed_apps table
func AddInstalledApp(db *DB, appName string) error {
	insertQuery := `INSERT INTO installed_apps (app_name) VALUES (?)`
	_, err := db.Exec(insertQuery, appName)
	if err != nil {
		return fmt.Errorf("failed to insert app: %v", err)
	}
	return nil
}

// IsAppInDB checks if an app is already stored in the database
func IsAppInDB(db *DB, appName string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM installed_apps WHERE app_name = ? LIMIT 1)`
	err := db.QueryRow(query, appName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check app existence: %v", err)
	}
	return exists, nil
}

// RemoveInstalledApp removes an app from the installed_apps table
func RemoveInstalledApp(db *DB, appName string) error {
	deleteQuery := `DELETE FROM installed_apps WHERE app_name = ?`
	_, err := db.Exec(deleteQuery, appName)
	if err != nil {
		return fmt.Errorf("failed to delete app: %v", err)
	}
	return nil
}
