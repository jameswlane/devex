package sdk_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Registry Security", func() {
	var (
		server *httptest.Server
		client *sdk.RegistryClient
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Log the request path for debugging
			GinkgoWriter.Printf("Request path: %s\n", r.URL.Path)

			// This should never be reached for malicious paths
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name": "test", "version": "1.0.0"}`))
		}))

		config := sdk.RegistryConfig{
			BaseURL: server.URL,
			Timeout: 30 * time.Second,
			Logger:  NewSilentLogger(),
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

	Describe("Plugin Name Validation", func() {
		Context("Path Traversal Prevention", func() {
			It("should reject plugin names with ..", func() {
				_, err := client.GetPlugin(ctx, "../etc/passwd")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid plugin name"))
				Expect(err.Error()).To(ContainSubstring("path separators or traversal"))
			})

			It("should reject plugin names with forward slashes", func() {
				_, err := client.GetPlugin(ctx, "plugin/../../etc/passwd")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid plugin name"))
			})

			It("should reject plugin names with backslashes", func() {
				_, err := client.GetPlugin(ctx, "plugin\\..\\..\\etc\\passwd")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid plugin name"))
			})

			It("should reject absolute paths", func() {
				_, err := client.GetPlugin(ctx, "/etc/passwd")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid plugin name"))
			})

			It("should reject Windows absolute paths", func() {
				_, err := client.GetPlugin(ctx, "C:\\Windows\\System32")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid plugin name"))
			})
		})

		Context("Name Format Validation", func() {
			It("should reject empty plugin names", func() {
				_, err := client.GetPlugin(ctx, "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin name cannot be empty"))
			})

			It("should reject excessively long plugin names", func() {
				longName := string(make([]byte, 101))
				for i := range longName {
					longName = longName[:i] + "a" + longName[i+1:]
				}
				_, err := client.GetPlugin(ctx, longName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin name too long"))
			})

			It("should reject names with special characters", func() {
				invalidNames := []string{
					"plugin$name",
					"plugin@name",
					"plugin#name",
					"plugin%name",
					"plugin&name",
					"plugin*name",
					"plugin(name",
					"plugin)name",
					"plugin=name",
					"plugin+name",
					"plugin[name",
					"plugin]name",
					"plugin{name",
					"plugin}name",
					"plugin|name",
					"plugin;name",
					"plugin:name",
					"plugin'name",
					"plugin\"name",
					"plugin<name",
					"plugin>name",
					"plugin?name",
					"plugin,name",
					"plugin.name",
					"plugin`name",
					"plugin~name",
					"plugin!name",
					"plugin name", // space
				}

				for _, name := range invalidNames {
					_, err := client.GetPlugin(ctx, name)
					Expect(err).To(HaveOccurred(), "Should reject: %s", name)
					Expect(err.Error()).To(ContainSubstring("invalid plugin name"))
				}
			})

			It("should accept valid plugin names", func() {
				validNames := []string{
					"plugin-name",
					"plugin_name",
					"plugin123",
					"123plugin",
					"a",
					"A",
					"Plugin-Name_123",
					"devex-plugin-docker",
				}

				// Mock server for valid requests
				testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{
						"name": "test-plugin",
						"version": "1.0.0",
						"description": "A test plugin"
					}`))
				}))
				defer testServer.Close()

				config := sdk.RegistryConfig{
					BaseURL: testServer.URL,
					Timeout: 30 * time.Second,
					Logger:  NewSilentLogger(),
				}
				validClient, err := sdk.NewRegistryClient(config)
				Expect(err).ToNot(HaveOccurred())

				for _, name := range validNames {
					_, err := validClient.GetPlugin(ctx, name)
					// We expect no validation error (might get network error which is fine)
					if err != nil {
						Expect(err.Error()).ToNot(ContainSubstring("invalid plugin name"), "Should accept: %s", name)
					}
				}
			})

			It("should reject names starting with special characters", func() {
				invalidNames := []string{
					"-plugin",
					"_plugin",
					".plugin",
				}

				for _, name := range invalidNames {
					_, err := client.GetPlugin(ctx, name)
					Expect(err).To(HaveOccurred(), "Should reject: %s", name)
					Expect(err.Error()).To(ContainSubstring("invalid plugin name"))
				}
			})
		})
	})
})
