package datastore

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // SQLite driver
)

type SQLite struct {
	conn *sql.DB
}

// NewSQLite opens and migrates a database using a background context.
func NewSQLite(dbPath string) (*SQLite, error) {
	return NewSQLiteContext(context.Background(), dbPath)
}

// NewSQLiteContext validates and migrates the database before making it available.
func NewSQLiteContext(ctx context.Context, dbPath string) (*SQLite, error) {
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database %q: %w", dbPath, err)
	}
	// Keep in-memory databases and connection-local SQLite state consistent.
	conn.SetMaxOpenConns(1)
	if err := ApplySchemaUpdates(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("initialize SQLite database %q: %w; back up the database and restore a compatible backup or use a CLI supporting its schema", dbPath, err)
	}
	return &SQLite{conn: conn}, nil
}

// Conn returns the underlying SQL connection.
func (s *SQLite) Conn() *sql.DB {
	return s.conn
}

// InitializeSchema validates and applies the embedded schema to an existing connection.
func InitializeSchema(conn *sql.DB) error {
	return ApplySchemaUpdates(context.Background(), conn)
}

// Exec executes a query without returning rows (e.g., INSERT, UPDATE, DELETE).
func (s *SQLite) Exec(query string, args ...any) error {
	ctx := context.Background()
	_, err := s.conn.ExecContext(ctx, query, args...)
	return err
}

// QueryRow retrieves a single row result as a map[string]interface{}.
func (s *SQLite) QueryRow(query string, args ...any) *sql.Row {
	ctx := context.Background()
	return s.conn.QueryRowContext(ctx, query, args...)
}

// Query executes a query and returns rows for further processing.
func (s *SQLite) Query(query string, args ...any) (*sql.Rows, error) {
	ctx := context.Background()
	return s.conn.QueryContext(ctx, query, args...)
}

// RowsToMaps converts sql.Rows to a slice of map[string]interface{} for convenience.
func RowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(rows)

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		row := make(map[string]any)
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}

		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}

		for i, col := range columns {
			if ptr, ok := pointers[i].(*any); ok {
				row[col] = *ptr
			} else {
				row[col] = nil
			}
		}

		results = append(results, row)
	}

	return results, nil
}

// Close closes the database connection.
func (s *SQLite) Close() error {
	return s.conn.Close()
}
