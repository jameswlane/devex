package selfupdate_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/selfupdate"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Self update", func() {
	var server *httptest.Server
	var client *selfupdate.Client
	var releases []selfupdate.Release
	var assets map[string][]byte
	var requests atomic.Int32
	var status int
	var token atomic.Value
	var updater *selfupdate.Updater
	var binary string

	BeforeEach(func() {
		assets = make(map[string][]byte)
		requests.Store(0)
		status = http.StatusOK
		token.Store("")
		releases = []selfupdate.Release{{Tag: "packages/tool-shell/v99.0.0"}, {Tag: "v0.3.2"}, {Tag: "v0.3.3"}, {Tag: "v0.4.0-rc.1", Prerelease: true}, {Tag: "v9.0.0", Draft: true}}
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer GinkgoRecover()
			requests.Add(1)
			if status != http.StatusOK {
				w.WriteHeader(status)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/repos/") {
				token.Store(r.Header.Get("Authorization"))
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "/tags/") {
					tag := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
					for _, release := range releases {
						if release.Tag == tag {
							_ = json.NewEncoder(w).Encode(release)
							return
						}
					}
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.URL.Query().Get("page") != "1" {
					_ = json.NewEncoder(w).Encode([]selfupdate.Release{})
					return
				}
				_ = json.NewEncoder(w).Encode(releases)
				return
			}
			Expect(r.Header.Get("Authorization")).To(BeEmpty())
			if data, ok := assets[r.URL.Path]; ok {
				_, _ = w.Write(data)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		client = &selfupdate.Client{HTTP: server.Client(), BaseURL: server.URL, Token: "test-token"}
		binary = filepath.Join(GinkgoT().TempDir(), "devex")
		Expect(os.WriteFile(binary, []byte("old binary"), 0755)).To(Succeed())
		updater = &selfupdate.Updater{Client: client, Executable: func() (string, error) { return binary, nil }, OS: "linux", Arch: "amd64"}
	})
	AfterEach(func() { server.Close() })

	makeRelease := func(goos string) selfupdate.Release {
		name := "devex_0.3.3_" + goos + "_amd64.tar.gz"
		var data bytes.Buffer
		gz := gzip.NewWriter(&data)
		tw := tar.NewWriter(gz)
		entry := "bin/devex"
		if goos == "windows" {
			entry += ".exe"
		}
		Expect(tw.WriteHeader(&tar.Header{Name: entry, Mode: 0755, Size: int64(len("new binary"))})).To(Succeed())
		_, err := tw.Write([]byte("new binary"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tw.Close()).To(Succeed())
		Expect(gz.Close()).To(Succeed())
		assets["/"+name] = data.Bytes()
		assets["/devex_checksums.txt"] = fmt.Appendf(nil, "%x  %s\n", sha256.Sum256(data.Bytes()), name)
		assets["/devex_checksums.txt.sigstore.json"] = []byte("signature bundle")
		return selfupdate.Release{Tag: "v0.3.3", URL: "https://example.test/releases/v0.3.3", Assets: []selfupdate.Asset{
			{Name: name, URL: server.URL + "/" + name},
			{Name: "devex_checksums.txt", URL: server.URL + "/devex_checksums.txt"},
			{Name: "devex_checksums.txt.sigstore.json", URL: server.URL + "/devex_checksums.txt.sigstore.json"},
		}}
	}

	It("selects the newest stable CLI release and sends optional API authentication", func() {
		release, err := client.Find(context.Background(), "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.Tag).To(Equal("v0.3.3"))
		Expect(token.Load()).To(Equal("Bearer test-token"))
	})
	It("includes prereleases only when requested, including pinned releases", func() {
		release, err := client.Find(context.Background(), "", true)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.Tag).To(Equal("v0.4.0-rc.1"))
		_, err = client.Find(context.Background(), "v0.4.0-rc.1", false)
		Expect(err).To(MatchError(ContainSubstring("--pre")))
		release, err = client.Find(context.Background(), "v0.3.2", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.Tag).To(Equal("v0.3.2"))
		_, err = client.Find(context.Background(), "packages/tool-shell/v99.0.0", true)
		Expect(err).To(HaveOccurred())
	})
	It("paginates through plugin releases", func() {
		releases = make([]selfupdate.Release, 100)
		for i := range releases {
			releases[i].Tag = "packages/plugin/v1.0.0"
		}
		releases[99].Tag = "v0.3.3"
		release, err := client.Find(context.Background(), "", false)
		Expect(err).NotTo(HaveOccurred())
		Expect(release.Tag).To(Equal("v0.3.3"))
		Expect(requests.Load()).To(Equal(int32(2)))
	})
	DescribeTable("compares release versions", func(latest, current string, newer bool) { Expect(selfupdate.Newer(latest, current)).To(Equal(newer)) },
		Entry("newer", "v0.3.3", "0.3.2", true), Entry("same", "v0.3.3", "0.3.3", false), Entry("older", "v0.3.2", "0.3.3", false), Entry("dev", "v0.3.3", "dev", false))
	It("verifies and installs the archive and supports offline rollback", func() {
		release := makeRelease("linux")
		called := false
		updater.VerifySignature = func(_ context.Context, checksums, bundle []byte, dir string) error {
			called = true
			Expect(checksums).To(Equal(assets["/devex_checksums.txt"]))
			Expect(bundle).To(Equal([]byte("signature bundle")))
			Expect(dir).To(BeADirectory())
			return nil
		}
		Expect(updater.Install(context.Background(), release)).To(Succeed())
		Expect(called).To(BeTrue())
		Expect(os.ReadFile(binary)).To(Equal([]byte("new binary")))
		Expect(os.ReadFile(filepath.Join(filepath.Dir(binary), "devex.previous"))).To(Equal([]byte("old binary")))
		previousRequests := requests.Load()
		Expect(updater.Rollback(context.Background())).To(Succeed())
		Expect(os.ReadFile(binary)).To(Equal([]byte("old binary")))
		Expect(requests.Load()).To(Equal(previousRequests))
	})
	It("supports the Windows running-executable rename and rollback sequence", func() {
		updater.OS = "windows"
		release := makeRelease("windows")
		Expect(updater.Install(context.Background(), release)).To(Succeed())
		Expect(updater.Rollback(context.Background())).To(Succeed())
		Expect(os.ReadFile(binary)).To(Equal([]byte("old binary")))
	})
	It("leaves the binary unchanged on checksum mismatch", func() {
		release := makeRelease("linux")
		assets["/devex_checksums.txt"] = []byte("bad  devex_0.3.3_linux_amd64.tar.gz\n")
		Expect(updater.Install(context.Background(), release)).To(MatchError(ContainSubstring("checksum mismatch")))
		Expect(os.ReadFile(binary)).To(Equal([]byte("old binary")))
		Expect(filepath.Join(filepath.Dir(binary), "devex.previous")).NotTo(BeAnExistingFile())
	})
	It("fails closed on signature mismatch or missing bundle", func() {
		release := makeRelease("linux")
		updater.VerifySignature = func(context.Context, []byte, []byte, string) error { return fmt.Errorf("invalid signature") }
		Expect(updater.Install(context.Background(), release)).To(MatchError(ContainSubstring("invalid signature")))
		delete(assets, "/devex_checksums.txt.sigstore.json")
		Expect(updater.Install(context.Background(), release)).To(HaveOccurred())
		Expect(os.ReadFile(binary)).To(Equal([]byte("old binary")))
	})
	It("refuses unwritable and package-managed installs without downloading", func() {
		dir := filepath.Dir(binary)
		Expect(os.Chmod(dir, 0555)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chmod(dir, 0755)).To(Succeed()) })
		Expect(updater.Install(context.Background(), makeRelease("linux"))).To(MatchError(ContainSubstring("not writable")))
		Expect(requests.Load()).To(BeZero())
		Expect(os.Chmod(dir, 0755)).To(Succeed())
		updater.Managed = func(context.Context, string) bool { return true }
		Expect(updater.Install(context.Background(), makeRelease("linux"))).To(MatchError(ContainSubstring("package manager")))
		Expect(requests.Load()).To(BeZero())
	})
	It("handles rate limiting, offline failures and cancellation with actionable errors", func() {
		status = http.StatusForbidden
		_, err := client.Find(context.Background(), "", false)
		Expect(err).To(MatchError(ContainSubstring("GITHUB_TOKEN")))
		server.Close()
		_, err = client.Find(context.Background(), "", false)
		Expect(err).To(MatchError(ContainSubstring("check your connection")))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = client.Find(ctx, "", false)
		Expect(err).To(HaveOccurred())
	})
	It("caches timestamps and release results for 24 hours", func() {
		dir := GinkgoT().TempDir()
		now := time.Now()
		message := selfupdate.CheckNotification(context.Background(), client, "0.3.2", dir, now)
		Expect(message).To(ContainSubstring("DevEx 0.3.3 is available (you have 0.3.2)"))
		Expect(requests.Load()).To(Equal(int32(1)))
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.3.2", dir, now.Add(23*time.Hour))).To(Equal(message))
		Expect(requests.Load()).To(Equal(int32(1)))
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.3.3", dir, now.Add(23*time.Hour))).To(BeEmpty())
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.4.0", dir, now.Add(23*time.Hour))).To(BeEmpty())
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.3.2", dir, now.Add(24*time.Hour))).To(Equal(message))
		Expect(requests.Load()).To(Equal(int32(2)))
	})
	It("caches failed checks without printing errors or retrying each run", func() {
		status = http.StatusTooManyRequests
		dir := GinkgoT().TempDir()
		now := time.Now()
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.3.2", dir, now)).To(BeEmpty())
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.3.2", dir, now.Add(time.Hour))).To(BeEmpty())
		Expect(requests.Load()).To(Equal(int32(1)))
	})
	It("keeps an interrupted request retryable without consuming the 24 hour window", func() {
		dir := GinkgoT().TempDir()
		path := filepath.Join(dir, "update-check.json")
		now := time.Now()
		started := make(chan struct{})
		slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done()
		}))
		DeferCleanup(slowServer.Close)
		slowClient := &selfupdate.Client{HTTP: slowServer.Client(), BaseURL: slowServer.URL}
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		finished := make(chan string, 1)
		go func() { finished <- selfupdate.CheckNotification(ctx, slowClient, "0.3.2", dir, now) }()
		Eventually(started).Should(BeClosed())
		Expect(path).NotTo(BeAnExistingFile())
		Expect(path + ".lock").To(BeAnExistingFile())
		// A concurrent invocation cannot start a duplicate lookup.
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.3.2", dir, now)).To(BeEmpty())
		Expect(requests.Load()).To(BeZero())
		cancel()
		Eventually(finished).Should(Receive(BeEmpty()))
		Expect(path).NotTo(BeAnExistingFile())
		Expect(path + ".lock").NotTo(BeAnExistingFile())
		Expect(selfupdate.CheckNotification(context.Background(), client, "0.3.2", dir, now.Add(time.Second))).To(ContainSubstring("0.3.3 is available"))
		Expect(requests.Load()).To(Equal(int32(1)))
	})
})

var _ = Describe("Notification suppression", func() {
	DescribeTable("does no checking for suppressed policies", func(change func(*selfupdate.NotificationPolicy)) {
		p := selfupdate.NotificationPolicy{Version: "0.3.2", TTY: true}
		Expect(p.Enabled()).To(BeTrue())
		change(&p)
		Expect(p.Enabled()).To(BeFalse())
	},
		Entry("JSON", func(p *selfupdate.NotificationPolicy) { p.Format = "json" }),
		Entry("YAML", func(p *selfupdate.NotificationPolicy) { p.Format = "yaml" }),
		Entry("non-TTY", func(p *selfupdate.NotificationPolicy) { p.TTY = false }),
		Entry("CI", func(p *selfupdate.NotificationPolicy) { p.CI = true }),
		Entry("non-interactive", func(p *selfupdate.NotificationPolicy) { p.NonInteractive = true }),
		Entry("config or environment opt-out", func(p *selfupdate.NotificationPolicy) { p.Disabled = true }),
		Entry("offline", func(p *selfupdate.NotificationPolicy) { p.Offline = true }),
		Entry("dev", func(p *selfupdate.NotificationPolicy) { p.Version = "dev" }),
		Entry("snapshot", func(p *selfupdate.NotificationPolicy) { p.Version = "0.3.3-next" }))
})
