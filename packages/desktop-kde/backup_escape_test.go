package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Pre-existing symlinks beneath HOME that point outside it must never let a
// restore write outside HOME.
var _ = Describe("KDE restore containment for symlinks leaving HOME", func() {
	var home, outside, archive string
	BeforeEach(func() {
		home = GinkgoT().TempDir()
		outside = GinkgoT().TempDir()
		archive = filepath.Join(GinkgoT().TempDir(), "backup.tar.gz")
		GinkgoT().Setenv("HOME", home)
		Expect(os.MkdirAll(filepath.Join(home, ".config"), 0o700)).To(Succeed())
	})
	restore := func() error {
		members, err := readBackupMembers(archive)
		if err != nil {
			return err
		}
		_, err = restoreBackupMembers(archive, home, members)
		return err
	}

	It("does not overwrite an external sentinel through a .config file symlink", func() {
		sentinel := filepath.Join(outside, "sentinel")
		Expect(os.WriteFile(sentinel, []byte("external"), 0o600)).To(Succeed())
		Expect(os.Symlink(sentinel, filepath.Join(home, ".config", "kdeglobals"))).To(Succeed())
		writeTarGz(archive, map[string]string{".config/kdeglobals": "restored"})

		Expect(restore()).To(Succeed())

		Expect(os.ReadFile(sentinel)).To(BeEquivalentTo("external"))
		info, err := os.Lstat(filepath.Join(home, ".config", "kdeglobals"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().IsRegular()).To(BeTrue(), "the link itself is replaced inside HOME")
		Expect(os.ReadFile(filepath.Join(home, ".config", "kdeglobals"))).To(BeEquivalentTo("restored"))
	})

	DescribeTable("rejects a .config child directory symlink that leaves HOME",
		func(target func() string) {
			sentinel := filepath.Join(outside, "settings")
			Expect(os.WriteFile(sentinel, []byte("external"), 0o600)).To(Succeed())
			Expect(os.Symlink(target(), filepath.Join(home, ".config", "plasma-org"))).To(Succeed())
			writeTarGz(archive, map[string]string{
				".config/plasma-org/settings": "overwrite",
				".config/plasma-org/new":      "create",
			})

			Expect(restore()).To(MatchError(ContainSubstring("path escapes from parent")))

			Expect(os.ReadFile(sentinel)).To(BeEquivalentTo("external"))
			Expect(filepath.Join(outside, "new")).NotTo(BeAnExistingFile())
			entries, err := os.ReadDir(outside)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1), "no temporary or restored file is created outside HOME")
		},
		Entry("absolute target", func() string { return outside }),
		Entry("relative target", func() string {
			relative, err := filepath.Rel(filepath.Join(home, ".config"), outside)
			Expect(err).NotTo(HaveOccurred())
			return relative
		}),
	)

	It("rejects a .config root symlink that leaves HOME", func() {
		Expect(os.Remove(filepath.Join(home, ".config"))).To(Succeed())
		Expect(os.Symlink(outside, filepath.Join(home, ".config"))).To(Succeed())
		writeTarGz(archive, map[string]string{".config/kdeglobals": "overwrite"})

		Expect(restore()).NotTo(Succeed())
		Expect(filepath.Join(outside, "kdeglobals")).NotTo(BeAnExistingFile())
	})
})
