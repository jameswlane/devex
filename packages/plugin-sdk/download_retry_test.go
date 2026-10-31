package sdk_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

type retryWarningLogger struct {
	SilentLogger
	warnings []string
}

func (l *retryWarningLogger) Warning(msg string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(msg, args...))
}

var _ = Describe("Plugin download retries", func() {
	var (
		pluginDir   string
		platformKey string
		payload     []byte
		checksum    string
	)

	BeforeEach(func() {
		pluginDir = GinkgoT().TempDir()
		platformKey = runtime.GOOS + "-" + runtime.GOARCH
		payload = []byte(fakeExecutable)
		checksum = fmt.Sprintf("%x", sha256.Sum256(payload))
	})

	newDownloader := func(registryURL string) *sdk.Downloader {
		downloader := sdk.NewSecureDownloader(sdk.DownloaderConfig{
			RegistryURL:     registryURL,
			PluginDir:       pluginDir,
			CacheDir:        GinkgoT().TempDir(),
			VerifyChecksums: true,
		})
		downloader.SetSilent(true)
		downloader.SetRetryBackoff(func(context.Context, int, time.Duration) error { return nil })
		return downloader
	}

	writeMetadata := func(w http.ResponseWriter, expectedChecksum string) {
		_ = json.NewEncoder(w).Encode(sdk.PluginMetadata{
			PluginInfo: sdk.PluginInfo{Name: "demo", Version: "1.0.0"},
			Binaries: map[string]sdk.PlatformBinary{
				platformKey: {URL: "https://example.invalid/demo", Checksum: expectedChecksum, Size: int64(len(payload))},
			},
		})
	}

	It("succeeds when a transient 500 is followed by a 200", func() {
		downloads := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/plugins/demo":
				writeMetadata(w, checksum)
			case "/api/v1/plugins/demo/download/" + platformKey:
				downloads++
				if downloads == 1 {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		err := newDownloader(server.URL).DownloadPluginWithContext(context.Background(), "demo")
		Expect(err).NotTo(HaveOccurred())
		Expect(downloads).To(Equal(2))
	})

	It("fails after three attempts when the server keeps returning 500", func() {
		downloads := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/plugins/demo" {
				writeMetadata(w, checksum)
				return
			}
			downloads++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		err := newDownloader(server.URL).DownloadPluginWithContext(context.Background(), "demo")
		Expect(err).To(MatchError(ContainSubstring("download failed: HTTP 500")))
		Expect(downloads).To(Equal(3))
	})

	It("does not retry a 404", func() {
		downloads := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/plugins/demo" {
				writeMetadata(w, checksum)
				return
			}
			downloads++
			http.NotFound(w, r)
		}))
		defer server.Close()

		err := newDownloader(server.URL).DownloadPluginWithContext(context.Background(), "demo")
		Expect(err).To(MatchError(ContainSubstring("download failed: HTTP 404")))
		Expect(downloads).To(Equal(1))
	})

	It("does not retry a checksum mismatch", func() {
		downloads := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/plugins/demo":
				writeMetadata(w, fmt.Sprintf("%x", sha256.Sum256([]byte("different payload"))))
			case "/api/v1/plugins/demo/download/" + platformKey:
				downloads++
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		err := newDownloader(server.URL).DownloadPluginWithContext(context.Background(), "demo")
		Expect(err).To(MatchError(ContainSubstring("checksum verification failed")))
		Expect(downloads).To(Equal(1))
	})

	It("stops retrying when the context is canceled", func() {
		downloads := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/plugins/demo" {
				writeMetadata(w, checksum)
				return
			}
			downloads++
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())
		downloader := newDownloader(server.URL)
		downloader.SetRetryBackoff(func(ctx context.Context, _ int, _ time.Duration) error {
			cancel()
			return ctx.Err()
		})
		err := downloader.DownloadPluginWithContext(ctx, "demo")
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(downloads).To(Equal(1))
	})

	It("honors and caps Retry-After for a 429 response", func() {
		downloads := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/plugins/demo":
				writeMetadata(w, checksum)
			case "/api/v1/plugins/demo/download/" + platformKey:
				downloads++
				if downloads == 1 {
					w.Header().Set("Retry-After", "10")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		var retryAfter time.Duration
		downloader := newDownloader(server.URL)
		downloader.SetRetryBackoff(func(_ context.Context, attempt int, delay time.Duration) error {
			Expect(attempt).To(Equal(1))
			retryAfter = delay
			return nil
		})
		Expect(downloader.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())
		Expect(downloads).To(Equal(2))
		Expect(retryAfter).To(Equal(time.Second))
	})

	It("retries registry metadata after a transient 500", func() {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			if requests == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(sdk.PluginRegistry{Plugins: map[string]sdk.PluginMetadata{
				"demo": {PluginInfo: sdk.PluginInfo{Name: "demo", Version: "1.0.0"}},
			}})
		}))
		defer server.Close()

		plugins, err := newDownloader(server.URL).GetAvailablePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(plugins).To(HaveKey("demo"))
		Expect(requests).To(Equal(2))
	})

	It("reports an unreachable registry once with offline guidance", func() {
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		registryURL := server.URL
		server.Close()

		logger := &retryWarningLogger{}
		downloader := newDownloader(registryURL)
		downloader.SetLogger(logger)
		retries := 0
		downloader.SetRetryBackoff(func(context.Context, int, time.Duration) error {
			retries++
			return nil
		})
		err := downloader.DownloadRequiredPluginsWithContext(context.Background(), []string{"demo", "other"})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("plugin registry is unreachable"))
		Expect(err.Error()).To(ContainSubstring("check your network connection"))
		Expect(err.Error()).To(ContainSubstring("retry"))
		Expect(err.Error()).To(ContainSubstring("--offline"))
		var unavailableErr *sdk.RegistryUnavailableError
		Expect(errors.As(err, &unavailableErr)).To(BeTrue())
		Expect(retries).To(Equal(2))
		Expect(logger.warnings).To(HaveLen(1))
	})
})
