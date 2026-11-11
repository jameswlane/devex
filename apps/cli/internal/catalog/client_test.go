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
	It("maps a recorded registry response, keeping platform alternatives and requirements", func() {
		apps, _, _, err := client.List(context.Background(), Filters{}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(2))
		Expect(apps[0].Name).To(Equal("Eza"))
		Expect(apps[0].Category).To(Equal("System Utilities"))
		Expect(apps[0].Linux.Alternatives).NotTo(BeEmpty())
		pacman := apps[0].Linux.Alternatives[0]
		Expect(pacman.InstallMethod).To(Equal("pacman"))
		Expect(pacman.InstallCommand).To(Equal("eza"))
		Expect(pacman.PlatformRequirements).To(ContainElement(types.PlatformRequirement{OS: "arch"}))
		Expect(apps[1].Name).To(Equal("GitHub CLI"))
		Expect(apps[1].Linux.Alternatives[0].PlatformRequirements).To(ContainElement(types.PlatformRequirement{OS: "fedora"}))
	})
	It("reads the recorded detail response by escaped name", func() {
		detail, err := os.ReadFile("testdata/application.json")
		Expect(err).NotTo(HaveOccurred())
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			Expect(r.URL.Path).To(Equal("/api/v1/applications/git"))
			_, _ = w.Write(detail)
		})
		app, err := client.Get(context.Background(), "git")
		Expect(err).NotTo(HaveOccurred())
		Expect(app.Linux.InstallMethod).To(Equal("apt"))
		Expect(app.Linux.InstallCommand).To(Equal("git"))
	})
	It("accepts the camelCase platform keys older registry seeds wrote", func() {
		a := Application{Name: "tool", DesktopEnvironments: []string{"gnome"}, Platforms: map[string]json.RawMessage{
			"linux": json.RawMessage(`{"installMethod":"apt","installCommand":"tool","officialSupport":true,"alternatives":[{"method":"dnf","command":"tool","platformRequirements":[{"os":"fedora"}]}]}`),
			"macos": json.RawMessage(`null`),
		}}
		app, err := Map(a)
		Expect(err).NotTo(HaveOccurred())
		Expect(app.Linux.InstallMethod).To(Equal("apt"))
		Expect(app.Linux.OfficialSupport).To(BeTrue())
		Expect(app.Linux.Alternatives).To(HaveLen(1))
		Expect(app.Linux.Alternatives[0].InstallMethod).To(Equal("dnf"))
		Expect(app.Linux.Alternatives[0].PlatformRequirements).To(ConsistOf(types.PlatformRequirement{OS: "fedora"}))
		Expect(app.MacOS).To(Equal(types.OSConfig{}))
		Expect(app.DesktopEnvironments).To(Equal([]string{"gnome"}))
	})
	It("keeps the snake_case fields the YAML catalog defines", func() {
		a := Application{Name: "tool", Platforms: map[string]json.RawMessage{
			"linux": json.RawMessage(`{"install_method":"apt","install_command":"tool","uninstall_command":"tool","dependencies":["curl"],"apt_sources":[{"key_source":"https://example.invalid/key.gpg","key_name":"/etc/apt/keyrings/tool.gpg","source_repo":"deb https://example.invalid stable main","source_name":"/etc/apt/sources.list.d/tool.list"}]}`),
		}}
		app, err := Map(a)
		Expect(err).NotTo(HaveOccurred())
		Expect(app.Linux.UninstallCommand).To(Equal("tool"))
		Expect(app.Linux.Dependencies).To(Equal([]string{"curl"}))
		Expect(app.Linux.AptSources).To(HaveLen(1))
	})
	It("rejects a platform that is not an object", func() {
		_, err := Map(Application{Name: "tool", Platforms: map[string]json.RawMessage{"linux": json.RawMessage(`"apt"`)}})
		Expect(err).To(MatchError(ContainSubstring("platform linux")))
	})
	Describe("WithBundled", func() {
		bundled := []types.CrossPlatformApp{
			{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git", Dependencies: []string{"ca-certificates"}}},
			{Name: "Docker", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "docker-ce"}},
		}
		It("keeps every bundled definition when the registry is partial or lossy", func() {
			registry := []types.CrossPlatformApp{{Name: "GIT", Linux: types.OSConfig{Alternatives: []types.OSConfig{{InstallMethod: "dnf", InstallCommand: "git"}}}}}
			merged := WithBundled(bundled, registry)
			Expect(merged).To(Equal(bundled))
		})
		It("adds installable registry applications the binary does not ship", func() {
			registry := []types.CrossPlatformApp{
				{Name: "newtool", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "newtool"}},
				{Name: "empty"},
				{Name: "alt-only", Linux: types.OSConfig{Alternatives: []types.OSConfig{{InstallMethod: "dnf", InstallCommand: "alt"}}}},
			}
			merged := WithBundled(bundled, registry)
			names := make([]string, 0, len(merged))
			for _, app := range merged {
				names = append(names, app.Name)
			}
			Expect(names).To(Equal([]string{"git", "Docker", "newtool", "alt-only"}))
		})
		It("returns the bundled applications when the registry is empty", func() {
			Expect(WithBundled(bundled, nil)).To(Equal(bundled))
		})
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
		Expect(apps).To(HaveLen(4))
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
		Expect(result.Apps).To(HaveLen(2))
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
		Expect(result.Apps).To(HaveLen(2))
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
