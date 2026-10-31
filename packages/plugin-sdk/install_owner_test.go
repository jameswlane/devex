package sdk

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Elevated plugin download ownership", func() {
	It("assigns the intended owner to directories, downloaded executables and records", func() {
		home := GinkgoT().TempDir()
		dir := filepath.Join(home, ".devex", "plugins")
		payload := []byte("\x7fELF" + strings.Repeat("\x00", 64))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) }))
		DeferCleanup(server.Close)
		d := NewDownloader(server.URL, dir)
		d.SetSilent(true)
		Expect(d.SetInstallOwner(UserIdentity{Username: "alice", UID: fmt.Sprint(os.Getuid()), GID: fmt.Sprint(os.Getgid()), HomeDir: home})).To(Succeed())
		var owned []string
		d.chown = func(path string, uid, gid int) error {
			Expect(uid).To(Equal(os.Getuid()))
			Expect(gid).To(Equal(os.Getgid()))
			Expect(path).NotTo(HavePrefix("/root/"))
			owned = append(owned, path)
			return nil
		}
		Expect(d.PrepareInstallDirectory()).To(Succeed())
		Expect(d.downloadAndVerifyPlugin(context.Background(), "example", "1", PlatformBinary{URL: server.URL, Checksum: fmt.Sprintf("%x", sha256.Sum256(payload)), Size: int64(len(payload))})).To(Succeed())
		Expect(owned).To(ContainElements(filepath.Join(home, ".devex"), dir, installRecordPath(dir, "example")))
		binaryOwned := false
		for _, path := range owned {
			if strings.Contains(filepath.Base(path), "-bin-") {
				binaryOwned = true
			}
		}
		Expect(binaryOwned).To(BeTrue())
		Expect(filepath.Join(dir, "devex-plugin-example")).To(BeAnExistingFile())
		d.cacheRegistry(&PluginRegistry{})
		Expect(owned).To(ContainElements(filepath.Join(home, ".devex", "plugin-cache"), filepath.Join(home, ".devex", "plugin-cache", "registry.json")))
	})
	It("fails a download when ownership cannot be assigned", func() {
		home := GinkgoT().TempDir()
		d := NewDownloader("http://unused.invalid", filepath.Join(home, ".devex", "plugins"))
		Expect(d.SetInstallOwner(UserIdentity{UID: "1000", GID: "1000", HomeDir: home})).To(Succeed())
		d.chown = func(string, int, int) error { return os.ErrPermission }
		Expect(d.PrepareInstallDirectory()).To(MatchError(ContainSubstring("set plugin owner")))
	})
})
