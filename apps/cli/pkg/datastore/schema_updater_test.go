package datastore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jameswlane/devex/pkg/types"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/datastore/repository"
)

func TestApplySchemaUpdates(t *testing.T) {
	t.Parallel()
	db := datastore.NewInMemorySQLite()
	defer func(db types.Database) {
		err := db.Close()
		if err != nil {
			panic(err)
		}
	}(db)

	repo := repository.NewSchemaRepository(db)

	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "devex_test")
	assert.NoError(t, err)
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			panic(err)
		}
	}(tmpDir)

	// Create the expected directory structure: homeDir/.local/share/devex/migrations
	migrationsDir := filepath.Join(tmpDir, ".local", "share", "devex", "migrations")
	err = os.MkdirAll(migrationsDir, 0755)
	assert.NoError(t, err)

	// Test with empty migrations directory (should succeed)
	err = datastore.ApplySchemaUpdates(repo, tmpDir)
	assert.NoError(t, err)
}
