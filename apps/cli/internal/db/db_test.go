package db_test

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/db"
	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

var _ = Describe("Db", func() {
	var (
		tempDB   string
		conn     *sql.DB
		database *db.DB
	)

	BeforeEach(func() {
		// Use an in-memory filesystem for testing
		fs.UseMemMapFs()

		// Create a mock log file in the in-memory filesystem
		logFile, err := fs.Create("/log/test.log")
		Expect(err).ToNot(HaveOccurred())

		// Initialize the logger with the in-memory log file
		log.InitDefaultLogger(logFile)

		tempDB = filepath.Join(os.TempDir(), "test.db")
		conn, err = sql.Open("sqlite3", tempDB)
		Expect(err).ToNot(HaveOccurred())

		database = &db.DB{DB: conn}

		// Create a table to ensure the connection is valid
		_, err = conn.Exec(`CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)`)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := conn.Close()
		if err != nil {
			return
		}
		err = os.Remove(tempDB)
		if err != nil {
			return
		}
	})

	Context("ValidateConnection", func() {
		It("validates an active database connection", func() {
			err := database.ValidateConnection()
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails validation with a closed connection", func() {
			err := conn.Close()
			if err != nil {
				return
			}
			err = database.ValidateConnection()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to validate database connection"))
		})
	})

	Context("Exec", func() {
		Context("Exec", func() {
			It("executes a query successfully", func() {
				query := `CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)`
				err := database.Exec(query)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Context("QueryRow", func() {
		BeforeEach(func() {
			_ = database.Exec(`DROP TABLE IF EXISTS test_table`)
			query := `CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)`
			if err := database.Exec(query); err != nil {
				Fail(fmt.Sprintf("Failed to create test table: %v", err))
			}
			if err := database.Exec(`INSERT INTO test_table (id, name) VALUES (1, 'test')`); err != nil {
				Fail(fmt.Sprintf("Failed to insert test data: %v", err))
			}
		})

		AfterEach(func() {
			_ = database.Exec(`DROP TABLE IF EXISTS test_table`)
		})

		It("retrieves a single row successfully", func() {
			query := `SELECT id, name FROM test_table WHERE id = 1`
			result, err := database.QueryRow(query)
			Expect(err).ToNot(HaveOccurred())
			Expect(result["id"]).To(Equal(int64(1)))
			Expect(result["name"]).To(Equal("test"))
		})

		It("returns an error for non-existent rows", func() {
			query := `SELECT id, name FROM test_table WHERE id = 99`
			result, err := database.QueryRow(query)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Context("GetColumns", func() {
		BeforeEach(func() {
			_ = database.Exec(`DROP TABLE IF EXISTS test_table`)
			query := `CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)`
			if err := database.Exec(query); err != nil {
				Fail(fmt.Sprintf("Failed to create test table: %v", err))
			}
		})

		AfterEach(func() {
			_ = database.Exec(`DROP TABLE IF EXISTS test_table`)
		})

		It("retrieves column names successfully", func() {
			query := `SELECT id, name FROM test_table`
			columns, err := database.GetColumns(query) // Call the exported method
			Expect(err).ToNot(HaveOccurred())
			Expect(columns).To(ContainElements("id", "name")) // Assert that the columns are retrieved
		})

		It("returns an error for invalid queries", func() {
			query := `SELECT * FROM non_existent_table`
			columns, err := database.GetColumns(query)
			Expect(err).To(HaveOccurred()) // Ensure an error is returned
			Expect(columns).To(BeNil())    // Ensure no columns are returned
		})
	})
})
