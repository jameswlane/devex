package datastore_test

import (
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SQLite initialization", func() {
	It("returns the complete embedded schema on a fresh install", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Chdir(home)
		db, err := datastore.NewSQLiteContext(ctx, filepath.Join(home, "datastore.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(db.Close()).To(Succeed()) })
		for _, table := range []string{"system_data", "installed_apps", "schema_migrations", "app_dependencies", "app_dependents", "uninstall_history", "orphaned_packages", "protected_packages"} {
			var name string
			Expect(db.Conn().QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)).To(Succeed())
			Expect(name).To(Equal(table))
		}
		var count int
		Expect(db.Conn().QueryRowContext(ctx, "SELECT COUNT(*) FROM protected_packages").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(10))
	})

	It("enforces foreign keys on the runtime connection so dependency rows cascade", func(ctx SpecContext) {
		db, err := datastore.NewSQLiteContext(ctx, filepath.Join(GinkgoT().TempDir(), "datastore.db"))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(db.Close()).To(Succeed()) })
		Expect(db.Exec("INSERT INTO installed_apps(app_name) VALUES ('editor')")).To(Succeed())
		Expect(db.Exec("INSERT INTO app_dependencies(app_name, dependency_name) VALUES ('editor', 'runtime')")).To(Succeed())
		Expect(db.Exec("INSERT INTO app_dependencies(app_name, dependency_name) VALUES ('missing', 'runtime')")).NotTo(Succeed())
		Expect(db.Exec("DELETE FROM installed_apps WHERE app_name = 'editor'")).To(Succeed())
		var count int
		Expect(db.Conn().QueryRowContext(ctx, "SELECT COUNT(*) FROM app_dependencies").Scan(&count)).To(Succeed())
		Expect(count).To(BeZero())
	})
})
