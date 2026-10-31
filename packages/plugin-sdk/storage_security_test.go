package sdk

import (
	"context"
	"crypto/sha256"
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

var _ = Describe("Elevated plugin storage symlink safety", func() {
	var d *Downloader
	var home, pluginDir, outside string
	var payload []byte
	var binary PlatformBinary
	BeforeEach(func() {
		home = GinkgoT().TempDir()
		pluginDir = PathsForHome(home).Plugins()
		outside = filepath.Join(GinkgoT().TempDir(), "outside")
		Expect(os.WriteFile(outside, []byte("untouched"), 0600)).To(Succeed())
		payload = []byte("#!/bin/sh\necho plugin\n")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) }))
		DeferCleanup(server.Close)
		d = NewDownloader(server.URL, pluginDir)
		d.SetSilent(true)
		Expect(d.SetInstallOwner(UserIdentity{Username: "intended", UID: fmt.Sprint(os.Getuid()), GID: fmt.Sprint(os.Getgid()), HomeDir: home})).To(Succeed())
		d.chown = func(string, int, int) error { return nil }
		binary = PlatformBinary{URL: server.URL, Size: int64(len(payload)), Checksum: fmt.Sprintf("%x", sha256.Sum256(payload))}
	})
	AfterEach(func() {
		data, err := os.ReadFile(outside)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("untouched"))
		info, err := os.Stat(outside)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
	})
	It("refuses an install-record symlink without truncating its outside target", func() {
		Expect(d.PrepareInstallDirectory()).To(Succeed())
		Expect(os.Symlink(outside, installRecordPath(pluginDir, "example"))).To(Succeed())
		Expect(d.downloadAndVerifyPlugin(context.Background(), "example", "1", binary)).To(MatchError(ContainSubstring("symlinked plugin storage path")))
		// The standalone writer uses the same atomic publication policy.
		Expect(writeInstallRecord(pluginDir, InstallRecord{Plugin: "example"})).To(HaveOccurred())
	})
	It("skips a symlinked registry cache without writing through it", func() {
		root, err := d.openInstallStorage(true)
		Expect(err).NotTo(HaveOccurred())
		Expect(root.Close()).To(Succeed())
		Expect(os.Symlink(outside, filepath.Join(d.cacheDir, "registry.json"))).To(Succeed())
		d.cacheRegistry(&PluginRegistry{})
	})
	It("refuses a symlinked installed binary before attempting ownership changes", func() {
		Expect(d.PrepareInstallDirectory()).To(Succeed())
		Expect(os.Symlink(outside, filepath.Join(pluginDir, pluginExecutableName("example", runtime.GOOS)))).To(Succeed())
		Expect(d.downloadAndVerifyPlugin(context.Background(), "example", "1", binary)).To(MatchError(ContainSubstring("symlinked plugin storage path")))
	})
	It("refuses a linked destination in the local plugin installer", func() {
		Expect(d.PrepareInstallDirectory()).To(Succeed())
		name := pluginExecutableName("example", runtime.GOOS)
		Expect(os.Symlink(outside, filepath.Join(pluginDir, name))).To(Succeed())
		source := filepath.Join(home, "source")
		Expect(os.WriteFile(source, payload, 0600)).To(Succeed())
		Expect(NewExecutableManager(pluginDir).InstallPlugin(source, "example")).To(HaveOccurred())
	})
	It("keeps extraction and chmod on the exclusive inode when the temporary name is replaced", func() {
		root, err := d.openInstallStorage(false)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(root.Close)
		file, err := createStorageTemp(root, ".devex-test-bin-")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(file.Close)
		name := filepath.Base(file.Name())
		Expect(root.Remove(name)).To(Succeed())
		Expect(root.Symlink(outside, name)).To(Succeed())
		source, err := os.CreateTemp(GinkgoT().TempDir(), "payload-")
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(source.Close)
		_, err = source.Write(payload)
		Expect(err).NotTo(HaveOccurred())
		_, err = extractPluginExecutableTo(source, file, "example", "linux")
		Expect(err).NotTo(HaveOccurred())
		Expect(file.Chmod(0755)).To(Succeed())
		Expect(d.ownInstallFile(file, filepath.Join(pluginDir, name))).To(Succeed())
		Expect(publishStorageFile(root, file, "example")).To(MatchError(ContainSubstring("temporary file was replaced")))
	})
	It("anchors atomic publication when the user replaces the parent path", func() {
		root, err := d.openInstallStorage(false)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(root.Close)
		moved := pluginDir + "-moved"
		Expect(os.Rename(pluginDir, moved)).To(Succeed())
		Expect(os.Symlink(filepath.Dir(outside), pluginDir)).To(Succeed())
		Expect(atomicStorageWrite(root, "outside", []byte("safe"), 0600, nil)).To(Succeed())
		data, err := os.ReadFile(filepath.Join(moved, "outside"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("safe"))
	})
	DescribeTable("refuses symlinked storage directories",
		func(component string) {
			target := GinkgoT().TempDir()
			path := filepath.Join(home, ".local")
			if component == "plugins" {
				path = pluginDir
			}
			if component == "plugin-cache" {
				path = d.cacheDir
			}
			Expect(os.MkdirAll(filepath.Dir(path), 0755)).To(Succeed())
			Expect(os.Symlink(target, path)).To(Succeed())
			var err error
			if component == "plugin-cache" {
				var root *os.Root
				root, err = d.openInstallStorage(true)
				if root != nil {
					_ = root.Close()
				}
			} else {
				err = d.PrepareInstallDirectory()
			}
			Expect(err).To(MatchError(ContainSubstring("symlinked plugin storage directory")))
			entries, err := os.ReadDir(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		}, Entry("data ancestor", ".local"), Entry("plugins", "plugins"), Entry("cache", "plugin-cache"),
	)
	It("refuses an existing directory owned by a different account", func() {
		Expect(os.MkdirAll(filepath.Join(home, ".local"), 0755)).To(Succeed())
		d.installOwner.UID = fmt.Sprint(os.Getuid() + 1)
		Expect(d.PrepareInstallDirectory()).To(MatchError(ContainSubstring("not owned by intended UID")))
	})
	It("does not truncate a symlink passed to the public extraction helper", func() {
		source := filepath.Join(home, "payload")
		Expect(os.WriteFile(source, payload, 0600)).To(Succeed())
		dest := filepath.Join(home, "destination")
		Expect(os.Symlink(outside, dest)).To(Succeed())
		_, err := ExtractPluginExecutable(source, dest, "example", "linux")
		Expect(err).To(HaveOccurred())
		Expect(strings.Contains(err.Error(), "symlinked")).To(BeTrue())
	})
})
