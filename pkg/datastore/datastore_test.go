package datastore_test

import (
	"fmt"
	"io"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
)

var _ = Describe("Repository", func() {
	var repo *mocks.MockRepository

	BeforeEach(func() {
		log.InitDefaultLogger(io.Discard)
		repo = mocks.NewMockRepository()
	})

	Context("Basic Operations", func() {
		It("sets and retrieves a key-value pair", func() {
			err := repo.Set("testKey", "testValue")
			Expect(err).ToNot(HaveOccurred())

			val, err := repo.Get("testKey")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal("testValue"))
		})

		It("returns an error for a non-existent key", func() {
			_, err := repo.Get("nonExistentKey")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("key not found"))
		})
	})

	Context("App Management", func() {
		It("adds and retrieves an app", func() {
			app := &types.AppConfig{Name: "test-app"}
			err := repo.AddApp("test-app")
			Expect(err).To(BeNil())

			retrievedApp, err := repo.GetApp("test-app")
			Expect(err).To(BeNil())
			Expect(retrievedApp.Name).To(Equal(app.Name))
		})

		It("lists all apps", func() {
			err := repo.AddApp("app1")
			if err != nil {
				return
			}
			err = repo.AddApp("app2")
			if err != nil {
				return
			}

			apps, err := repo.ListApps()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(apps)).To(Equal(2))
		})
	})

	Context("Concurrency", func() {
		It("supports concurrent access", func() {
			const workers = 10
			const keysPerWorker = 10

			wg := sync.WaitGroup{}
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func(worker int) {
					defer wg.Done()
					for j := 0; j < keysPerWorker; j++ {
						key := fmt.Sprintf("worker%d-key%d", worker, j)
						err := repo.Set(key, fmt.Sprintf("value%d", j))
						if err != nil {
							return
						}
					}
				}(i)
			}
			wg.Wait()

			val, err := repo.Get("worker0-key0")
			Expect(err).ToNot(HaveOccurred())
			Expect(val).To(Equal("value0"))
		})
	})
})
