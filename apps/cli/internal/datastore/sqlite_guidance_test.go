package datastore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Initialization failure guidance", func() {
	DescribeTable("chooses guidance by failure kind", func(err error, expected string) {
		Expect(initializationGuidance(err)).To(Equal(expected))
	},
		Entry("canceled", fmt.Errorf("begin: %w", context.Canceled), retryGuidance),
		Entry("deadline", fmt.Errorf("begin: %w", context.DeadlineExceeded), retryGuidance),
		Entry("schema failure", schemaErrorf("corrupt migration history"), recoveryGuidance),
		Entry("unrelated failure", errors.New("permission denied"), ""),
	)

	It("tells the user to retry, not restore a backup, when the database stays locked", func() {
		path := filepath.Join(GinkgoT().TempDir(), "datastore.db")
		holderDB, err := sql.Open("sqlite", sqliteDSN(path))
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(holderDB.Close()).To(Succeed()) })
		holder, err := holderDB.Conn(context.Background())
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(holder.Close()).To(Succeed()) })
		_, err = holder.ExecContext(context.Background(), "BEGIN IMMEDIATE")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { _, _ = holder.ExecContext(context.Background(), "ROLLBACK") })

		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()
		db, err := NewSQLiteContext(ctx, path)
		Expect(db).To(BeNil())
		Expect(err).To(MatchError(ContainSubstring(retryGuidance)))
		Expect(err).NotTo(MatchError(ContainSubstring("back up")))
	})

	It("keeps backup guidance for a database that is not SQLite", func(ctx SpecContext) {
		path := filepath.Join(GinkgoT().TempDir(), "datastore.db")
		Expect(os.WriteFile(path, []byte("not a SQLite database"), 0600)).To(Succeed())
		_, err := NewSQLiteContext(ctx, path)
		Expect(err).To(MatchError(ContainSubstring(recoveryGuidance)))
	})
})
