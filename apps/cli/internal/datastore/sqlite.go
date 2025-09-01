package datastore

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

type SQLite struct {
	conn *sql.DB
}

// NewSQLite initializes SQLite with schema and file path
func NewSQLite(dbPath string) (*SQLite, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	err = InitializeSchema(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &SQLite{conn: conn}, nil
}

// Conn returns the underlying SQL connection
func (s *SQLite) Conn() *sql.DB {
	return s.conn
}

// InitializeSchema ensures the required database schema is created
func InitializeSchema(conn *sql.DB) error {
	schema := `
    CREATE TABLE IF NOT EXISTS system_data (
        key TEXT PRIMARY KEY,
        value TEXT NOT NULL,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );
    CREATE TABLE IF NOT EXISTS installed_apps (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        app_name TEXT NOT NULL UNIQUE
    );
    CREATE TABLE IF NOT EXISTS schema_migrations (
        version INTEGER PRIMARY KEY,
        applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
    );`
	ctx := context.Background()
	_, err := conn.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	return nil
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
