package datastore_test

import (
	"context"
	"path/filepath"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent datastore startup", func() {
	const startups = 12

	// openTogether releases every startup at once so they contend for the same file.
	openTogether := func(ctx context.Context, path string) []error {
		start := make(chan struct{})
		errs := make([]error, startups)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				<-start
				db, err := datastore.NewSQLiteContext(ctx, path)
				if err == nil {
					err = db.Close()
				}
				errs[i] = err
			}()
		}
		close(start)
		wg.Wait()
		return errs
	}

	It("starts concurrently against an already migrated database", func(ctx SpecContext) {
		path := filepath.Join(GinkgoT().TempDir(), "datastore.db")
		warm, err := datastore.NewSQLiteContext(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		Expect(warm.Close()).To(Succeed())

		for round := 0; round < 3; round++ {
			Expect(openTogether(ctx, path)).To(Equal(make([]error, startups)))
		}
	})

	It("migrates a fresh database exactly once when startups race", func(ctx SpecContext) {
		path := filepath.Join(GinkgoT().TempDir(), "datastore.db")
		Expect(openTogether(ctx, path)).To(Equal(make([]error, startups)))

		db, err := datastore.NewSQLiteContext(ctx, path)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(db.Close()).To(Succeed()) })
		var count int
		Expect(db.Conn().QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations").Scan(&count)).To(Succeed())
		Expect(count).To(Equal(5))
	})
})
