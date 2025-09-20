package sdk_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Registry Search Optimization", func() {
	var (
		server *httptest.Server
		client *sdk.RegistryClient
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/registry":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"base_url": "https://registry.example.com",
					"version": "1.0",
					"last_updated": "2023-01-01T00:00:00Z",
					"plugins": {
						"docker": {
							"name": "docker",
							"version": "1.0.0",
							"description": "Docker container management",
							"tags": ["containers", "deployment", "devops"]
						},
						"git": {
							"name": "git",
							"version": "1.1.0",
							"description": "Git version control system",
							"tags": ["vcs", "development", "collaboration"]
						},
						"nodejs": {
							"name": "nodejs",
							"version": "2.0.0",
							"description": "Node.js JavaScript runtime",
							"tags": ["javascript", "runtime", "development"]
						},
						"python": {
							"name": "python",
							"version": "1.5.0",
							"description": "Python programming language",
							"tags": ["python", "programming", "development"]
						},
						"mysql": {
							"name": "mysql",
							"version": "1.2.0",
							"description": "MySQL database server",
							"tags": ["database", "sql", "storage"]
						}
					}
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

	Describe("Tag-Based Search Optimization", func() {
		It("should efficiently search by single tag", func() {
			results, err := client.SearchPlugins(ctx, "", []string{"development"}, 10)
			Expect(err).ToNot(HaveOccurred())

			// Should find git, nodejs, and python (all have "development" tag)
			Expect(len(results)).To(Equal(3))

			names := make([]string, len(results))
			for i, plugin := range results {
				names[i] = plugin.Name
			}
			Expect(names).To(ContainElements("git", "nodejs", "python"))
		})

		It("should efficiently search by multiple tags", func() {
			results, err := client.SearchPlugins(ctx, "", []string{"database", "containers"}, 10)
			Expect(err).ToNot(HaveOccurred())

			// Should find docker (containers) and mysql (database)
			Expect(len(results)).To(Equal(2))

			names := make([]string, len(results))
			for i, plugin := range results {
				names[i] = plugin.Name
			}
			Expect(names).To(ContainElements("docker", "mysql"))
		})

		It("should combine tag and query search efficiently", func() {
			results, err := client.SearchPlugins(ctx, "javascript", []string{"development"}, 10)
			Expect(err).ToNot(HaveOccurred())

			// Should find only nodejs (has "development" tag AND contains "javascript" in description)
			Expect(len(results)).To(Equal(1))
			Expect(results[0].Name).To(Equal("nodejs"))
		})

		It("should return empty results for non-existent tags", func() {
			results, err := client.SearchPlugins(ctx, "", []string{"nonexistent"}, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(results)).To(Equal(0))
		})
	})

	Describe("Query Search Optimization", func() {
		It("should search by name efficiently", func() {
			results, err := client.SearchPlugins(ctx, "docker", nil, 10)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(results)).To(Equal(1))
			Expect(results[0].Name).To(Equal("docker"))
		})

		It("should search by partial name", func() {
			results, err := client.SearchPlugins(ctx, "node", nil, 10)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(results)).To(Equal(1))
			Expect(results[0].Name).To(Equal("nodejs"))
		})

		It("should search by description", func() {
			results, err := client.SearchPlugins(ctx, "container", nil, 10)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(results)).To(Equal(1))
			Expect(results[0].Name).To(Equal("docker"))
		})

		It("should be case insensitive", func() {
			results, err := client.SearchPlugins(ctx, "MYSQL", nil, 10)
			Expect(err).ToNot(HaveOccurred())

			Expect(len(results)).To(Equal(1))
			Expect(results[0].Name).To(Equal("mysql"))
		})
	})

	Describe("Search Performance", func() {
		It("should respect search limits", func() {
			results, err := client.SearchPlugins(ctx, "", []string{"development"}, 2)
			Expect(err).ToNot(HaveOccurred())

			// Should return at most 2 results even though 3 plugins match
			Expect(len(results)).To(Equal(2))
		})

		It("should handle empty queries efficiently", func() {
			results, err := client.SearchPlugins(ctx, "", nil, 3)
			Expect(err).ToNot(HaveOccurred())

			// Should return first 3 plugins (respects limit)
			Expect(len(results)).To(Equal(3))
		})

		It("should cache search results", func() {
			// First search
			start := time.Now()
			results1, err := client.SearchPlugins(ctx, "docker", nil, 10)
			firstDuration := time.Since(start)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(results1)).To(Equal(1))

			// Second identical search (should hit cache)
			start = time.Now()
			results2, err := client.SearchPlugins(ctx, "docker", nil, 10)
			secondDuration := time.Since(start)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(results2)).To(Equal(1))

			// Cache should be significantly faster
			Expect(secondDuration).To(BeNumerically("<", firstDuration/2))
		})
	})

	Describe("Edge Cases", func() {
		It("should handle zero limit gracefully", func() {
			results, err := client.SearchPlugins(ctx, "docker", nil, 0)
			Expect(err).ToNot(HaveOccurred())

			// Should use default limit (100)
			Expect(len(results)).To(Equal(1))
		})

		It("should handle negative limit gracefully", func() {
			results, err := client.SearchPlugins(ctx, "docker", nil, -5)
			Expect(err).ToNot(HaveOccurred())

			// Should use default limit (100)
			Expect(len(results)).To(Equal(1))
		})

		It("should return empty slice instead of nil", func() {
			results, err := client.SearchPlugins(ctx, "nonexistent", nil, 10)
			Expect(err).ToNot(HaveOccurred())

			Expect(results).ToNot(BeNil())
			Expect(len(results)).To(Equal(0))
		})
	})
})
