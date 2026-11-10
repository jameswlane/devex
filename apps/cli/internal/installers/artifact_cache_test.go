package installers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Plugin cache preparation", func() {
	BeforeEach(func() { viper.Set("offline", false); viper.Set("no-cache", false) })
	AfterEach(func() { viper.Set("offline", false); viper.Set("no-cache", false) })
	It("provides an independent verified offline copy and cleans it after dispatch", func() {
		home, err := os.UserHomeDir()
		Expect(err).NotTo(HaveOccurred())
		manager, err := cache.NewCacheManager(config.CrossPlatformSettings{HomeDir: home})
		Expect(err).NotTo(HaveOccurred())
		source := filepath.Join(GinkgoT().TempDir(), "source")
		Expect(os.WriteFile(source, []byte("fixture"), 0600)).To(Succeed())
		metadata := sdk.ArtifactVerification{SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("fixture")))}
		const url = "https://example.test/tool"
		_, err = manager.SetCacheEntry(cache.ArtifactKey(url, metadata.SHA256), cache.CacheTypeDownload, source, nil, 0)
		Expect(err).NotTo(HaveOccurred())
		viper.Set("offline", true)
		prepared, cleanup, err := PrepareArtifact(context.Background(), url, metadata)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(cleanup)
		Expect(manager.ClearCache()).To(Succeed())
		result, err := sdk.VerifyArtifact(context.Background(), prepared.CachedPath, metadata)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.ChecksumVerified).To(BeTrue())
		cleanup()
		_, err = os.Stat(prepared.CachedPath)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
	It("bypasses preparation online and rejects misses offline", func() {
		viper.Set("no-cache", true)
		metadata := sdk.ArtifactVerification{SHA256: "test-digest"}
		prepared, cleanup, err := PrepareArtifact(context.Background(), "https://example.test/tool", metadata)
		Expect(err).NotTo(HaveOccurred())
		defer cleanup()
		Expect(prepared.CachedPath).To(BeEmpty())
		viper.Set("offline", true)
		_, _, err = PrepareArtifact(context.Background(), "https://example.test/tool", metadata)
		Expect(err).To(MatchError(ContainSubstring("cannot be combined")))
		viper.Set("no-cache", false)
		_, _, err = PrepareArtifact(context.Background(), "https://example.test/tool", metadata)
		Expect(err).To(MatchError(ContainSubstring("offline artifact cache miss")))
	})
})
