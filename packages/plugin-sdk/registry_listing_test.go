package sdk_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// registryPage renders one page of /api/v1/registry in the shape
// registry.devex.sh serves, trimmed to the fields the SDK reads.
func registryPage(page, totalPages int, plugins ...string) string {
	entries := ""
	for i, name := range plugins {
		if i > 0 {
			entries += ","
		}
		entries += fmt.Sprintf(`{
			"name": %q,
			"description": "plugin %s",
			"type": "package-manager",
			"priority": 10,
			"status": "active",
			"platforms": ["linux"],
			"version": "0.3.1",
			"latestVersion": "0.3.1",
			"binaries": {"linux-amd64": {"url": "https://example.invalid/%s", "size": 1, "format": "binary", "checksum": "abc"}}
		}`, name, name, name)
	}
	return fmt.Sprintf(`{
		"base_url": "https://github.com/jameswlane/devex/releases/download",
		"version": "2.0.0",
		"last_updated": "2026-01-01T00:00:00.000Z",
		"source": "database",
		"data": {"plugins": [%s], "applications": [], "configs": [], "stacks": []},
		"pagination": {"page": %d, "limit": 100, "totalPages": %d, "totalItems": {"plugins": %d}}
	}`, entries, page, totalPages, len(plugins))
}

var _ = Describe("Downloader registry listing", func() {
	var cacheDir string

	BeforeEach(func() {
		cacheDir = GinkgoT().TempDir()
	})

	newDownloader := func(registryURL string) *sdk.Downloader {
		d := sdk.NewSecureDownloader(sdk.DownloaderConfig{
			RegistryURL: registryURL,
			PluginDir:   GinkgoT().TempDir(),
			CacheDir:    cacheDir,
		})
		d.SetSilent(true)
		d.SetRetryBackoff(func(context.Context, int, time.Duration) error { return nil })
		return d
	}

	It("reads plugins from the registry's paginated response", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/api/v1/registry"))
			Expect(r.URL.Query().Get("resource")).To(Equal("plugins"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, registryPage(1, 1, "tool-git", "tool-stackdetector"))
		}))
		defer server.Close()

		plugins, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(plugins).To(HaveKey("tool-git"))
		Expect(plugins["tool-stackdetector"].Version).To(Equal("0.3.1"))
		Expect(plugins["tool-git"].Type).To(Equal("package-manager"))
		Expect(plugins["tool-git"].Binaries).To(HaveKey("linux-amd64"))
	})

	It("follows every page of the listing", func() {
		var pages []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			page := r.URL.Query().Get("page")
			pages = append(pages, page)
			w.Header().Set("Content-Type", "application/json")
			if page == "2" {
				_, _ = fmt.Fprint(w, registryPage(2, 2, "tool-shell"))
				return
			}
			_, _ = fmt.Fprint(w, registryPage(1, 2, "tool-git"))
		}))
		defer server.Close()

		plugins, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(plugins).To(HaveKey("tool-git"))
		Expect(plugins).To(HaveKey("tool-shell"))
		Expect(pages).To(Equal([]string{"1", "2"}))
	})

	It("rejects a listing with more pages than it reads instead of truncating it", func() {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, registryPage(1, 101, "tool-git"))
		}))
		defer server.Close()

		_, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
		Expect(err).To(MatchError(ContainSubstring("registry listing has 101 pages")))
		Expect(requests).To(Equal(1))
	})

	DescribeTable("reports a listing without plugins as an error",
		func(body string) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, body)
			}))
			defer server.Close()

			plugins, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
			Expect(err).To(MatchError(ContainSubstring("registry response contained no plugins")))
			Expect(plugins).To(BeEmpty())
		},
		Entry("unknown shape", `{"items": [{"name": "tool-git"}]}`),
		Entry("empty paginated listing", registryPage(1, 1)),
	)

	It("ages the cache from fetch time, not the server's last_updated", func() {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Content-Type", "application/json")
			// registryPage reports a last_updated far older than the cache duration.
			_, _ = fmt.Fprint(w, registryPage(1, 1, "tool-git"))
		}))
		defer server.Close()

		d := newDownloader(server.URL)
		_, err := d.GetAvailablePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		plugins, err := d.GetAvailablePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(plugins).To(HaveKey("tool-git"))
		Expect(requests).To(Equal(1))
	})

	It("ignores a cached registry with no plugins", func() {
		stale := []byte(fmt.Sprintf(`{"base_url": "", "version": "", "last_updated": %q, "plugins": null}`, time.Now().UTC().Format(time.RFC3339)))
		Expect(os.WriteFile(filepath.Join(cacheDir, "registry.json"), stale, 0o644)).To(Succeed())
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, registryPage(1, 1, "tool-git"))
		}))
		defer server.Close()

		plugins, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(plugins).To(HaveKey("tool-git"))
	})

	Context("when a later listing page fails", func() {
		writeExpiredCache := func() {
			data, err := json.Marshal(sdk.PluginRegistry{
				LastUpdated: time.Now().Add(-24 * time.Hour),
				Plugins:     map[string]sdk.PluginMetadata{"cached-plugin": {}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(cacheDir, "registry.json"), data, 0o600)).To(Succeed())
		}

		// pagedServer serves page 1 of a two-page listing and hands page 2 to onPage2.
		pagedServer := func(onPage2 func(http.ResponseWriter, *http.Request)) *httptest.Server {
			return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") == "2" {
					onPage2(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, registryPage(1, 2, "tool-git"))
			}))
		}

		dropConnection := func(w http.ResponseWriter, _ *http.Request) {
			conn, _, err := w.(http.Hijacker).Hijack()
			Expect(err).NotTo(HaveOccurred())
			_ = conn.Close()
		}

		It("falls back to an expired cache on a transport failure", func() {
			writeExpiredCache()
			server := pagedServer(dropConnection)
			defer server.Close()

			plugins, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
			Expect(err).NotTo(HaveOccurred())
			Expect(plugins).To(HaveKey("cached-plugin"))
			Expect(plugins).NotTo(HaveKey("tool-git"))
		})

		It("reports a transport failure when there is no cache", func() {
			server := pagedServer(dropConnection)
			defer server.Close()

			_, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
			Expect(err).To(MatchError(ContainSubstring("failed to fetch registry page 2")))
		})

		It("keeps a non-200 response an error even with a cache", func() {
			writeExpiredCache()
			server := pagedServer(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			})
			defer server.Close()

			_, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
			Expect(err).To(MatchError(ContainSubstring("registry server returned HTTP 404")))
		})

		It("keeps an unparseable page an error even with a cache", func() {
			writeExpiredCache()
			server := pagedServer(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = fmt.Fprint(w, "not json")
			})
			defer server.Close()

			_, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
			Expect(err).To(MatchError(ContainSubstring("failed to parse registry page 2")))
		})

		It("returns the context error when cancelled during a later page", func() {
			writeExpiredCache()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := pagedServer(func(w http.ResponseWriter, r *http.Request) {
				cancel()
				dropConnection(w, r)
			})
			defer server.Close()

			_, err := newDownloader(server.URL).GetAvailablePlugins(ctx)
			Expect(err).To(MatchError(context.Canceled))
		})
	})
})
