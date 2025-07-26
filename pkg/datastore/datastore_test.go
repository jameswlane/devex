package datastore_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/stretchr/testify/assert"
)

func TestNewSQLite(t *testing.T) {
	// Initialize in-memory SQLite
	db, err := datastore.NewSQLite(":memory:")
	assert.NoError(t, err, "Failed to initialize SQLite")
	defer db.Conn().Close()

	// Verify schema tables exist
	tables := []string{"system_data", "installed_apps", "schema_migrations"}
	for _, table := range tables {
		query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?;`
		row := db.Conn().QueryRow(query, table)
		var name string
		err := row.Scan(&name)
		assert.NoErrorf(t, err, "Table %s should exist", table)
		assert.Equal(t, table, name)
	}
}
