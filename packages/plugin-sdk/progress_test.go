package sdk_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Required plugin download progress", func() {
	BeforeEach(func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	})

	It("reports byte progress, verification and per-plugin completion in download order", func() {
		payload := []byte(fakeExecutable)
		platform := runtime.GOOS + "-" + runtime.GOARCH
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/download/") {
				w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
				_, _ = w.Write(payload)
				return
			}
			name := strings.TrimPrefix(r.URL.Path, "/api/v1/plugins/")
			checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
			if name == "broken" {
				checksum = strings.Repeat("0", 64)
			}
			_ = json.NewEncoder(w).Encode(sdk.PluginMetadata{
				PluginInfo: sdk.PluginInfo{Name: name, Version: "1.0.0"},
				Binaries: map[string]sdk.PlatformBinary{platform: {
					URL: "https://example.invalid/plugin", Checksum: checksum, Size: int64(len(payload)),
				}},
			})
		}))
		defer server.Close()
		d := sdk.NewSecureDownloader(sdk.DownloaderConfig{
			RegistryURL: server.URL, PluginDir: GinkgoT().TempDir(), CacheDir: GinkgoT().TempDir(), VerifyChecksums: true,
		})
		d.SetSilent(true)
		var events []sdk.PluginProgressEvent
		err := d.DownloadRequiredPluginsWithContextAndProgress(context.Background(), []string{"demo", "broken"}, func(event sdk.PluginProgressEvent) {
			events = append(events, event)
		})
		Expect(err).To(HaveOccurred())
		var stages []sdk.PluginProgressStage
		var previous sdk.PluginProgressStage
		var terminal []string
		for _, event := range events {
			if event.PluginName == "demo" {
				if event.Stage != previous {
					stages = append(stages, event.Stage)
					previous = event.Stage
				}
			} else {
				Expect(terminal).To(ContainElement("demo"))
			}
			if event.Stage == sdk.PluginProgressDone {
				terminal = append(terminal, event.PluginName)
			}
		}
		Expect(stages).To(Equal([]sdk.PluginProgressStage{sdk.PluginProgressDownloading, sdk.PluginProgressVerifying, sdk.PluginProgressDone}))
		Expect(events).To(ContainElement(sdk.PluginProgressEvent{
			PluginName: "demo", Stage: sdk.PluginProgressDownloading, BytesDownloaded: int64(len(payload)), TotalBytes: int64(len(payload)),
		}))
		Expect(events[len(events)-1].PluginName).To(Equal("broken"))
		Expect(events[len(events)-1].Stage).To(Equal(sdk.PluginProgressError))
		Expect(events[len(events)-1].Err).To(MatchError(ContainSubstring("checksum verification failed")))
	})

	It("settles every required plugin when the registry is unavailable", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		d := sdk.NewSecureDownloader(sdk.DownloaderConfig{
			RegistryURL: server.URL, PluginDir: GinkgoT().TempDir(), CacheDir: GinkgoT().TempDir(),
		})
		d.SetSilent(true)
		var events []sdk.PluginProgressEvent
		Expect(d.DownloadRequiredPluginsWithContextAndProgress(context.Background(), []string{"first", "second"}, func(event sdk.PluginProgressEvent) {
			events = append(events, event)
		})).NotTo(Succeed())
		Expect(events).To(HaveLen(4))
		for i, name := range []string{"first", "second"} {
			Expect(events[i*2].PluginName).To(Equal(name))
			Expect(events[i*2].Stage).To(Equal(sdk.PluginProgressDownloading))
			Expect(events[i*2+1].PluginName).To(Equal(name))
			Expect(events[i*2+1].Stage).To(Equal(sdk.PluginProgressError))
			Expect(events[i*2+1].Err).To(HaveOccurred())
		}
	})

	It("accepts a nil callback for existing callers", func() {
		d := sdk.NewSecureDownloader(sdk.DownloaderConfig{PluginDir: GinkgoT().TempDir(), CacheDir: GinkgoT().TempDir()})
		d.SetSilent(true)
		Expect(d.DownloadRequiredPluginsWithContextAndProgress(context.Background(), nil, nil)).To(Succeed())
	})
})
