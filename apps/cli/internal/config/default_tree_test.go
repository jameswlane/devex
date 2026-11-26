package config_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/config"
)

type manifestDoc struct {
	Files map[string]string `json:"files"`
}

func testSHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readManifest(configDir string) manifestDoc {
	data, err := os.ReadFile(filepath.Join(configDir, config.ShippedManifestFile))
	Expect(err).NotTo(HaveOccurred())
	var doc manifestDoc
	Expect(json.Unmarshal(data, &doc)).To(Succeed())
	return doc
}

func writeManifest(configDir string, doc manifestDoc) {
	data, err := json.MarshalIndent(doc, "", "  ")
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(filepath.Join(configDir, config.ShippedManifestFile), data, 0600)).To(Succeed())
}

var _ = Describe("Shipped default configuration tree and manifest", func() {
	const relPath = "applications/development/build-essential.yaml"

	var (
		homeDir      string
		configDir    string
		targetPath   string
		shippedBytes []byte
		shippedHash  string
	)

	BeforeEach(func() {
		homeDir = GinkgoT().TempDir()
		settings := config.CrossPlatformSettings{HomeDir: homeDir}
		configDir = settings.GetConfigDir()
		targetPath = filepath.Join(configDir, filepath.FromSlash(relPath))

		var err error
		shippedBytes, err = shipconfig.Files.ReadFile(relPath)
		Expect(err).NotTo(HaveOccurred())
		shippedHash = testSHA256Hex(shippedBytes)
	})

	Describe("WriteDefaultConfig", func() {
		It("writes the shipped tree and records the manifest with mode 0600", func() {
			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			written, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(written).To(Equal(shippedBytes))

			manifestPath := filepath.Join(configDir, config.ShippedManifestFile)
			info, err := os.Stat(manifestPath)
			Expect(err).NotTo(HaveOccurred())
			if runtime.GOOS != "windows" {
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
			}

			manifest := readManifest(configDir)
			Expect(manifest.Files).To(HaveKeyWithValue(relPath, shippedHash))

			// Verify loaders and strict validation ignore the manifest file.
			files, err := config.ConfigurationFiles(configDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(files).NotTo(ContainElement(config.ShippedManifestFile))

			result, err := config.ValidateConfig(context.Background(), config.CrossPlatformSettings{HomeDir: homeDir}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Valid).To(BeTrue(), "diagnostics: %+v", result.Diagnostics)
			Expect(result.Warnings).To(Equal(0))
		})

		It("refreshes an unedited stale shipped file whose hash matches the manifest", func() {
			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			staleContent := []byte("name: Build Essential\ndescription: Old shipped version\ncategory: Development Tools\n")
			staleHash := testSHA256Hex(staleContent)
			Expect(os.WriteFile(targetPath, staleContent, 0600)).To(Succeed())

			manifest := readManifest(configDir)
			manifest.Files[relPath] = staleHash
			manifest.Files["applications/retired/old-tool.yaml"] = "deadbeef"
			writeManifest(configDir, manifest)

			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			after, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(shippedBytes))

			updatedManifest := readManifest(configDir)
			Expect(updatedManifest.Files).To(HaveKeyWithValue(relPath, shippedHash))
			Expect(updatedManifest.Files).To(HaveKeyWithValue("applications/retired/old-tool.yaml", "deadbeef"))
		})

		It("preserves a user-edited file and leaves its manifest entry unchanged", func() {
			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			editedContent := []byte("name: Build Essential\ndescription: User customized\ncategory: Development Tools\n")
			Expect(os.WriteFile(targetPath, editedContent, 0600)).To(Succeed())

			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			after, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(editedContent))

			manifest := readManifest(configDir)
			Expect(manifest.Files).To(HaveKeyWithValue(relPath, shippedHash))
		})

		It("preserves a legacy file when no manifest entry exists", func() {
			Expect(os.MkdirAll(filepath.Dir(targetPath), 0750)).To(Succeed())
			legacyContent := []byte("name: Build Essential\ndescription: Legacy install copy\ncategory: Development Tools\n")
			Expect(os.WriteFile(targetPath, legacyContent, 0600)).To(Succeed())

			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			after, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(legacyContent))

			manifest := readManifest(configDir)
			Expect(manifest.Files).NotTo(HaveKey(relPath))
		})

		It("leaves a legacy file identical to the shipped copy untracked", func() {
			Expect(os.MkdirAll(filepath.Dir(targetPath), 0750)).To(Succeed())
			Expect(os.WriteFile(targetPath, shippedBytes, 0600)).To(Succeed())

			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			manifest := readManifest(configDir)
			Expect(manifest.Files).NotTo(HaveKey(relPath))
		})
	})

	Describe("OverwriteDefaultConfig", func() {
		It("replaces a user-edited shipped file and updates its manifest hash", func() {
			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			editedContent := []byte("name: Build Essential\ndescription: User customized\ncategory: Development Tools\n")
			Expect(os.WriteFile(targetPath, editedContent, 0600)).To(Succeed())

			manifest := readManifest(configDir)
			manifest.Files[relPath] = "0000000000000000000000000000000000000000000000000000000000000000"
			writeManifest(configDir, manifest)

			Expect(config.OverwriteDefaultConfig(configDir)).To(Succeed())

			after, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(shippedBytes))

			updatedManifest := readManifest(configDir)
			Expect(updatedManifest.Files).To(HaveKeyWithValue(relPath, shippedHash))
		})
	})

	Describe("RefreshShippedConfig", func() {
		It("is a no-op when the config directory or manifest does not exist", func() {
			updated, err := config.RefreshShippedConfig(configDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).To(BeNil())

			Expect(os.MkdirAll(filepath.Dir(targetPath), 0750)).To(Succeed())
			legacyContent := []byte("name: Build Essential\ndescription: Legacy\n")
			Expect(os.WriteFile(targetPath, legacyContent, 0600)).To(Succeed())

			updated, err = config.RefreshShippedConfig(configDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).To(BeNil())

			after, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(legacyContent))
		})

		It("refreshes unedited stale files, skips edited files, and never recreates deleted files", func() {
			const (
				editedRelPath  = "applications/development/git.yaml"
				deletedRelPath = "applications/development/curl.yaml"
			)

			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())

			staleContent := []byte("name: Build Essential\ndescription: Previous shipped version\ncategory: Development Tools\n")
			staleHash := testSHA256Hex(staleContent)
			Expect(os.WriteFile(targetPath, staleContent, 0600)).To(Succeed())

			editedTarget := filepath.Join(configDir, filepath.FromSlash(editedRelPath))
			editedContent := []byte("name: git\ndescription: User edited git\ncategory: development\n")
			Expect(os.WriteFile(editedTarget, editedContent, 0600)).To(Succeed())

			deletedTarget := filepath.Join(configDir, filepath.FromSlash(deletedRelPath))
			Expect(os.Remove(deletedTarget)).To(Succeed())

			manifest := readManifest(configDir)
			manifest.Files[relPath] = staleHash
			manifest.Files[editedRelPath] = "1111111111111111111111111111111111111111111111111111111111111111"
			manifest.Files[deletedRelPath] = "2222222222222222222222222222222222222222222222222222222222222222"
			writeManifest(configDir, manifest)

			updated, err := config.RefreshShippedConfig(configDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).To(ConsistOf(relPath))

			refreshed, err := os.ReadFile(targetPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(refreshed).To(Equal(shippedBytes))

			stillEdited, err := os.ReadFile(editedTarget)
			Expect(err).NotTo(HaveOccurred())
			Expect(stillEdited).To(Equal(editedContent))

			Expect(deletedTarget).NotTo(BeAnExistingFile())

			updatedManifest := readManifest(configDir)
			Expect(updatedManifest.Files).To(HaveKeyWithValue(relPath, shippedHash))
			Expect(updatedManifest.Files).To(HaveKeyWithValue(editedRelPath, "1111111111111111111111111111111111111111111111111111111111111111"))
			Expect(updatedManifest.Files).To(HaveKeyWithValue(deletedRelPath, "2222222222222222222222222222222222222222222222222222222222222222"))

			// Second run is a no-op once refreshed.
			again, err := config.RefreshShippedConfig(configDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(again).To(BeEmpty())
		})

		It("writes shipped files that are new since the manifest was recorded", func() {
			// An older CLI never shipped this file, so its manifest has no entry.
			const newRelPath = "applications/development/bubblewrap.yaml"
			Expect(config.WriteDefaultConfig(configDir)).To(Succeed())
			newTarget := filepath.Join(configDir, filepath.FromSlash(newRelPath))
			Expect(os.Remove(newTarget)).To(Succeed())
			manifest := readManifest(configDir)
			delete(manifest.Files, newRelPath)
			writeManifest(configDir, manifest)

			updated, err := config.RefreshShippedConfig(configDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).To(ConsistOf(newRelPath))
			shipped, err := shipconfig.Files.ReadFile(newRelPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.ReadFile(newTarget)).To(Equal(shipped))
			Expect(readManifest(configDir).Files).To(HaveKeyWithValue(newRelPath, testSHA256Hex(shipped)))
		})
	})
})
