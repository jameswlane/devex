package main

import (
	"database/sql"
	"os"
	"path/filepath"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Database startup", func() {
	It("migrates before returning a repository and preserves apps on repeat startup", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		// A temporary explicit home keeps this test away from the user's datastore.
		repo, db, err := initializeDatabase(ctx, home, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.AddApp("editor")).To(Succeed())
		Expect(db.Close()).To(Succeed())
		repo, db, err = initializeDatabase(ctx, home, false)
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
		repo, db, err := initializeDatabase(ctx, home, false)
		Expect(repo).To(BeNil())
		Expect(db).To(BeNil())
		Expect(err).To(MatchError(And(ContainSubstring("newer than supported"), ContainSubstring("upgrade DevEx"), ContainSubstring("datastore.db"))))
	})
	It("previews real installation state read-only without changing the database", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		repo, db, err := initializeDatabase(ctx, home, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.AddApp("tracked-editor")).To(Succeed())
		Expect(db.Close()).To(Succeed())
		path := filepath.Join(devexpaths.ForHome(home).State, "datastore.db")
		before, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		repo, db, err = initializeDatabase(ctx, home, true)
		Expect(err).NotTo(HaveOccurred())
		apps, err := repo.ListApps()
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ConsistOf(HaveField("Name", "tracked-editor")))
		Expect(repo.AddApp("must-not-write")).NotTo(Succeed())
		Expect(db.Close()).To(Succeed())
		Expect(os.ReadFile(path)).To(Equal(before))
	})

	It("reads the legacy datastore in previews without creating the absent state directory", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		repo, db, err := initializeDatabase(ctx, home, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.AddApp("legacy-editor")).To(Succeed())
		Expect(db.Close()).To(Succeed())
		state := devexpaths.ForHome(home).State
		Expect(os.MkdirAll(filepath.Join(home, ".devex"), 0700)).To(Succeed())
		Expect(os.Rename(filepath.Join(state, "datastore.db"), filepath.Join(home, ".devex", "datastore.db"))).To(Succeed())
		Expect(os.RemoveAll(state)).To(Succeed())
		repo, db, err = initializeDatabase(ctx, home, true)
		Expect(err).NotTo(HaveOccurred())
		apps, err := repo.ListApps()
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ConsistOf(HaveField("Name", "legacy-editor")))
		Expect(db.Close()).To(Succeed())
		Expect(state).NotTo(BeADirectory())
	})

	It("keeps a first-run preview off disk when no datastore exists", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		repo, db, err := initializeDatabase(ctx, home, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.ListApps()).To(BeEmpty())
		Expect(db.Close()).To(Succeed())
		Expect(devexpaths.ForHome(home).State).NotTo(BeADirectory())
	})

})
