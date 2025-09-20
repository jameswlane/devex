package datastore_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Schema Updater", func() {
	Describe("ApplySchemaUpdates", func() {
		var db types.Database
		var repo types.SchemaRepository
		var tmpDir string

		BeforeEach(func() {
			db = datastore.NewInMemorySQLite()
			repo = repository.NewSchemaRepository(db)

			// Create a temporary directory structure for testing
			var err error
			tmpDir, err = os.MkdirTemp("", "devex_test")
			Expect(err).ToNot(HaveOccurred())

			// Create the expected directory structure: homeDir/.local/share/devex/migrations
			migrationsDir := filepath.Join(tmpDir, ".local", "share", "devex", "migrations")
			err = os.MkdirAll(migrationsDir, 0755)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			err := db.Close()
			if err != nil {
				panic(err)
			}

			err = os.RemoveAll(tmpDir)
			if err != nil {
				panic(err)
			}
		})

		It("should succeed with empty migrations directory", func() {
			err := datastore.ApplySchemaUpdates(repo, tmpDir)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
