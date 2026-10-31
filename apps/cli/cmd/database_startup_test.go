package main

import (
	"database/sql"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Database startup", func() {
	It("migrates before returning a repository and preserves apps on repeat startup", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		// A temporary explicit home keeps this test away from the user's datastore.
		repo, db, err := initializeDatabase(ctx, home)
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.AddApp("editor")).To(Succeed())
		Expect(db.Close()).To(Succeed())
		repo, db, err = initializeDatabase(ctx, home)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(db.Close()).To(Succeed()) })
		app, err := repo.GetApp("editor")
		Expect(err).NotTo(HaveOccurred())
		Expect(app.Name).To(Equal("editor"))
		var version int
		Expect(db.Conn().QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version)).To(Succeed())
		Expect(version).To(Equal(5))
	})

	It("returns an actionable error instead of a repository for an unsupported schema", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(home, ".devex"), 0750)).To(Succeed())
		conn, err := sql.Open("sqlite", filepath.Join(home, ".devex", "datastore.db"))
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.ExecContext(ctx, "CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY); INSERT INTO schema_migrations VALUES (99)")
		Expect(err).NotTo(HaveOccurred())
		Expect(conn.Close()).To(Succeed())
		repo, db, err := initializeDatabase(ctx, home)
		Expect(repo).To(BeNil())
		Expect(db).To(BeNil())
		Expect(err).To(MatchError(And(ContainSubstring("newer than supported"), ContainSubstring("upgrade DevEx"), ContainSubstring("datastore.db"))))
	})
})
