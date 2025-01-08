package db

import (
	"database/sql"
	"fmt"

	"github.com/jameswlane/devex/pkg/log"
)

type DB struct {
	*sql.DB
}

// Exec conforms to the `types.Database` interface.
func (d *DB) Exec(query string, args ...any) error {
	_, err := d.DB.Exec(query, args...)
	return err
}

// QueryRow conforms to the `types.Database` interface.
func (d *DB) QueryRow(query string, args ...any) (map[string]any, error) {
	row := d.DB.QueryRow(query, args...)

	// Get column names
	columns, err := d.GetColumns(query, args...)
	if err != nil {
		log.Error("Failed to get columns for QueryRow", err)
		return nil, err
	}

	// Scan values into a map
	values := make([]any, len(columns))
	valuePointers := make([]any, len(columns))
	for i := range values {
		valuePointers[i] = &values[i]
	}

	if err := row.Scan(valuePointers...); err != nil {
		log.Error("Failed to scan row for QueryRow", err)
		return nil, err
	}

	// Convert to map
	result := make(map[string]any)
	for i, col := range columns {
		result[col] = values[i]
	}

	return result, nil
}

// GetColumns retrieves column names for the query.
func (d *DB) GetColumns(query string, args ...any) ([]string, error) {
	rows, err := d.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve columns: %w", err)
	}
	return columns, nil
}

// ValidateConnection pings the database to ensure the connection is active.
func (d *DB) ValidateConnection() error {
	log.Info("Validating database connection")
	if err := d.DB.Ping(); err != nil {
		log.Error("Database connection validation failed", err)
		return fmt.Errorf("failed to validate database connection: %w", err)
	}
	log.Info("Database connection validated successfully")
	return nil
}
