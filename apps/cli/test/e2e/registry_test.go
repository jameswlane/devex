//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// sourceRegistry keeps real plugin download/checksum verification in the path.
// Only staged plugins are overridden; other requests use the selected registry.
func sourceRegistry(upstream string, metadata map[string]sdk.PluginMetadata, files map[string]string) *httptest.Server {
	target, err := url.Parse(upstream)
	Expect(err).NotTo(HaveOccurred())
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(target) }}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 4 && strings.Join(parts[:3], "/") == "api/v1/plugins" {
			name := parts[3]
			if entry, ok := metadata[name]; ok {
				switch {
				case len(parts) == 4 || (len(parts) == 6 && parts[4] == "versions" && parts[5] == "latest"):
					entry.Binaries = maps.Clone(entry.Binaries)
					for platform, binary := range entry.Binaries {
						binary.URL = fmt.Sprintf("http://%s/api/v1/plugins/%s/download/%s", r.Host, name, platform)
						entry.Binaries[platform] = binary
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(entry)
					return
				case len(parts) == 6 && parts[4] == "download" && parts[5] == runtime.GOOS+"-"+runtime.GOARCH:
					http.ServeFile(w, r, files[name])
					return
				}
			}
		}
		proxy.ServeHTTP(w, r)
	}))
}

func configureSourceRegistry() {
	directory := filepath.Join(envDefault("DEVEX_E2E_TESTDATA", "/opt/devex-e2e/testdata"), "plugins")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
	if len(entries) == 0 {
		return
	}
	metadata := map[string]sdk.PluginMetadata{}
	files := map[string]string{}
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		output, err := sandbox.run(sandbox.home, path, 15*time.Second, nil, "--plugin-info")
		Expect(err).NotTo(HaveOccurred(), "source plugin metadata: %s", output)
		var info sdk.PluginMetadata
		Expect(json.Unmarshal([]byte(output), &info)).To(Succeed())
		Expect(info.Name).To(Equal(entry.Name()))
		stat, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		info.Binaries = map[string]sdk.PlatformBinary{runtime.GOOS + "-" + runtime.GOARCH: {Checksum: checksum(path), Size: stat.Size(), OS: runtime.GOOS, Arch: runtime.GOARCH}}
		metadata[info.Name], files[info.Name] = info, path
		GinkgoWriter.Printf("Source plugin %s SHA256 %s\n", info.Name, checksum(path))
	}
	server := sourceRegistry(sandbox.registry, metadata, files)
	DeferCleanup(server.Close)
	sandbox.registry = server.URL
}

var _ = Describe("Source plugin registry", Label("registry"), func() {
	It("serves source metadata and binaries while forwarding other plugins", func() {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "upstream:"+r.URL.Path) }))
		defer upstream.Close()
		file := filepath.Join(sandbox.work, "source-plugin")
		Expect(os.WriteFile(file, []byte("source-binary"), 0600)).To(Succeed())
		server := sourceRegistry(upstream.URL, map[string]sdk.PluginMetadata{"demo": {PluginInfo: sdk.PluginInfo{Name: "demo", Version: "test"}, Binaries: map[string]sdk.PlatformBinary{runtime.GOOS + "-" + runtime.GOARCH: {URL: upstream.URL + "/source", Checksum: checksum(file), Size: int64(len("source-binary"))}}}}, map[string]string{"demo": file})
		defer server.Close()
		destination := filepath.Join(sandbox.work, "downloaded-source")
		downloader := sdk.NewDownloader(server.URL, destination)
		Expect(downloader.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())
		Expect(checksum(filepath.Join(destination, "devex-plugin-demo"))).To(Equal(checksum(file)))
		for path, expected := range map[string]string{
			"/api/v1/plugins/demo":                                                 `"version":"test"`,
			"/api/v1/plugins/demo/versions/latest":                                 `"name":"demo"`,
			"/api/v1/plugins/demo/download/" + runtime.GOOS + "-" + runtime.GOARCH: "source-binary",
			"/api/v1/plugins/other":                                                "upstream:/api/v1/plugins/other",
		} {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
			Expect(err).NotTo(HaveOccurred())
			response, err := http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
			var output strings.Builder
			_, err = io.Copy(&output, response.Body)
			_ = response.Body.Close()
			cancel()
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			Expect(output.String()).To(ContainSubstring(expected))
		}
	})
})
