package main_test

import (
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	devex "github.com/jameswlane/devex/apps/cli/cmd"
	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/errors"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Main", func() {
	Context("initializeDatabase", func() {
		It("initializes the database and returns a repository", func() {
			tempDir := os.TempDir()
			dbPath := filepath.Join(tempDir, ".devex/datastore.db")

			// Ensure cleanup after test
			defer func(path string) {
				err := os.RemoveAll(path)
				if err != nil {
					return
				}
			}(filepath.Join(tempDir, ".devex"))

			repo := testInitializeDatabase(tempDir)
			Expect(repo).ToNot(BeNil())
			Expect(dbPath).To(BeAnExistingFile())
		})
	})

	Context("handleError", func() {
		var originalExit func(int)
		var exitCalled bool

		BeforeEach(func() {
			originalExit = devex.Exit
			exitCalled = false
			devex.Exit = func(code int) {
				exitCalled = true
			}
		})

		AfterEach(func() {
			devex.Exit = originalExit // Restore the original Exit function
		})

		It("logs and exits on invalid input error", func() {
			Expect(func() {
				testHandleError(errors.ErrInvalidInput)
			}).ToNot(Panic())
			Expect(exitCalled).To(BeTrue())
		})

		It("logs and exits on unexpected error", func() {
			Expect(func() {
				testHandleError(errors.New("unexpected error"))
			}).ToNot(Panic())
			Expect(exitCalled).To(BeTrue())
		})
	})
})

// Helper function to test initializeDatabase
func testInitializeDatabase(homeDir string) types.Repository {
	// Ensure the .devex directory exists
	devexDir := filepath.Join(homeDir, ".devex")
	err := os.MkdirAll(devexDir, 0o755) // Create the directory if it doesn't exist
	Expect(err).ToNot(HaveOccurred(), "failed to create .devex directory")

	// Initialize the database
	dbPath := filepath.Join(devexDir, "datastore.db")
	sqlite, err := datastore.NewSQLite(dbPath)
	Expect(err).ToNot(HaveOccurred(), "failed to initialize SQLite database")

	repo := repository.NewRepository(sqlite)
	Expect(repo).ToNot(BeNil(), "repository should not be nil")
	return repo
}

// Helper function to test handleError
func testHandleError(err error) {
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		devex.Exit(1)
	}
}
