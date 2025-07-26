package datastore_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/datastore/repository"
)

func TestApplySchemaUpdates(t *testing.T) {
	db := datastore.NewInMemorySQLite(`
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`)
	defer db.Close()

	repo := repository.NewSchemaRepository(db)
	err := datastore.ApplySchemaUpdates(repo, "/migrations")
	assert.NoError(t, err)
}
