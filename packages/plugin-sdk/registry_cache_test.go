package sdk_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Registry Caching", func() {
	var (
		server      *httptest.Server
		client      *sdk.RegistryClient
		ctx         context.Context
		requestCount int32
	)

	BeforeEach(func() {
		ctx = context.Background()
		atomic.StoreInt32(&requestCount, 0)

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&requestCount, 1)

			switch r.URL.Path {
			case "/api/v1/registry":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"base_url": "https://registry.example.com",
					"version": "1.0",
					"last_updated": "2023-01-01T00:00:00Z",
					"plugins": {
						"test-plugin": {
							"name": "test-plugin",
							"version": "1.0.0",
							"description": "A test plugin"
						}
					}
				}`)
			case "/api/v1/plugins/test-plugin":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"name": "test-plugin",
					"version": "1.0.0",
					"description": "A test plugin",
					"author": "Test Author"
				}`)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))

		config := sdk.RegistryConfig{
			BaseURL:  server.URL,
			Timeout:  30 * time.Second,
			CacheTTL: 100 * time.Millisecond,
			Logger:   NewSilentLogger(),
		}
		var err error
		client, err = sdk.NewRegistryClient(config)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("Registry Caching", func() {
		It("should cache GetRegistry responses", func() {
			// First call should hit the server
			registry1, err := client.GetRegistry(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(registry1).ToNot(BeNil())
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(int32(1)))

			// Second call should use cache
			registry2, err := client.GetRegistry(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(registry2).To(Equal(registry1))
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(int32(1))) // No additional request

			// Wait for cache to expire
			time.Sleep(150 * time.Millisecond)

			// Third call should hit the server again
			registry3, err := client.GetRegistry(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(registry3).ToNot(BeNil())
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(int32(2)))
		})

		It("should cache GetPlugin responses", func() {
			// First call should hit the server
			plugin1, err := client.GetPlugin(ctx, "test-plugin")
			Expect(err).ToNot(HaveOccurred())
			Expect(plugin1).ToNot(BeNil())
			initialCount := atomic.LoadInt32(&requestCount)

			// Second call should use cache
			plugin2, err := client.GetPlugin(ctx, "test-plugin")
			Expect(err).ToNot(HaveOccurred())
			Expect(plugin2).To(Equal(plugin1))
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(initialCount)) // No additional request

			// Wait for cache to expire
			time.Sleep(150 * time.Millisecond)

			// Third call should hit the server again
			plugin3, err := client.GetPlugin(ctx, "test-plugin")
			Expect(err).ToNot(HaveOccurred())
			Expect(plugin3).ToNot(BeNil())
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(initialCount + 1))
		})

		It("should cache SearchPlugins responses", func() {
			// First search should fetch registry
			results1, err := client.SearchPlugins(ctx, "test", nil, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(results1).ToNot(BeNil())
			initialCount := atomic.LoadInt32(&requestCount)

			// Second identical search should use cache
			results2, err := client.SearchPlugins(ctx, "test", nil, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(results2).To(Equal(results1))
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(initialCount)) // No additional request

			// Different search parameters should result in new request
			results3, err := client.SearchPlugins(ctx, "different", nil, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(results3).ToNot(BeNil())
			// Note: This might use cached registry data, so we don't check request count

			// Wait for cache to expire
			time.Sleep(150 * time.Millisecond)

			// Same search after expiry should fetch again
			results4, err := client.SearchPlugins(ctx, "test", nil, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(results4).ToNot(BeNil())
			Expect(atomic.LoadInt32(&requestCount)).To(BeNumerically(">", initialCount))
		})

		It("should use separate cache keys for different plugins", func() {
			// Mock server that returns different data for different plugins
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&requestCount, 1)

				switch r.URL.Path {
				case "/api/v1/plugins/plugin1":
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, `{"name": "plugin1", "version": "1.0.0"}`)
				case "/api/v1/plugins/plugin2":
					w.WriteHeader(http.StatusOK)
					_, _ = fmt.Fprint(w, `{"name": "plugin2", "version": "2.0.0"}`)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer testServer.Close()

			config := sdk.RegistryConfig{
				BaseURL:  testServer.URL,
				CacheTTL: 100 * time.Millisecond,
				Logger:   NewSilentLogger(),
			}
			testClient, err := sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())

			// Reset counter
			atomic.StoreInt32(&requestCount, 0)

			// Get plugin1
			plugin1, err := testClient.GetPlugin(ctx, "plugin1")
			Expect(err).ToNot(HaveOccurred())
			Expect(plugin1.Name).To(Equal("plugin1"))
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(int32(1)))

			// Get plugin2 (should not use plugin1's cache)
			plugin2, err := testClient.GetPlugin(ctx, "plugin2")
			Expect(err).ToNot(HaveOccurred())
			Expect(plugin2.Name).To(Equal("plugin2"))
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(int32(2)))

			// Get plugin1 again (should use cache)
			plugin1Again, err := testClient.GetPlugin(ctx, "plugin1")
			Expect(err).ToNot(HaveOccurred())
			Expect(plugin1Again.Name).To(Equal("plugin1"))
			Expect(atomic.LoadInt32(&requestCount)).To(Equal(int32(2))) // No additional request
		})
	})

	Describe("Context Cancellation", func() {
		It("should respect context cancellation", func() {
			// Create a slow server
			slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(1 * time.Second)
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"name": "test"}`)
			}))
			defer slowServer.Close()

			config := sdk.RegistryConfig{
				BaseURL: slowServer.URL,
				Timeout: 5 * time.Second,
				Logger:  NewSilentLogger(),
			}
			slowClient, err := sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())

			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Should fail due to context cancellation
			_, err = slowClient.GetPlugin(ctx, "test-plugin")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("context"))
		})

		It("should handle already cancelled context", func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			_, err := client.GetRegistry(ctx)
			Expect(err).To(HaveOccurred())
		})
	})
})
