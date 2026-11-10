package cache_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installation caching", func() {
	var manager *cache.CacheManager
	var destination, source string
	var metadata sdk.ArtifactVerification
	var calls, hits int
	var download func(context.Context, string, string) error
	var options cache.InstallCacheOptions
	BeforeEach(func() {
		var err error
		manager, err = cache.NewCacheManager(config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir()})
		Expect(err).NotTo(HaveOccurred())
		destination = filepath.Join(GinkgoT().TempDir(), "install")
		source = "https://example.test/v1/tool"
		metadata = sdk.ArtifactVerification{SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("artifact")))}
		calls, hits = 0, 0
		download = func(ctx context.Context, url, path string) error {
			calls++
			return os.WriteFile(path, []byte("artifact"), 0600)
		}
		options = cache.InstallCacheOptions{Hit: func(string) { hits++ }}
	})
	fetch := func() error {
		return manager.FetchArtifact(context.Background(), source, destination, metadata, options, download)
	}
	It("rejects unsafe download URLs before any network request", func() {
		for _, source := range []string{"http://example.test/tool", "https://localhost/tool", "https://127.0.0.1/tool", "https://192.168.1.2/tool", "https://[::1]/tool"} {
			Expect(cache.DownloadArtifact(context.Background(), source, destination)).To(HaveOccurred())
		}
	})

	It("downloads cold and reuses verified bytes warm and offline", func() {
		Expect(fetch()).To(Succeed())
		options.Offline = true
		Expect(fetch()).To(Succeed())
		Expect(calls).To(Equal(1))
		Expect(hits).To(Equal(1))
		Expect(os.ReadFile(destination)).To(Equal([]byte("artifact")))
	})
	It("evicts corrupt bytes and redownloads online", func() {
		Expect(fetch()).To(Succeed())
		entry, err := manager.GetCacheEntry(cache.ArtifactKey(source, metadata.SHA256))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(entry.Path, []byte("corrupt"), 0600)).To(Succeed())
		Expect(fetch()).To(Succeed())
		Expect(calls).To(Equal(2))
		Expect(hits).To(BeZero())
	})
	It("refuses corrupt offline entries without network", func() {
		Expect(fetch()).To(Succeed())
		entry, err := manager.GetCacheEntry(cache.ArtifactKey(source, metadata.SHA256))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(entry.Path, []byte("corrupt"), 0600)).To(Succeed())
		options.Offline = true
		Expect(fetch()).To(MatchError(ContainSubstring("offline artifact cache miss")))
		Expect(calls).To(Equal(1))
	})
	It("survives missing cached files and clear after acquisition", func() {
		Expect(fetch()).To(Succeed())
		entry, err := manager.GetCacheEntry(cache.ArtifactKey(source, metadata.SHA256))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(entry.Path)).To(Succeed())
		Expect(fetch()).To(Succeed())
		Expect(calls).To(Equal(2))
		Expect(manager.ClearCache()).To(Succeed())
		Expect(os.ReadFile(destination)).To(Equal([]byte("artifact")))
	})
	It("bypasses both reads and writes", func() {
		options.Bypass = true
		Expect(fetch()).To(Succeed())
		Expect(fetch()).To(Succeed())
		Expect(calls).To(Equal(2))
		Expect(hits).To(BeZero())
		entries, err := manager.ListCacheEntries(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
	It("rejects offline misses and conflicting options", func() {
		options.Offline = true
		Expect(fetch()).To(MatchError(ContainSubstring("offline artifact cache miss")))
		options.Bypass = true
		Expect(fetch()).To(MatchError(ContainSubstring("cannot be combined")))
		Expect(calls).To(BeZero())
	})
	It("does not store unpinned or failed integrity downloads", func() {
		metadata.SHA256 = ""
		Expect(fetch()).To(Succeed())
		Expect(fetch()).To(Succeed())
		Expect(calls).To(Equal(2))
		metadata.SHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("wrong")))
		Expect(fetch()).To(MatchError(ContainSubstring("checksum mismatch")))
		entries, err := manager.ListCacheEntries(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
	It("expires stale artifacts", func() {
		Expect(fetch()).To(Succeed())
		_, err := manager.SetCacheEntry(cache.ArtifactKey(source, metadata.SHA256), cache.CacheTypeDownload, destination, nil, time.Nanosecond)
		Expect(err).NotTo(HaveOccurred())
		Expect(fetch()).To(Succeed())
		Expect(calls).To(Equal(2))
	})
	It("does not access downloads when canceled or offline signature verification is required", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(manager.FetchArtifact(ctx, source, destination, metadata, options, download)).To(MatchError(context.Canceled))
		metadata.SignatureURL = "https://example.test/signature"
		options.Offline = true
		Expect(fetch()).To(MatchError(ContainSubstring("offline signature")))
		Expect(calls).To(BeZero())
	})
	It("keys validation by app, artifact version and validator revision", func() {
		validations := 0
		validate := func() error { validations++; return nil }
		for _, key := range [][3]string{{"app", "v1", "validator1"}, {"app", "v1", "validator1"}, {"app", "v2", "validator1"}, {"app", "v2", "validator2"}, {"other", "v2", "validator2"}} {
			Expect(manager.Validate(context.Background(), key[0], key[1], key[2], options, validate)).To(Succeed())
		}
		Expect(validations).To(Equal(4))
		Expect(hits).To(Equal(1))
		options.Bypass = true
		Expect(manager.Validate(context.Background(), "app", "v1", "validator1", options, validate)).To(Succeed())
		Expect(validations).To(Equal(5))
	})
	It("never caches failed validations and expires successful results", func() {
		validate := func() error { return fmt.Errorf("unsafe script") }
		Expect(manager.Validate(context.Background(), "app", "v1", "validator1", options, validate)).To(MatchError("unsafe script"))
		Expect(manager.Validate(context.Background(), "app", "v1", "validator1", options, func() error { return nil })).To(Succeed())
		entries, err := manager.ListCacheEntries(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1))
		marker := filepath.Join(GinkgoT().TempDir(), "marker")
		Expect(os.WriteFile(marker, []byte("valid"), 0600)).To(Succeed())
		_, err = manager.SetCacheEntry(entries[0].Key, cache.CacheTypeMetadata, marker, nil, time.Nanosecond)
		Expect(err).NotTo(HaveOccurred())
		Expect(manager.Validate(context.Background(), "app", "v1", "validator1", options, validate)).To(MatchError("unsafe script"))
	})
})
