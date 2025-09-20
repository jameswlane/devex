package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

type DB struct {
	*sql.DB
}

// Exec conforms to the `types.Database` interface.
func (d *DB) Exec(query string, args ...any) error {
	ctx := context.Background()
	_, err := d.ExecContext(ctx, query, args...)
	return err
}

// QueryRow conforms to the `types.Database` interface.
func (d *DB) QueryRow(query string, args ...any) (map[string]any, error) {
	ctx := context.Background()
	row := d.QueryRowContext(ctx, query, args...)

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
	ctx := context.Background()
	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Error("Failed to close rows", err)
		}
	}(rows)

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve columns: %w", err)
	}
	return columns, nil
}

// ValidateConnection pings the database to ensure the connection is active.
func (d *DB) ValidateConnection() error {
	log.Info("Validating database connection")
	ctx := context.Background()
	if err := d.PingContext(ctx); err != nil {
		log.Error("Database connection validation failed", err)
		return fmt.Errorf("failed to validate database connection: %w", err)
	}
	log.Info("Database connection validated successfully")
	return nil
}
