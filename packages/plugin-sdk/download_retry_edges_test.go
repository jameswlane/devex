package sdk_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Download retry edge cases", func() {
	var pluginDir, cacheDir, platformKey string
	var payload []byte
	BeforeEach(func() {
		pluginDir, cacheDir = GinkgoT().TempDir(), GinkgoT().TempDir()
		platformKey = runtime.GOOS + "-" + runtime.GOARCH
		payload = []byte(fakeExecutable)
	})
	newDownloader := func(url string) *sdk.Downloader {
		d := sdk.NewSecureDownloader(sdk.DownloaderConfig{RegistryURL: url, PluginDir: pluginDir, CacheDir: cacheDir, VerifyChecksums: true})
		d.SetSilent(true)
		d.SetRetryBackoff(func(context.Context, int, time.Duration) error { return nil })
		return d
	}
	writeMetadata := func(w http.ResponseWriter) {
		_ = json.NewEncoder(w).Encode(sdk.PluginMetadata{PluginInfo: sdk.PluginInfo{Name: "demo", Version: "1.0.0"}, Binaries: map[string]sdk.PlatformBinary{platformKey: {URL: "https://example.invalid/demo", Checksum: fmt.Sprintf("%x", sha256.Sum256(payload)), Size: int64(len(payload))}}})
	}
	DescribeTable("retries truncated successful response bodies",
		func(target string) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == target {
					requests++
					if requests == 1 {
						w.Header().Set("Content-Length", "1000")
						_, _ = w.Write([]byte("partial"))
						return
					}
				}
				switch r.URL.Path {
				case "/api/v1/registry":
					_ = json.NewEncoder(w).Encode(sdk.PluginRegistry{Plugins: map[string]sdk.PluginMetadata{"demo": {PluginInfo: sdk.PluginInfo{Name: "demo"}}}})
				case "/api/v1/plugins/demo":
					writeMetadata(w)
				default:
					_, _ = w.Write(payload)
				}
			}))
			defer server.Close()
			d := newDownloader(server.URL)
			if target == "/api/v1/registry" {
				plugins, err := d.GetAvailablePlugins(context.Background())
				Expect(err).NotTo(HaveOccurred())
				Expect(plugins).To(HaveKey("demo"))
			} else {
				Expect(d.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())
				Expect(filepath.Join(pluginDir, "devex-plugin-demo")).To(BeAnExistingFile())
				files, err := filepath.Glob(filepath.Join(pluginDir, ".devex-plugin-*-download-*"))
				Expect(err).NotTo(HaveOccurred())
				Expect(files).To(BeEmpty())
			}
			Expect(requests).To(Equal(2))
		},
		Entry("registry metadata", "/api/v1/registry"),
		Entry("plugin metadata", "/api/v1/plugins/demo"),
		Entry("plugin binary", "/api/v1/plugins/demo/download/"+runtime.GOOS+"-"+runtime.GOARCH),
	)
	It("shares the three-attempt budget between HTTP status and body failures", func() {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/plugins/demo" {
				writeMetadata(w)
				return
			}
			requests++
			if requests == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write([]byte("partial"))
		}))
		defer server.Close()
		err := newDownloader(server.URL).DownloadPluginWithContext(context.Background(), "demo")
		Expect(err).To(MatchError(ContainSubstring("unexpected EOF")))
		Expect(requests).To(Equal(3))
		Expect(filepath.Join(pluginDir, "devex-plugin-demo")).NotTo(BeAnExistingFile())
		files, err := filepath.Glob(filepath.Join(pluginDir, ".devex-plugin-*-download-*"))
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(BeEmpty())
	})
	DescribeTable("preserves context errors over stale cache using the default retry timer",
		func(deadline bool) {
			data, err := json.Marshal(sdk.PluginRegistry{LastUpdated: time.Now().Add(-24 * time.Hour), Plugins: map[string]sdk.PluginMetadata{"stale": {}}})
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(cacheDir, "registry.json"), data, 0600)).To(Succeed())
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer server.Close()
			d := newDownloader(server.URL)
			d.SetRetryBackoff(nil)
			ctx, cancel := context.WithCancel(context.Background())
			expected := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
				expected = context.DeadlineExceeded
			} else {
				go func() {
					defer GinkgoRecover()
					Eventually(requests.Load).Should(Equal(int32(1)))
					cancel()
				}()
			}
			defer cancel()
			plugins, err := d.GetAvailablePlugins(ctx)
			Expect(errors.Is(err, expected)).To(BeTrue())
			Expect(plugins).To(BeNil())
			Expect(requests.Load()).To(Equal(int32(1)))
		},
		Entry("cancellation", false), Entry("deadline", true),
	)
	It("clamps numeric Retry-After before duration multiplication can overflow", func() {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requests++
			w.Header().Set("Retry-After", "9223372036854775807")
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()
		d := newDownloader(server.URL)
		d.SetRetryBackoff(func(_ context.Context, _ int, delay time.Duration) error {
			Expect(delay).To(Equal(time.Second))
			return nil
		})
		_, err := d.GetAvailablePlugins(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(requests).To(Equal(3))
	})
	It("does not classify truncated bodies as an unreachable registry", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "1000")
			_, _ = w.Write([]byte("partial"))
		}))
		defer server.Close()
		_, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
		var unavailable *sdk.RegistryUnavailableError
		Expect(errors.As(err, &unavailable)).To(BeFalse())
		Expect(err).To(MatchError(ContainSubstring("unexpected EOF")))
	})
	It("preserves earlier download failures when a later connection fails", func() {
		closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		closed.Close()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/plugins/package-manager-apt" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, closed.URL, http.StatusTemporaryRedirect)
		}))
		defer server.Close()
		d := newDownloader(server.URL)
		d.SetStrategy(sdk.ContinueOnError)
		err := d.DownloadRequiredPluginsWithContext(context.Background(), []string{"package-manager-apt", "later", "unattempted"})
		var multi *sdk.MultiError
		Expect(errors.As(err, &multi)).To(BeTrue())
		Expect(multi.Errors).To(HaveLen(2))
		Expect(multi.Errors[0]).To(MatchError(ContainSubstring("HTTP 500")))
		var op *net.OpError
		Expect(errors.As(multi.Errors[1], &op)).To(BeTrue())
		Expect(op.Op).To(Equal("dial"))
	})
})
