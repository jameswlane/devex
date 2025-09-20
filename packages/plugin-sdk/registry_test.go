package sdk_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Registry Client", func() {
	var (
		server     *httptest.Server
		client     *sdk.RegistryClient
		ctx        context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Default handler - individual tests can override by starting new servers
			switch r.URL.Path {
			case "/api/v1/registry":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"base_url": "https://registry.example.com", "version": "1.0", "last_updated": "2023-01-01T00:00:00Z", "plugins": {}}`)
			case "/api/v1/plugins/test-plugin":
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{
					"name": "test-plugin",
					"version": "1.0.0",
					"description": "A test plugin",
					"author": "Test Author",
					"download_url": "https://github.com/test/plugin/releases/download/v1.0.0/plugin.tar.gz",
					"checksum": "sha256:abcdef123456789",
					"created_at": "2023-01-01T00:00:00Z",
					"updated_at": "2023-01-01T00:00:00Z"
				}`)
			default:
				w.WriteHeader(http.StatusNotFound)
				_, _ = fmt.Fprint(w, `{"error": "Not found"}`)
			}
		}))

		config := sdk.RegistryConfig{
			BaseURL:   server.URL,
			Timeout:   30 * time.Second,
			Logger:    NewSilentLogger(),
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

	Describe("NewRegistryClient", func() {
		It("should create a new registry client", func() {
			config := sdk.RegistryConfig{
				BaseURL:   "https://registry.example.com",
				Timeout:   30 * time.Second,
			}
			client, err := sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())
			Expect(client).ToNot(BeNil())
		})

		It("should handle default timeout", func() {
			config := sdk.RegistryConfig{
				BaseURL:   "https://registry.example.com",
			}
			client, err := sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())
			Expect(client).ToNot(BeNil())
		})

		It("should validate BaseURL", func() {
			config := sdk.RegistryConfig{
				BaseURL: "ht!tp://invalid-url",
			}
			client, err := sdk.NewRegistryClient(config)
			Expect(err).To(HaveOccurred())
			Expect(client).To(BeNil())
		})
	})

	Describe("GetRegistry", func() {
		It("should get registry successfully", func() {
			registry, err := client.GetRegistry(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(registry).ToNot(BeNil())
		})

		Context("when server returns error", func() {
			BeforeEach(func() {
				server.Close()
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = fmt.Fprint(w, `{"error": "Internal server error"}`)
				}))

				config := sdk.RegistryConfig{
					BaseURL:   server.URL,
					Timeout:   30 * time.Second,
					Logger:    NewSilentLogger(),
				}
				var err error
				client, err = sdk.NewRegistryClient(config)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should handle server errors gracefully", func() {
				registry, err := client.GetRegistry(ctx)
				Expect(err).To(HaveOccurred())
				Expect(registry).To(BeNil())
			})
		})
	})

	Describe("GetPlugin", func() {
		It("should get plugin information", func() {
			plugin, err := client.GetPlugin(ctx, "test-plugin")
			Expect(err).ToNot(HaveOccurred())
			Expect(plugin).ToNot(BeNil())
			Expect(plugin.Name).To(Equal("test-plugin"))
		})

		It("should handle non-existent plugins", func() {
			plugin, err := client.GetPlugin(ctx, "nonexistent-plugin")
			Expect(err).To(HaveOccurred())
			Expect(plugin).To(BeNil())
		})

		It("should validate plugin names", func() {
			testCases := []struct {
				name        string
				shouldError bool
				description string
			}{
				{"", true, "empty name"},
				{"valid-plugin", false, "valid name"},
				{"plugin_with_underscore", false, "name with underscore"},
				{"plugin-with-dash", false, "name with dash"},
				{"plugin123", false, "name with numbers"},
				{"Plugin", false, "name with capital letter"},
				{"../invalid", true, "name with path traversal"},
				{"plugin/invalid", true, "name with slash"},
				{"plugin with spaces", true, "name with spaces"},
			}

			for _, tc := range testCases {
				By(fmt.Sprintf("validating %s", tc.description))
				_, err := client.GetPlugin(ctx, tc.name)
				if tc.shouldError && tc.name != "valid-plugin" {
					// We expect an error for invalid names, but the server might return different errors
					// so we just check that some error occurred
					Expect(err).To(HaveOccurred(), fmt.Sprintf("Expected error for %s", tc.description))
				}
			}
		})
	})

	Describe("Simple API", func() {
		It("should handle requests without authentication", func() {
			// Test that the simplified API works without authentication headers
			server.Close()
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simple API doesn't require authentication
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, `{"base_url": "https://registry.example.com", "version": "1.0", "last_updated": "2023-01-01T00:00:00Z", "plugins": {}}`)
			}))

			config := sdk.RegistryConfig{
				BaseURL:   server.URL,
				Timeout:   30 * time.Second,
				Logger:    NewSilentLogger(),
			}
			var err error
			client, err = sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())

			_, err = client.GetRegistry(ctx)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("Rate Limiting", func() {
		It("should handle rate limit responses", func() {
			server.Close()
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-RateLimit-Limit", "100")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", "1640995200")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = fmt.Fprint(w, `{"error": "Rate limit exceeded"}`)
			}))

			config := sdk.RegistryConfig{
				BaseURL:   server.URL,
				Timeout:   30 * time.Second,
				Logger:    NewSilentLogger(),
			}
			var err error
			client, err = sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())

			_, err = client.GetRegistry(ctx)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Rate limit exceeded"))
		})
	})

	Describe("Network Errors", func() {
		It("should handle connection failures", func() {
			// Create client with invalid URL to simulate connection failure
			config := sdk.RegistryConfig{
				BaseURL:   "http://localhost:99999", // Invalid port
				Timeout:   1 * time.Second, // Short timeout
				Logger:    NewSilentLogger(),
			}
			failClient, err := sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())

			_, err = failClient.GetRegistry(ctx)
			Expect(err).To(HaveOccurred())
		})

		It("should handle timeout errors", func() {
			server.Close()
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simulate slow response
				time.Sleep(2 * time.Second)
				w.WriteHeader(http.StatusOK)
			}))

			config := sdk.RegistryConfig{
				BaseURL:   server.URL,
				Timeout:   100 * time.Millisecond, // Very short timeout
				Logger:    NewSilentLogger(),
			}
			timeoutClient, err := sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()

			_, err = timeoutClient.GetRegistry(ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("SearchPlugins", func() {
		BeforeEach(func() {
			server.Close()
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
							"git-helper": {
								"name": "git-helper",
								"description": "Git workflow helper tool",
								"tags": ["git", "development", "tool"]
							},
							"docker-manager": {
								"name": "docker-manager",
								"description": "Docker container management utility",
								"tags": ["docker", "containers", "tool"]
							},
							"code-formatter": {
								"name": "code-formatter",
								"description": "Code formatting and linting tool",
								"tags": ["formatting", "development", "utility"]
							}
						}
					}`)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))

			config := sdk.RegistryConfig{
				BaseURL:   server.URL,
				Timeout:   30 * time.Second,
				Logger:    NewSilentLogger(),
			}
			var err error
			client, err = sdk.NewRegistryClient(config)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should search plugins by name", func() {
			plugins, err := client.SearchPlugins(ctx, "git", nil, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(HaveLen(1))
			Expect(plugins[0].Name).To(Equal("git-helper"))
		})

		It("should search plugins by description", func() {
			plugins, err := client.SearchPlugins(ctx, "docker", nil, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(HaveLen(1))
			Expect(plugins[0].Name).To(Equal("docker-manager"))
		})

		It("should search plugins by tags", func() {
			plugins, err := client.SearchPlugins(ctx, "", []string{"development"}, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(HaveLen(2))

			names := []string{plugins[0].Name, plugins[1].Name}
			Expect(names).To(ContainElement("git-helper"))
			Expect(names).To(ContainElement("code-formatter"))
		})

		It("should return empty results for no matches", func() {
			plugins, err := client.SearchPlugins(ctx, "nonexistent", nil, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(BeEmpty())
		})

		It("should respect search limit", func() {
			plugins, err := client.SearchPlugins(ctx, "", []string{"tool"}, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(HaveLen(1))
		})

		It("should handle case insensitive search", func() {
			plugins, err := client.SearchPlugins(ctx, "GIT", nil, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(HaveLen(1))
			Expect(plugins[0].Name).To(Equal("git-helper"))
		})

		It("should combine query and tag filters", func() {
			plugins, err := client.SearchPlugins(ctx, "helper", []string{"development"}, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(HaveLen(1))
			Expect(plugins[0].Name).To(Equal("git-helper"))
		})

		It("should handle empty query with no tags", func() {
			plugins, err := client.SearchPlugins(ctx, "", nil, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(plugins).To(HaveLen(3))
		})
	})
})

var _ = Describe("Secure Downloader", func() {
	var (
		server *httptest.Server
		tempDir string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "secure-downloader-test-*")
		Expect(err).ToNot(HaveOccurred())

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/secure-file":
				w.Header().Set("Content-Type", "application/octet-stream")
				w.WriteHeader(http.StatusOK)
				_, _ = fmt.Fprint(w, "secure file content")
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	It("should create a registry downloader", func() {
		registryConfig := sdk.RegistryConfig{
			BaseURL: server.URL,
		}
		downloaderConfig := sdk.DownloaderConfig{}
		downloader, err := sdk.NewRegistryDownloader(downloaderConfig, registryConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(downloader).ToNot(BeNil())
	})
})
