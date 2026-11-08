package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCatalog(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Registry catalog") }

var _ = Describe("Registry catalog", func() {
	var fixture []byte
	var server *httptest.Server
	var client *Client
	BeforeEach(func() {
		var err error
		fixture, err = os.ReadFile("testdata/applications.json")
		Expect(err).NotTo(HaveOccurred())
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(fixture) }))
		client = NewClient(server.URL)
	})
	AfterEach(func() { server.Close() })
	It("maps the route and seed contract to existing CLI types", func() {
		apps, _, _, err := client.List(context.Background(), Filters{}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(1))
		Expect(apps[0]).To(Equal(types.CrossPlatformApp{Name: "git", Description: "A distributed version control system", Category: "Dependency", DesktopEnvironments: []string{}, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}}))
	})
	It("reads the unwrapped detail response by escaped name", func() {
		detail, err := os.ReadFile("testdata/application.json")
		Expect(err).NotTo(HaveOccurred())
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/api/v1/applications/git"))
			_, _ = w.Write(detail)
		})
		app, err := client.Get(context.Background(), "git")
		Expect(err).NotTo(HaveOccurred())
		Expect(app.Linux.InstallCommand).To(Equal("git"))
	})
	It("preserves platform alternatives and desktop restrictions", func() {
		a := Application{Name: "tool", DesktopEnvironments: []string{"gnome"}, Platforms: map[string]*Platform{"linux": {InstallMethod: "apt", InstallCommand: "tool", Alternatives: []Alternative{{Method: "dnf", Command: "tool"}}}}}
		app, err := Map(a)
		Expect(err).NotTo(HaveOccurred())
		Expect(app.Linux.Alternatives).To(ConsistOf(types.OSConfig{InstallMethod: "dnf", InstallCommand: "tool"}))
		Expect(app.DesktopEnvironments).To(Equal([]string{"gnome"}))
	})
	It("paginates and forwards search, category, and platform filters", func() {
		calls := 0
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			Expect(r.URL.Query().Get("search")).To(Equal("git"))
			Expect(r.URL.Query().Get("category")).To(Equal("development"))
			Expect(r.URL.Query().Get("platform")).To(Equal("linux"))
			var response map[string]any
			Expect(json.Unmarshal(fixture, &response)).To(Succeed())
			pagination := response["pagination"].(map[string]any)
			pagination["page"] = calls
			pagination["hasNext"] = calls == 1
			Expect(json.NewEncoder(w).Encode(response)).To(Succeed())
		})
		apps, tag, _, err := client.List(context.Background(), Filters{Search: "git", Category: "development", Platform: "linux"}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(2))
		Expect(calls).To(Equal(2))
		Expect(tag).To(BeEmpty())
	})
	It("returns typed HTTP status errors", func() {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) })
		_, _, _, err := client.List(context.Background(), Filters{}, "")
		var apiErr *APIError
		Expect(errors.As(err, &apiErr)).To(BeTrue())
		Expect(apiErr.Status).To(Equal(503))
	})
	It("rejects malformed contracts rather than caching empty catalogs", func() {
		fixture = []byte(`{"applications":[]}`)
		_, _, _, err := client.List(context.Background(), Filters{}, "")
		Expect(err).To(HaveOccurred())
	})
	It("honors timeout and caller cancellation", func() {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
		client.HTTP.Timeout = 10 * time.Millisecond
		_, _, _, err := client.List(context.Background(), Filters{}, "")
		Expect(err).To(HaveOccurred())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _, _, err = client.List(ctx, Filters{}, "")
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})
	It("uses TTL then ETag revalidation, and falls back to stale cache offline", func() {
		calls := 0
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(304)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write(fixture)
		})
		store := Store{Client: client, Dir: GinkgoT().TempDir(), TTL: time.Hour}
		Expect(store.Load(context.Background(), nil).Source).To(Equal("registry"))
		Expect(store.Load(context.Background(), nil).Source).To(Equal("cache"))
		Expect(calls).To(Equal(1))
		store.TTL = 0
		Expect(store.Load(context.Background(), nil).Source).To(Equal("cache (revalidated)"))
		Expect(calls).To(Equal(2))
		store.Offline = true
		result := store.Load(context.Background(), nil)
		Expect(result.Source).To(Equal("cache (stale)"))
		Expect(result.Apps).To(HaveLen(1))
		Expect(calls).To(Equal(2))
	})
	It("uses stale cache on 5xx and bundled data without cache", func() {
		store := Store{Client: client, Dir: GinkgoT().TempDir()}
		Expect(store.Load(context.Background(), nil).Source).To(Equal("registry"))
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) })
		result := store.Load(context.Background(), nil)
		Expect(result.Source).To(Equal("cache (stale)"))
		Expect(result.Warning).To(HaveOccurred())
		store.Dir = GinkgoT().TempDir()
		result = store.Load(context.Background(), []types.CrossPlatformApp{{Name: "bundled"}})
		Expect(result.Source).To(Equal("bundled"))
		Expect(result.Apps[0].Name).To(Equal("bundled"))
		store.Offline = true
		Expect(store.Load(context.Background(), nil).Source).To(Equal("bundled"))
	})
	It("returns a typed 404 for missing detail entries", func() {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
		_, err := client.Get(context.Background(), "missing")
		var apiErr *APIError
		Expect(errors.As(err, &apiErr)).To(BeTrue())
		Expect(apiErr.Status).To(Equal(404))
	})
	It("does not replace a complete cache with a failed continuation page", func() {
		store := Store{Client: client, Dir: GinkgoT().TempDir()}
		Expect(store.Load(context.Background(), nil).Source).To(Equal("registry"))
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("page") == "2" {
				w.WriteHeader(503)
				return
			}
			_, _ = w.Write(bytes.ReplaceAll(fixture, []byte(`"hasNext": false`), []byte(`"hasNext": true`)))
		})
		result := store.Load(context.Background(), nil)
		Expect(result.Source).To(Equal("cache (stale)"))
		Expect(result.Apps).To(HaveLen(1))
		Expect(result.Warning).To(HaveOccurred())
	})
	It("isolates registry caches and tolerates unwritable cache storage", func() {
		dir := GinkgoT().TempDir()
		store := Store{Client: client, Dir: dir}
		Expect(store.Load(context.Background(), nil).Source).To(Equal("registry"))
		store.Client = NewClient("http://127.0.0.1:1")
		store.Offline = true
		Expect(store.Load(context.Background(), nil).Source).To(Equal("bundled"))
		file := filepath.Join(dir, "file")
		Expect(os.WriteFile(file, []byte("file"), 0600)).To(Succeed())
		store.Client = client
		store.Dir = file
		store.Offline = false
		result := store.Load(context.Background(), nil)
		Expect(result.Source).To(Equal("registry"))
		Expect(result.Warning).To(HaveOccurred())
	})

})
