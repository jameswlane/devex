package sdk

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installed plugin verification", func() {
	It("does not bypass required signatures for a checksum-matching cached plugin", func() {
		payload := []byte("#!/bin/sh\nexit 0\n")
		downloads := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { downloads++; _, _ = w.Write(payload) }))
		defer server.Close()
		downloader := NewSecureDownloader(DownloaderConfig{RegistryURL: server.URL, PluginDir: GinkgoT().TempDir(), CacheDir: GinkgoT().TempDir(), VerifyChecksums: true})
		downloader.SetSilent(true)
		binary := PlatformBinary{URL: server.URL, Checksum: fmt.Sprintf("%x", sha256.Sum256(payload)), Size: int64(len(payload))}
		Expect(downloader.downloadAndVerifyPlugin(context.Background(), "demo", "1.0.0", binary)).To(Succeed())
		Expect(downloads).To(Equal(1))
		Expect(downloader.downloadAndVerifyPlugin(context.Background(), "demo", "1.0.0", binary)).To(Succeed())
		Expect(downloads).To(Equal(1))
		downloader.verifySignatures = true
		Expect(downloader.downloadAndVerifyPlugin(context.Background(), "demo", "1.0.0", binary)).To(HaveOccurred())
		Expect(downloads).To(Equal(2))
	})

	It("matches verified payload bytes and rejects modified installed files", func() {
		payload := []byte("#!/bin/sh\nexit 0\n")
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/download/") {
				_, _ = w.Write(payload)
				return
			}
			_ = json.NewEncoder(w).Encode(PluginMetadata{PluginInfo: PluginInfo{Name: "demo", Version: "1.0.0"}, Binaries: map[string]PlatformBinary{runtime.GOOS + "-" + runtime.GOARCH: {URL: server.URL + "/payload", Checksum: fmt.Sprintf("%x", sha256.Sum256(payload)), Size: int64(len(payload))}}})
		}))
		defer server.Close()
		dir := GinkgoT().TempDir()
		path := filepath.Join(dir, "devex-plugin-demo")
		Expect(os.WriteFile(path, payload, 0700)).To(Succeed())
		downloader := NewSecureDownloader(DownloaderConfig{RegistryURL: server.URL, PluginDir: dir, CacheDir: GinkgoT().TempDir(), VerifyChecksums: true})
		Expect(downloader.VerifyInstalledPlugin(context.Background(), "demo", true, false)).To(Succeed())
		Expect(os.WriteFile(path, []byte("tampered"), 0700)).To(Succeed())
		Expect(downloader.VerifyInstalledPlugin(context.Background(), "demo", true, false)).To(MatchError(ContainSubstring("checksum mismatch")))
		Expect(downloader.VerifyInstalledPlugin(context.Background(), "demo", true, true)).To(HaveOccurred())
	})
})
