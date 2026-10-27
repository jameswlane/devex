package datastore

import (
	"database/sql"
	"log"

	"github.com/jameswlane/devex/apps/cli/internal/types"

	_ "modernc.org/sqlite"
)

// NewInMemorySQLite creates and initializes an in-memory SQLite database with the given schema.
func NewInMemorySQLite() types.Database {
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatalf("Failed to open SQLite in-memory database: %v", err)
	}
	err = InitializeSchema(conn)
	if err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	return &SQLite{conn: conn}
}
