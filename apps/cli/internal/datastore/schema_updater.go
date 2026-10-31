package datastore

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"reflect"
	"strconv"
	"strings"

	"github.com/jameswlane/devex/apps/cli/migrations"
)

type schemaMigration struct {
	version int
	name    string
	query   string
}

// ApplySchemaUpdates applies pending embedded migrations and their version records
// in one transaction. Unsupported versions and schema drift fail without changes.
func ApplySchemaUpdates(ctx context.Context, conn *sql.DB) error {
	return applySchemaUpdates(ctx, conn, migrations.Files)
}

func applySchemaUpdates(ctx context.Context, conn *sql.DB, resources fs.FS) error {
	updates, err := loadSchemaMigrations(resources)
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema migration transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var integrity string
	if err := tx.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("check SQLite integrity: %w", err)
	}
	if integrity != "ok" {
		return fmt.Errorf("corrupt SQLite database: %s", integrity)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("initialize migration history: %w", err)
	}

	current, err := schemaVersion(ctx, tx, len(updates))
	if err != nil {
		return err
	}
	if err := validateSchema(ctx, tx, updates, current); err != nil {
		return fmt.Errorf("incompatible schema at version %d: %w", current, err)
	}
	for _, update := range updates[current:] {
		if _, err := tx.ExecContext(ctx, update.query); err != nil {
			return fmt.Errorf("apply migration %s: %w", update.name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", update.version); err != nil {
			return fmt.Errorf("record migration %s: %w", update.name, err)
		}
	}
	if err := validateSchema(ctx, tx, updates, len(updates)); err != nil {
		return fmt.Errorf("validate migrated schema: %w", err)
	}
	// Retain the original tracker for compatibility; schema_migrations is authoritative.
	if _, err := tx.ExecContext(ctx, "UPDATE schema_version SET version = ? WHERE version <> ?", len(updates), len(updates)); err != nil {
		return fmt.Errorf("synchronize legacy schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema migrations: %w", err)
	}
	return nil
}

func loadSchemaMigrations(resources fs.FS) ([]schemaMigration, error) {
	// Glob returns sorted names. Require a contiguous sequence starting at one.
	names, err := fs.Glob(resources, "*_up.sql")
	if err != nil {
		return nil, fmt.Errorf("list embedded migrations: %w", err)
	}
	if len(names) < 4 {
		return nil, fmt.Errorf("embedded migrations are incomplete")
	}
	updates := make([]schemaMigration, 0, len(names))
	for i, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil || version != i+1 {
			return nil, fmt.Errorf("invalid embedded migration sequence at %s: expected version %d", name, i+1)
		}
		query, err := fs.ReadFile(resources, name)
		if err != nil {
			return nil, fmt.Errorf("read embedded migration %s: %w", name, err)
		}
		updates = append(updates, schemaMigration{version: version, name: name, query: string(query)})
	}
	return updates, nil
}

func schemaVersion(ctx context.Context, tx *sql.Tx, latest int) (int, error) {
	rows, err := tx.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return 0, fmt.Errorf("read migration history: %w", err)
	}
	defer func() { _ = rows.Close() }()
	current := 0
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return 0, fmt.Errorf("invalid migration history: %w", err)
		}
		if version > latest {
			return 0, fmt.Errorf("database schema version %d is newer than supported version %d; upgrade DevEx", version, latest)
		}
		if version != current+1 {
			return 0, fmt.Errorf("corrupt migration history: expected version %d, found %d", current+1, version)
		}
		current = version
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read migration history: %w", err)
	}
	return current, nil
}

// Validate against a schema built from the same versioned resources rather than
// duplicating column definitions in Go. Extra application tables are permitted.
func validateSchema(ctx context.Context, tx *sql.Tx, updates []schemaMigration, version int) error {
	reference, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Errorf("open schema reference: %w", err)
	}
	defer func() { _ = reference.Close() }()
	reference.SetMaxOpenConns(1)
	for _, update := range updates[:max(1, version)] {
		if _, err := reference.ExecContext(ctx, update.query); err != nil {
			return fmt.Errorf("build schema reference from %s: %w", update.name, err)
		}
	}
	if _, err := reference.ExecContext(ctx, updates[3].query); err != nil {
		return fmt.Errorf("build migration history reference: %w", err)
	}

	tables, err := schemaRows(ctx, reference, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return err
	}
	if version == 0 {
		actual, err := schemaRows(ctx, tx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations'")
		if err != nil {
			return err
		}
		if len(actual) == 0 {
			// Fresh database: only the newly-created history table exists.
			tables = [][]any{{"schema_migrations"}}
		}
	}
	checks := []struct {
		name  string
		query string
	}{
		{"columns", `SELECT name, UPPER(type), CASE WHEN name = 'applied_at' THEN 0 ELSE "notnull" END, dflt_value, pk FROM pragma_table_info(?) ORDER BY cid`},
		{"indexes", `SELECT il.name, il."unique", il.partial, ii.seqno, ii.name FROM pragma_index_list(?) il JOIN pragma_index_info(il.name) ii ORDER BY il.name, ii.seqno`},
		{"foreign keys", `SELECT "table", "from", "to", on_update, on_delete, match FROM pragma_foreign_key_list(?) ORDER BY id, seq`},
	}
	for _, table := range tables {
		name, ok := table[0].(string)
		if !ok {
			return fmt.Errorf("invalid schema reference table name: %v", table[0])
		}
		if version == 0 && name == "schema_version" {
			continue // The old minimal schema did not have this table.
		}
		for _, check := range checks {
			expected, err := schemaRows(ctx, reference, check.query, name)
			if err != nil {
				return err
			}
			actual, err := schemaRows(ctx, tx, check.query, name)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return fmt.Errorf("table %s has incompatible %s", name, check.name)
			}
		}
	}
	if version > 0 {
		var count, legacy int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(MAX(version), 0) FROM schema_version").Scan(&count, &legacy); err != nil {
			return fmt.Errorf("read legacy schema version: %w", err)
		}
		if count != 1 || legacy < 1 || legacy > version {
			return fmt.Errorf("invalid legacy schema version %d (rows: %d)", legacy, count)
		}
	}
	violations, err := schemaRows(ctx, tx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("check foreign keys: %w", err)
	}
	if len(violations) != 0 {
		return fmt.Errorf("corrupt database: foreign key violations: %v", violations)
	}
	return nil
}

type schemaQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func schemaRows(ctx context.Context, db schemaQuerier, query string, args ...any) ([][]any, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("inspect schema: %w", err)
	}
	defer func() { _ = rows.Close() }()
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("inspect schema columns: %w", err)
	}
	var result [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, fmt.Errorf("inspect schema row: %w", err)
		}
		result = append(result, values)
	}
	return result, rows.Err()
}
