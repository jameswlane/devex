package datastore_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/stretchr/testify/assert"
)

func TestNewSQLite(t *testing.T) {
	t.Parallel()
	// Initialize in-memory SQLite
	db, err := datastore.NewSQLite(":memory:")
	assert.NoError(t, err, "Failed to initialize SQLite")
	defer func(conn *sql.DB) {
		err := conn.Close()
		if err != nil {
			panic(err)
		}
	}(db.Conn())

	// Verify schema tables exist
	tables := []string{"system_data", "installed_apps", "schema_migrations"}
	for _, table := range tables {
		query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?;`
		ctx := context.Background()
		row := db.Conn().QueryRowContext(ctx, query, table)
		var name string
		err := row.Scan(&name)
		assert.NoErrorf(t, err, "Table %s should exist", table)
		assert.Equal(t, table, name)
	}
}
