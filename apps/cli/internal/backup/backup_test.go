package backup_test

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
)

var _ = Describe("Restoring a backup", func() {
	var (
		manager   *backup.BackupManager
		configDir string
		id        string
	)

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, key := range []string{
			"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
			"DEVEX_CONFIG_DIR", "DEVEX_DATA_DIR", "DEVEX_STATE_DIR", "DEVEX_CACHE_DIR",
		} {
			GinkgoT().Setenv(key, "")
		}
		base := devexpaths.ForHome(home).Config
		configDir = filepath.Join(base, "config")
		Expect(os.MkdirAll(configDir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("git:\n  name: Me\n"), 0o600)).To(Succeed())
		manager = backup.NewBackupManager(base)
		metadata, err := manager.CreateBackup(backup.BackupOptions{Type: "manual", Compress: true})
		Expect(err).NotTo(HaveOccurred())
		id = metadata.ID
	})

	preRestoreBackups := func() []*backup.BackupMetadata {
		backups, err := manager.ListBackups("pre-restore", 0)
		Expect(err).NotTo(HaveOccurred())
		return backups
	}

	It("snapshots an existing configuration before restoring over it", func() {
		Expect(manager.RestoreBackup(id, "")).To(Succeed())
		Expect(preRestoreBackups()).To(HaveLen(1))
	})

	Context("on a machine with no configuration yet", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(filepath.Dir(configDir))).To(Succeed())
		})

		It("restores without a snapshot, since there is nothing to keep", func() {
			Expect(manager.RestoreBackup(id, "")).To(Succeed())
			Expect(os.ReadFile(filepath.Join(configDir, "config.yaml"))).To(Equal([]byte("git:\n  name: Me\n")))
			Expect(preRestoreBackups()).To(BeEmpty())
		})

		It("removes what a failed restore created", func() {
			dir, err := manager.BackupDir(id)
			Expect(err).NotTo(HaveOccurred())
			// A good entry is extracted before the unsafe one stops the restore.
			file, err := os.Create(filepath.Join(dir, "config.tar.gz"))
			Expect(err).NotTo(HaveOccurred())
			compressed := gzip.NewWriter(file)
			archive := tar.NewWriter(compressed)
			for _, name := range []string{"config.yaml", "../escape"} {
				Expect(archive.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: 1, Typeflag: tar.TypeReg})).To(Succeed())
				_, err := archive.Write([]byte("x"))
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(archive.Close()).To(Succeed())
			Expect(compressed.Close()).To(Succeed())
			Expect(file.Close()).To(Succeed())

			Expect(manager.RestoreBackup(id, "")).To(MatchError(ContainSubstring("failed to extract backup")))
			Expect(configDir).NotTo(BeAnExistingFile())
			Expect(preRestoreBackups()).To(BeEmpty())
		})
	})
})
