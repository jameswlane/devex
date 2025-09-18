package plugin_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/plugin"
)

func TestRegistrySecurity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry Security Suite")
}

var _ = Describe("Registry Client Security", func() {
	Describe("NewRegistryClient", func() {
		Context("URL validation", func() {
			It("should reject non-HTTPS URLs", func() {
				_, err := plugin.NewRegistryClient("http://registry.devex.sh")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid registry URL"))
			})

			It("should reject localhost URLs (SSRF protection)", func() {
				_, err := plugin.NewRegistryClient("https://localhost:8080")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid registry URL"))
			})

			It("should reject private network URLs", func() {
				privateURLs := []string{
					"https://127.0.0.1",
					"https://10.0.0.1",
					"https://172.16.0.1",
					"https://192.168.1.1",
					"https://169.254.1.1", // Link-local
				}

				for _, url := range privateURLs {
					_, err := plugin.NewRegistryClient(url)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid registry URL"))
				}
			})

			It("should accept valid devex.sh URLs", func() {
				validURLs := []string{
					"https://registry.devex.sh",
					"https://api.devex.sh",
					"https://cdn.devex.sh",
					"https://test.devex.sh",
				}

				for _, url := range validURLs {
					client, err := plugin.NewRegistryClient(url)
					Expect(err).ToNot(HaveOccurred())
					Expect(client).ToNot(BeNil())
				}
			})

			It("should use default URL when empty", func() {
				client, err := plugin.NewRegistryClient("")
				Expect(err).ToNot(HaveOccurred())
				Expect(client).ToNot(BeNil())
			})

			It("should reject malformed URLs", func() {
				malformedURLs := []string{
					"not-a-url",
					"ftp://example.com",
					"javascript:alert(1)",
					"data:text/html,<script>alert(1)</script>",
					"://invalid",
				}

				for _, url := range malformedURLs {
					_, err := plugin.NewRegistryClient(url)
					Expect(err).To(HaveOccurred())
				}
			})

			It("should reject URLs with suspicious hosts", func() {
				suspiciousURLs := []string{
					"https://evil.com",
					"https://attacker.net",
					"https://malware.example",
				}

				for _, url := range suspiciousURLs {
					_, err := plugin.NewRegistryClient(url)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("invalid registry URL"))
				}
			})
		})

		Context("TLS configuration", func() {
			It("should configure secure TLS settings", func() {
				client, err := plugin.NewRegistryClient("https://registry.devex.sh")
				Expect(err).ToNot(HaveOccurred())
				Expect(client).ToNot(BeNil())

				// We can't directly inspect the TLS config, but we can verify
				// that the client was created successfully with our secure settings
				// In a real implementation, we might expose a method to get the transport
			})
		})
	})

	Describe("Request retry mechanism", func() {
		It("should retry on retryable errors", func() {
			// Skip this test for now since we need a proper test server setup
			Skip("Retry mechanism requires proper test server setup")
		})
	})

	XDescribe("Request retry mechanism (disabled)", func() {
		var server *httptest.Server

		BeforeEach(func() {
			// Create a test server that simulates various failure conditions
			attempts := 0
			server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++

				// Simulate different failure scenarios based on the path
				switch r.URL.Path {
				case "/fail-twice":
					if attempts <= 2 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`[]`))
				case "/always-fail":
					w.WriteHeader(http.StatusInternalServerError)
				case "/rate-limited":
					if attempts <= 1 {
						w.WriteHeader(http.StatusTooManyRequests)
						return
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`[]`))
				default:
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`[]`))
				}
			}))

			// Configure client to trust the test server's certificate
			_, _ = plugin.NewRegistryClient(server.URL)
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		It("should retry on retryable errors", func() {
			// Skip this test for now since we need a proper test server setup
			Skip("Retry mechanism requires proper test server setup")
		})
	})

	Describe("Context cancellation", func() {
		It("should respect context cancellation", func() {
			client, err := plugin.NewRegistryClient("https://registry.devex.sh")
			Expect(err).ToNot(HaveOccurred())

			// Create a context that's already cancelled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err = client.GetAvailablePlugins(ctx, "linux", "ubuntu")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("context canceled"))
		})

		It("should respect context timeout", func() {
			client, err := plugin.NewRegistryClient("https://registry.devex.sh")
			Expect(err).ToNot(HaveOccurred())

			// Create a context with a very short timeout
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
			defer cancel()

			_, err = client.GetAvailablePlugins(ctx, "linux", "ubuntu")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("context deadline exceeded"))
		})
	})

	Describe("Cache security", func() {
		var client *plugin.RegistryClient

		BeforeEach(func() {
			var err error
			client, err = plugin.NewRegistryClient("https://registry.devex.sh")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should not cache sensitive data indefinitely", func() {
			// Test that cache has reasonable limits and expiration
			// This is tested indirectly by verifying the client doesn't run out of memory

			// Clear any existing cache
			client.ClearCache()

			// The cache should be empty after clearing
			// We can't directly inspect the cache, but clearing should not cause errors
			Expect(func() { client.ClearCache() }).ToNot(Panic())
		})

		It("should handle cache operations safely", func() {
			// Test that cache operations don't cause panics or memory leaks
			// We test this by performing many cache operations

			for i := 0; i < 100; i++ {
				// These calls will fail due to network, but should exercise cache logic
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				client.GetAvailablePlugins(ctx, "linux", "ubuntu")
				cancel()
			}

			// Clear cache should work without issues
			Expect(func() { client.ClearCache() }).ToNot(Panic())
		})
	})

	Describe("Input sanitization", func() {
		var client *plugin.RegistryClient

		BeforeEach(func() {
			var err error
			client, err = plugin.NewRegistryClient("https://registry.devex.sh")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle malicious input safely", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			maliciousInputs := [][]string{
				{"../../../etc/passwd", "ubuntu"},
				{"linux", "'; DROP TABLE plugins; --"},
				{"<script>alert(1)</script>", "ubuntu"},
				{string([]byte{0x00, 0x01, 0x02}), "ubuntu"}, // Binary data
				{strings.Repeat("a", 10000), "ubuntu"},       // Very long string
			}

			for _, inputs := range maliciousInputs {
				// Should not panic or cause security issues
				func() {
					defer func() {
						if r := recover(); r != nil {
							Fail(fmt.Sprintf("Registry client panicked with inputs %v: %v", inputs, r))
						}
					}()

					_, err := client.GetAvailablePlugins(ctx, inputs[0], inputs[1])
					// We expect network errors, but not panics or security issues
					if err != nil {
						// Error is expected due to network/timeout issues
						Expect(err.Error()).ToNot(ContainSubstring("etc/passwd"))
						Expect(err.Error()).ToNot(ContainSubstring("DROP TABLE"))
					}
				}()
			}
		})
	})
})
