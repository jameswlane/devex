package version_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/version"
)

var _ = Describe("VersionManager", func() {
	var baseDir string

	BeforeEach(func() {
		baseDir = GinkgoT().TempDir()
	})

	Context("on a fresh base directory with no config subdir", func() {
		It("initializes and returns the current version", func() {
			vm := version.NewVersionManager(baseDir)

			info, err := vm.GetCurrentVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(info).NotTo(BeNil())
			Expect(filepath.Join(baseDir, "config", version.VersionFile)).To(BeAnExistingFile())
		})

		It("creates a version and backup via UpdateVersion", func() {
			vm := version.NewVersionManager(baseDir)

			info, err := vm.UpdateVersion("init", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(info).NotTo(BeNil())
			Expect(info.Version).To(Equal("0.0.1"))
			Expect(info.BackupID).NotTo(BeEmpty())
			Expect(filepath.Join(baseDir, "config", version.VersionFile)).To(BeAnExistingFile())

			next, err := vm.UpdateVersion("second", nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(next).NotTo(BeNil())
			Expect(next.Version).To(Equal("0.0.2"))
			Expect(next.BackupID).NotTo(BeEmpty())
			Expect(next.BackupID).NotTo(Equal(info.BackupID))
		})
	})
})
