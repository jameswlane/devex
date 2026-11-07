package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KDE archive review regressions", func() {
	var home, archive string
	BeforeEach(func() {
		home = GinkgoT().TempDir()
		archive = filepath.Join(GinkgoT().TempDir(), "backup.tar.gz")
		GinkgoT().Setenv("HOME", home)
	})
	put := func(name, content string) {
		target := filepath.Join(home, name)
		Expect(os.MkdirAll(filepath.Dir(target), 0o700)).To(Succeed())
		Expect(os.WriteFile(target, []byte(content), 0o600)).To(Succeed())
	}
	restore := func() error {
		members, err := readBackupMembers(archive)
		if err != nil {
			return err
		}
		_, err = restoreBackupMembers(archive, home, members)
		return err
	}
	It("treats metacharacters in HOME literally and tolerates absent roots", func() {
		home = filepath.Join(home, "literal[home]*?")
		Expect(os.MkdirAll(home, 0o700)).To(Succeed())
		Expect(backupMembers(home)).To(BeEmpty())
		put(".config/kdeglobals", "settings")
		Expect(backupMembers(home)).To(Equal([]string{".config/kdeglobals"}))
	})
	It("preserves invalid-pattern errors even when the directory is absent", func() {
		_, err := expandBackupPattern(home, "missing/[")
		Expect(errors.Is(err, filepath.ErrBadPattern)).To(BeTrue())
	})
	It("propagates directory read failures instead of silently omitting settings", func() {
		put(".config/kdeglobals", "settings")
		config := filepath.Join(home, ".config")
		Expect(os.Chmod(config, 0)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chmod(config, 0o700)).To(Succeed()) })
		// Root can read chmod(0) directories: exercise a real I/O error there too.
		if os.Geteuid() == 0 {
			Expect(os.Chmod(config, 0o700)).To(Succeed())
			Expect(os.RemoveAll(config)).To(Succeed())
			Expect(os.WriteFile(config, nil, 0o600)).To(Succeed())
		}
		_, err := backupMembers(home)
		Expect(err).To(HaveOccurred())
	})
	It("round-trips selected symlinks and shared-inode files as regular data", func() {
		Expect(hostTarErr).NotTo(HaveOccurred())
		Expect(hostGzipErr).NotTo(HaveOccurred())
		put(".config/kdeglobals", "original")
		Expect(os.Link(filepath.Join(home, ".config/kdeglobals"), filepath.Join(home, ".config/kwinrc"))).To(Succeed())
		Expect(os.Symlink("kdeglobals", filepath.Join(home, ".config/plasmarc"))).To(Succeed())
		Expect(NewBackupManager().createConfigBackup(context.Background(), archive)).To(Succeed())
		members, err := readBackupMembers(archive)
		Expect(err).NotTo(HaveOccurred())
		Expect(members).To(HaveLen(3))
		Expect(os.RemoveAll(filepath.Join(home, ".config"))).To(Succeed())
		Expect(restore()).To(Succeed())
		for _, name := range []string{"kdeglobals", "kwinrc", "plasmarc"} {
			target := filepath.Join(home, ".config", name)
			Expect(os.ReadFile(target)).To(BeEquivalentTo("original"))
			info, err := os.Lstat(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().IsRegular()).To(BeTrue())
		}
	})
	It("normalizes legacy members in both passes while preserving both roots", func() {
		writeTarGz(archive, map[string]string{
			"home/alice/.config/kdeglobals":        "config",
			"home/alice/.local/share/plasma/theme": "theme",
		})
		Expect(restore()).To(Succeed())
		Expect(os.ReadFile(filepath.Join(home, ".config/kdeglobals"))).To(BeEquivalentTo("config"))
		Expect(os.ReadFile(filepath.Join(home, ".local/share/plasma/theme"))).To(BeEquivalentTo("theme"))
		Expect(filepath.Join(home, "home")).NotTo(BeAnExistingFile())
	})
	DescribeTable("rejects unsafe legacy members",
		func(header tar.Header) {
			_, err := validateBackupMember(&header)
			Expect(err).To(HaveOccurred())
		},
		Entry("traversal", tar.Header{Name: "home/alice/.config/../escape", Typeflag: tar.TypeReg}),
		Entry("unrelated", tar.Header{Name: "home/alice/.ssh/key", Typeflag: tar.TypeReg}),
		Entry("link", tar.Header{Name: "home/alice/.config/link", Typeflag: tar.TypeSymlink}),
	)
	DescribeTable("confines existing directory symlinks to the selected backup tree",
		func(link, target, member string) {
			Expect(os.MkdirAll(filepath.Join(home, filepath.Dir(link)), 0o700)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(home, "unrelated"), 0o700)).To(Succeed())
			Expect(os.Symlink(target, filepath.Join(home, link))).To(Succeed())
			writeTarGz(archive, map[string]string{member: "bad"})
			Expect(restore()).NotTo(Succeed())
			Expect(filepath.Join(home, "unrelated/settings")).NotTo(BeAnExistingFile())
		},
		Entry("root redirected inside HOME", ".config", "unrelated", ".config/settings"),
		Entry("nested redirected inside HOME", ".config/redirect", "../unrelated", ".config/redirect/settings"),
		Entry("share ancestor redirected", ".local", "unrelated", ".local/share/plasma/settings"),
	)
	It("allows a nested directory symlink that stays inside its backup tree", func() {
		put(".config/actual/settings", "old")
		Expect(os.Symlink("actual", filepath.Join(home, ".config/redirect"))).To(Succeed())
		writeTarGz(archive, map[string]string{".config/redirect/settings": "new"})
		Expect(restore()).To(Succeed())
		Expect(os.ReadFile(filepath.Join(home, ".config/actual/settings"))).To(BeEquivalentTo("new"))
	})
	It("atomically replaces a destination symlink without changing its target", func() {
		put("unrelated", "keep")
		Expect(os.MkdirAll(filepath.Join(home, ".config"), 0o700)).To(Succeed())
		Expect(os.Symlink("../unrelated", filepath.Join(home, ".config/kdeglobals"))).To(Succeed())
		writeTarGz(archive, map[string]string{".config/kdeglobals": "new"})
		Expect(restore()).To(Succeed())
		Expect(os.ReadFile(filepath.Join(home, "unrelated"))).To(BeEquivalentTo("keep"))
		info, err := os.Lstat(filepath.Join(home, ".config/kdeglobals"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().IsRegular()).To(BeTrue())
	})
	It("preserves the previous archive and native status when tar fails after writing", func() {
		put(".config/kdeglobals", "settings")
		Expect(os.WriteFile(archive, []byte("previous"), 0o600)).To(Succeed())
		bin := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(bin, "tar"), []byte("#!/bin/sh\nprintf partial > \"$2\"\nexit 42\n"), 0o700)).To(Succeed())
		GinkgoT().Setenv("PATH", bin)
		Expect(NewBackupManager().createConfigBackup(context.Background(), archive)).To(MatchError("exit status 42"))
		Expect(os.ReadFile(archive)).To(BeEquivalentTo("previous"))
		entries, err := os.ReadDir(filepath.Dir(archive))
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1))
	})

	It("keeps Linux backslashes literal", func() {
		name := `.config/plasma\..\literal`
		writeTarGz(archive, map[string]string{name: "literal"})
		Expect(restore()).To(Succeed())
		Expect(os.ReadFile(filepath.Join(home, name))).To(BeEquivalentTo("literal"))
	})
	It("replaces hardlinked destinations with a fresh inode and exact mode", func() {
		put("unrelated", "keep")
		Expect(os.MkdirAll(filepath.Join(home, ".config"), 0o700)).To(Succeed())
		Expect(os.Link(filepath.Join(home, "unrelated"), filepath.Join(home, ".config/kdeglobals"))).To(Succeed())
		writeTarEntries(archive, []tarEntry{{header: tar.Header{Name: ".config/kdeglobals", Typeflag: tar.TypeReg, Mode: 0o440}, content: "new"}})
		Expect(restore()).To(Succeed())
		Expect(os.ReadFile(filepath.Join(home, "unrelated"))).To(BeEquivalentTo("keep"))
		info, err := os.Stat(filepath.Join(home, ".config/kdeglobals"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o440)))
	})
	It("leaves the destination intact when streaming a replacement fails", func() {
		put(".config/kdeglobals", "keep")
		root, err := os.OpenRoot(home)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = root.Close() }()
		err = writeBackupFile(root, ".config/kdeglobals", io.MultiReader(bytes.NewBufferString("partial"), archiveErrorReader{}), 0o600)
		Expect(err).To(MatchError("test read error"))
		Expect(os.ReadFile(filepath.Join(home, ".config/kdeglobals"))).To(BeEquivalentTo("keep"))
		entries, err := os.ReadDir(filepath.Join(home, ".config"))
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(1))
	})
	DescribeTable("validates the gzip trailer before invoking any restore visitor",
		func(truncate bool) {
			writeTarGz(archive, map[string]string{".config/kdeglobals": "new"})
			members, err := readBackupMembers(archive)
			Expect(err).NotTo(HaveOccurred())
			raw, err := os.ReadFile(archive)
			Expect(err).NotTo(HaveOccurred())
			if truncate {
				raw = raw[:len(raw)-4]
			} else {
				raw[len(raw)-8] ^= 0xff
			}
			Expect(os.WriteFile(archive, raw, 0o600)).To(Succeed())
			_, err = readBackupMembers(archive)
			Expect(err).To(HaveOccurred())
			_, err = restoreBackupMembers(archive, home, members)
			Expect(err).To(HaveOccurred())
			Expect(filepath.Join(home, ".config")).NotTo(BeADirectory())
		}, Entry("bad checksum", false), Entry("truncated trailer", true),
	)
	It("propagates invalid tar data after a valid member", func() {
		writeTarGz(archive, map[string]string{".config/kdeglobals": "new"})
		raw, err := os.ReadFile(archive)
		Expect(err).NotTo(HaveOccurred())
		gz, err := gzip.NewReader(bytes.NewReader(raw))
		Expect(err).NotTo(HaveOccurred())
		data, err := io.ReadAll(gz)
		Expect(err).NotTo(HaveOccurred())
		Expect(gz.Close()).To(Succeed())
		data = data[:len(data)-1024] // Replace tar's zero end blocks with a bad header.
		data = append(data, bytes.Repeat([]byte{'x'}, 512)...)
		var output bytes.Buffer
		writer := gzip.NewWriter(&output)
		_, err = writer.Write(data)
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.Close()).To(Succeed())
		Expect(os.WriteFile(archive, output.Bytes(), 0o600)).To(Succeed())
		Expect(restore()).NotTo(Succeed())
		Expect(filepath.Join(home, ".config")).NotTo(BeADirectory())
	})
	DescribeTable("preserves an existing backup when tar output fails validation",
		func(empty bool) {
			put(".config/kdeglobals", "settings")
			Expect(os.WriteFile(archive, []byte("previous backup"), 0o600)).To(Succeed())
			fixture := filepath.Join(GinkgoT().TempDir(), "output.tar.gz")
			if empty {
				writeTarEntries(fixture, []tarEntry{{header: tar.Header{Name: ".config/", Typeflag: tar.TypeDir}}})
			} else {
				Expect(os.WriteFile(fixture, []byte("broken"), 0o600)).To(Succeed())
			}
			// A successful tar process is insufficient: its output still needs validation.
			bin := GinkgoT().TempDir()
			GinkgoT().Setenv("KDE_TEST_ARCHIVE", fixture)
			Expect(os.WriteFile(filepath.Join(bin, "tar"), []byte("#!/bin/sh\n/bin/cat \"$KDE_TEST_ARCHIVE\" > \"$2\"\n"), 0o700)).To(Succeed())
			GinkgoT().Setenv("PATH", bin)
			Expect(NewBackupManager().createConfigBackup(context.Background(), archive)).To(MatchError(ContainSubstring("invalid created backup")))
			Expect(os.ReadFile(archive)).To(BeEquivalentTo("previous backup"))
			entries, err := os.ReadDir(filepath.Dir(archive))
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
		}, Entry("empty config archive", true), Entry("malformed archive", false),
	)
})

type archiveErrorReader struct{}

func (archiveErrorReader) Read([]byte) (int, error) { return 0, errors.New("test read error") }
