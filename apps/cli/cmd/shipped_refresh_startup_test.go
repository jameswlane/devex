package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
)

var _ = Describe("Shipped configuration refresh at startup", func() {
	const relPath = "applications/development/build-essential.yaml"
	var refreshHome, configDir, target, manifest string

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		configDir = (&config.CrossPlatformSettings{HomeDir: home}).GetConfigDir()
		Expect(config.WriteDefaultConfig(configDir)).To(Succeed())
		target = filepath.Join(configDir, filepath.FromSlash(relPath))
		manifest = filepath.Join(configDir, config.ShippedManifestFile)
		stale := []byte("name: Build Essential\ndescription: older shipped copy\n")
		sum := sha256.Sum256(stale)
		Expect(os.WriteFile(target, stale, 0600)).To(Succeed())
		Expect(os.WriteFile(manifest, []byte(`{"files":{"`+relPath+`":"`+hex.EncodeToString(sum[:])+`"}}`), 0600)).To(Succeed())
		refreshHome = home
	})

	It("leaves the configuration and manifest unchanged in previews and read commands", func() {
		beforeFile, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		beforeManifest, err := os.ReadFile(manifest)
		Expect(err).NotTo(HaveOccurred())
		refreshShippedConfig(refreshHome, true)
		Expect(os.ReadFile(target)).To(Equal(beforeFile))
		Expect(os.ReadFile(manifest)).To(Equal(beforeManifest))
	})

	It("replaces the unedited stale copy with the shipped file otherwise", func() {
		shipped, err := shipconfig.Files.ReadFile(relPath)
		Expect(err).NotTo(HaveOccurred())
		refreshShippedConfig(refreshHome, false)
		Expect(os.ReadFile(target)).To(Equal(shipped))
	})
})
