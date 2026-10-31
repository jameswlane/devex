package datastore

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"testing/fstest"

	"github.com/jameswlane/devex/apps/cli/migrations"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transactional schema updates", func() {
	var conn *sql.DB
	var path string

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		path = filepath.Join(home, "datastore.db")
		var err error
		conn, err = sql.Open("sqlite", path)
		Expect(err).NotTo(HaveOccurred())
		conn.SetMaxOpenConns(1)
		DeferCleanup(func() { Expect(conn.Close()).To(Succeed()) })
	})

	seedVersion := func(ctx context.Context, version int) {
		updates, err := loadSchemaMigrations(migrations.Files)
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP)`)
		Expect(err).NotTo(HaveOccurred())
		for _, update := range updates[:version] {
			_, err = conn.ExecContext(ctx, update.query)
			Expect(err).NotTo(HaveOccurred())
			_, err = conn.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES (?)", update.version)
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = conn.ExecContext(ctx, "INSERT INTO installed_apps(app_name) VALUES ('editor')")
		Expect(err).NotTo(HaveOccurred())
	}

	It("creates a fresh database and repeats startup without changing history or app records", func(ctx SpecContext) {
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
		_, err := conn.ExecContext(ctx, "INSERT INTO installed_apps(app_name, version) VALUES ('editor', '1.2')")
		Expect(err).NotTo(HaveOccurred())
		before, err := schemaRows(ctx, conn, "SELECT version, applied_at FROM schema_migrations ORDER BY version")
		Expect(err).NotTo(HaveOccurred())
		Expect(before).To(HaveLen(5))
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
		after, err := schemaRows(ctx, conn, "SELECT version, applied_at FROM schema_migrations ORDER BY version")
		Expect(err).NotTo(HaveOccurred())
		Expect(after).To(Equal(before))
		var version string
		Expect(conn.QueryRowContext(ctx, "SELECT version FROM installed_apps WHERE app_name='editor'").Scan(&version)).To(Succeed())
		Expect(version).To(Equal("1.2"))
		Expect(conn.Close()).To(Succeed())
		db, err := NewSQLiteContext(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		Expect(db.Close()).To(Succeed())
	})

	It("migrates the legacy minimal database without losing app IDs or system data", func(ctx SpecContext) {
		_, err := conn.ExecContext(ctx, `
		CREATE TABLE system_data (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE installed_apps (id INTEGER PRIMARY KEY AUTOINCREMENT, app_name TEXT NOT NULL UNIQUE);
		CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME DEFAULT CURRENT_TIMESTAMP);
		INSERT INTO installed_apps(id, app_name) VALUES (42, 'editor');
		INSERT INTO system_data(key, value) VALUES ('theme', 'dark');`)
		Expect(err).NotTo(HaveOccurred())
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
		var id, version int
		var value string
		Expect(conn.QueryRowContext(ctx, "SELECT id FROM installed_apps WHERE app_name='editor'").Scan(&id)).To(Succeed())
		Expect(id).To(Equal(42))
		Expect(conn.QueryRowContext(ctx, "SELECT value FROM system_data WHERE key='theme'").Scan(&value)).To(Succeed())
		Expect(value).To(Equal("dark"))
		Expect(conn.QueryRowContext(ctx, "SELECT version FROM schema_version").Scan(&version)).To(Succeed())
		Expect(version).To(Equal(5))
		// The dependency migration must reference the real, unique app_name column.
		_, err = conn.ExecContext(ctx, "PRAGMA foreign_keys=ON; INSERT INTO app_dependencies(app_name, dependency_name) VALUES ('editor', 'runtime')")
		Expect(err).NotTo(HaveOccurred())
		_, err = conn.ExecContext(ctx, "INSERT INTO app_dependencies(app_name, dependency_name) VALUES ('missing', 'runtime')")
		Expect(err).To(HaveOccurred())
	})

	DescribeTable("upgrades an existing versioned database", func(ctx SpecContext, version int) {
		seedVersion(ctx, version)
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
		var count int
		Expect(conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(5))
		Expect(conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM installed_apps WHERE app_name='editor'").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(1))
	}, Entry("version one", 1), Entry("version two", 2), Entry("version four", 4))

	It("rolls back pending DDL, data, and version records when a later migration fails", func(ctx SpecContext) {
		seedVersion(ctx, 1)
		resources := fstest.MapFS{}
		Expect(fs.WalkDir(migrations.Files, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			data, err := migrations.Files.ReadFile(name)
			resources[name] = &fstest.MapFile{Data: data}
			return err
		})).To(Succeed())
		resources["005_add_dependency_tracking_up.sql"] = &fstest.MapFile{Data: []byte(`
			CREATE TABLE partial_migration (id INTEGER);
			INSERT INTO installed_apps(app_name) VALUES ('partial-app');
			THIS IS INVALID SQL;`)}
		err := applySchemaUpdates(ctx, conn, resources)
		Expect(err).To(MatchError(ContainSubstring("apply migration 005_add_dependency_tracking_up.sql")))
		var count int
		Expect(conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(1))
		Expect(conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM installed_apps").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(1))
		Expect(conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE name='partial_migration'").Scan(&count)).To(Succeed())
		Expect(count).To(BeZero())
		columns, err := schemaRows(ctx, conn, "SELECT name FROM pragma_table_info('installed_apps')")
		Expect(err).NotTo(HaveOccurred())
		Expect(columns).To(Equal([][]any{{"id"}, {"app_name"}}))
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
	})

	DescribeTable("rejects incompatible or corrupt databases without changing them", func(ctx SpecContext, mutation, message string) {
		seedVersion(ctx, 5)
		_, err := conn.ExecContext(ctx, mutation)
		Expect(err).NotTo(HaveOccurred())
		before, err := schemaRows(ctx, conn, "SELECT version, applied_at FROM schema_migrations ORDER BY version")
		Expect(err).NotTo(HaveOccurred())
		Expect(ApplySchemaUpdates(ctx, conn)).To(MatchError(ContainSubstring(message)))
		after, err := schemaRows(ctx, conn, "SELECT version, applied_at FROM schema_migrations ORDER BY version")
		Expect(err).NotTo(HaveOccurred())
		Expect(after).To(Equal(before))
	},
		Entry("newer version", "INSERT INTO schema_migrations(version) VALUES (6)", "newer than supported"),
		Entry("history gap", "DELETE FROM schema_migrations WHERE version=3", "corrupt migration history"),
		Entry("invalid version", "INSERT INTO schema_migrations(version) VALUES (-1)", "corrupt migration history"),
		Entry("missing table", "DROP TABLE protected_packages", "incompatible columns"),
		Entry("missing column", "ALTER TABLE installed_apps DROP COLUMN version", "incompatible columns"),
		Entry("missing index", "DROP INDEX idx_installed_apps_app_name", "incompatible indexes"),
		Entry("foreign key violation", "INSERT INTO app_dependencies(app_name, dependency_name) VALUES ('missing', 'runtime')", "foreign key violations"),
		Entry("invalid legacy version", "UPDATE schema_version SET version=99", "invalid legacy schema version"),
	)

	It("verifies a current database read-only without writing", func(ctx SpecContext) {
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
		before, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())

		readOnly, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(readOnly.Close()).To(Succeed()) })
		Expect(ApplySchemaUpdates(ctx, readOnly)).To(Succeed())

		after, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(after).To(Equal(before))
	})

	It("does not wait for the write lock when the database is current", func(ctx SpecContext) {
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
		writer, err := sql.Open("sqlite", sqliteDSN(path))
		Expect(err).NotTo(HaveOccurred())
		writer.SetMaxOpenConns(1)
		DeferCleanup(func() { Expect(writer.Close()).To(Succeed()) })
		holder, err := writer.Conn(ctx)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(holder.Close()).To(Succeed()) })
		_, err = holder.ExecContext(ctx, "BEGIN IMMEDIATE")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _, _ = holder.ExecContext(context.Background(), "ROLLBACK") })

		// Another process holds the write lock; verification only needs to read.
		Expect(ApplySchemaUpdates(ctx, conn)).To(Succeed())
	})

	DescribeTable("validates schema drift when migrations are pending", func(ctx SpecContext, version int, mutation, message string) {
		seedVersion(ctx, version)
		_, err := conn.ExecContext(ctx, mutation)
		Expect(err).NotTo(HaveOccurred())
		Expect(ApplySchemaUpdates(ctx, conn)).To(MatchError(ContainSubstring(message)))
	},
		Entry("missing column", 4, "ALTER TABLE installed_apps DROP COLUMN version", "incompatible columns"),
		Entry("missing index", 3, "DROP INDEX idx_installed_apps_app_name", "incompatible indexes"),
		Entry("missing table", 4, "DROP TABLE system_data", "incompatible columns"),
		Entry("foreign key violation", 5,
			"INSERT INTO app_dependencies(app_name, dependency_name) VALUES ('missing', 'runtime'); DELETE FROM schema_migrations WHERE version=5",
			"foreign key violations"),
	)

	It("rolls back a migration when its version record cannot be written", func(ctx SpecContext) {
		seedVersion(ctx, 1)
		_, err := conn.ExecContext(ctx, `CREATE TRIGGER reject_migration_version
			BEFORE INSERT ON schema_migrations BEGIN SELECT RAISE(ABORT, 'history is read-only'); END`)
		Expect(err).NotTo(HaveOccurred())
		Expect(ApplySchemaUpdates(ctx, conn)).To(MatchError(ContainSubstring("record migration 002_add_version_column_up.sql")))
		columns, err := schemaRows(ctx, conn, "SELECT name FROM pragma_table_info('installed_apps')")
		Expect(err).NotTo(HaveOccurred())
		Expect(columns).To(Equal([][]any{{"id"}, {"app_name"}}))
		var version int
		Expect(conn.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&version)).To(Succeed())
		Expect(version).To(Equal(1))
	})

	It("rejects an unrelated unversioned schema and rolls back history creation", func(ctx SpecContext) {
		_, err := conn.ExecContext(ctx, "CREATE TABLE installed_apps(name TEXT)")
		Expect(err).NotTo(HaveOccurred())
		Expect(ApplySchemaUpdates(ctx, conn)).To(MatchError(ContainSubstring("incompatible")))
		var count int
		Expect(conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE name='schema_migrations'").Scan(&count)).To(Succeed())
		Expect(count).To(BeZero())
	})

	It("rejects a corrupt SQLite file with recovery guidance", func(ctx SpecContext) {
		Expect(conn.Close()).To(Succeed())
		Expect(os.WriteFile(path, []byte("not a SQLite database"), 0600)).To(Succeed())
		db, err := NewSQLiteContext(ctx, path)
		Expect(db).To(BeNil())
		Expect(err).To(MatchError(And(ContainSubstring("check SQLite integrity"), ContainSubstring("back up the database"))))
	})

	It("propagates cancellation and leaves the database untouched", func(ctx SpecContext) {
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		Expect(ApplySchemaUpdates(canceled, conn)).To(MatchError(ContainSubstring("context canceled")))
		var count int
		Expect(conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table'").Scan(&count)).To(Succeed())
		Expect(count).To(BeZero())
	})
})
