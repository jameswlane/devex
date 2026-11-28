package commands_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

// loadShippedApp decodes one shipped catalog file.
func loadShippedApp(path string) types.CrossPlatformApp {
	data, err := fs.ReadFile(shipconfig.Files, path)
	Expect(err).NotTo(HaveOccurred())
	var app types.CrossPlatformApp
	Expect(yaml.Unmarshal(data, &app)).To(Succeed())
	return app
}

// hookSandbox runs a catalog hook with its absolute system paths moved under
// a temp root and with curl, uname, chown, id and sudo replaced by stubs.
type hookSandbox struct {
	root     string
	bin      string
	curlLog  string
	chownLog string
	arch     string
	tarball  string
}

func newHookSandbox() *hookSandbox {
	if runtime.GOOS != "linux" {
		Skip("catalog hooks target Linux")
	}
	// The hooks write system paths. Unprivileged, any path the rewrite
	// missed fails with EACCES instead of touching the host.
	if os.Geteuid() == 0 {
		Skip("catalog hook specs refuse to run as root")
	}
	for _, tool := range []string{"flock", "sha256sum", "tar", "mktemp", "readlink"} {
		if _, err := exec.LookPath(tool); err != nil {
			Skip(tool + " is not available")
		}
	}
	root := GinkgoT().TempDir()
	realTar, err := exec.LookPath("tar")
	Expect(err).NotTo(HaveOccurred())
	s := &hookSandbox{root: root, bin: filepath.Join(root, "stub-bin"), curlLog: filepath.Join(root, "curl.log"), chownLog: filepath.Join(root, "chown.log"), arch: "x86_64"}
	for _, dir := range []string{s.bin, "opt", "usr/local/bin", "usr/share/applications", "var/lib", "etc", "home"} {
		Expect(os.MkdirAll(filepath.Join(root, strings.TrimPrefix(dir, root)), 0o755)).To(Succeed())
	}
	stubs := map[string]string{
		// curl -fsSL --proto '=https' -o OUT URL
		"curl":  "#!/bin/sh\nout=\"\"; url=\"\"\nwhile [ $# -gt 0 ]; do case \"$1\" in -o) out=\"$2\"; shift ;; https://*) url=\"$1\" ;; esac; shift; done\necho \"$url\" >> \"$CURL_LOG\"\n[ -n \"$FIXTURE_TARBALL\" ] || exit 22\ncp \"$FIXTURE_TARBALL\" \"$out\"\n",
		"uname": "#!/bin/sh\necho \"$STUB_ARCH\"\n",
		// Unprivileged chown cannot hand files to root; record the call.
		"chown": "#!/bin/sh\necho \"$*\" >> \"$CHOWN_LOG\"\n",
		// Keep the archive's modes regardless of umask, as root's tar does,
		// so the specs see what chmod -R go-w has to fix.
		"tar":  "#!/bin/sh\nexec " + realTar + " \"$@\" --preserve-permissions\n",
		"id":   "#!/bin/sh\necho 1000\n",
		"sudo": "#!/bin/sh\nexec \"$@\"\n",
	}
	for name, body := range stubs {
		Expect(os.WriteFile(filepath.Join(s.bin, name), []byte(body), 0o755)).To(Succeed()) //nolint:gosec // test stubs must be executable
	}
	return s
}

func (s *hookSandbox) path(p string) string { return filepath.Join(s.root, p) }

// rewrite moves the system paths a hook touches under the sandbox root.
func (s *hookSandbox) rewrite(script string) string {
	for _, prefix := range []string{"/opt/", "/usr/local/bin/", "/usr/share/applications/", "/var/lib/devex", "/etc/os-release"} {
		script = strings.ReplaceAll(script, prefix, s.root+prefix)
	}
	// Every absolute path left must be inside the sandbox root.
	for _, m := range absolutePath.FindAllStringSubmatch(script, -1) {
		if !strings.HasPrefix(m[1], s.root) && m[1] != "/dev/null" {
			Fail("hook path " + m[1] + " is not rewritten under the sandbox root")
		}
	}
	return script
}

var absolutePath = regexp.MustCompile(`(?:^|[\s"'=(])(/[A-Za-z0-9._%-][A-Za-z0-9._%/-]*)`)

func (s *hookSandbox) run(script string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "bash", "-c", s.rewrite(script))
	// A scrubbed environment: no BASH_ENV, and HOME inside the sandbox.
	cmd.Env = []string{
		"PATH=" + s.bin + ":/usr/bin:/bin",
		"HOME=" + s.path("home"),
		"LANG=C",
		"CHOWN_LOG=" + s.chownLog,
		"CURL_LOG=" + s.curlLog,
		"STUB_ARCH=" + s.arch,
		"FIXTURE_TARBALL=" + s.tarball,
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (s *hookSandbox) curlCalls() string {
	data, err := os.ReadFile(s.curlLog)
	if os.IsNotExist(err) {
		return ""
	}
	Expect(err).NotTo(HaveOccurred())
	return string(data)
}

// writeAntigravityTarball builds a tarball shaped like the vendor archive,
// with one group/world-writable file, and returns its SHA-256.
func (s *hookSandbox) writeAntigravityTarball() string {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	entries := []struct {
		name string
		mode int64
		body string
	}{
		{"Antigravity IDE/", 0o755 | int64(fs.ModeDir), ""},
		{"Antigravity IDE/bin/", 0o755 | int64(fs.ModeDir), ""},
		{"Antigravity IDE/bin/antigravity-ide", 0o755, "#!/bin/sh\necho 2.8.0\n"},
		{"Antigravity IDE/antigravity-ide", 0o777, "elf"},
		{"Antigravity IDE/chrome-sandbox", 0o755, "elf"},
	}
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: e.mode & 0o7777, Size: int64(len(e.body)), Uname: "nobody", Gname: "nogroup", Uid: 65534, Gid: 65534}
		if strings.HasSuffix(e.name, "/") {
			hdr.Typeflag = tar.TypeDir
		} else {
			hdr.Typeflag = tar.TypeReg
		}
		Expect(tw.WriteHeader(hdr)).To(Succeed())
		_, err := tw.Write([]byte(e.body))
		Expect(err).NotTo(HaveOccurred())
	}
	Expect(tw.Close()).To(Succeed())
	Expect(gz.Close()).To(Succeed())
	s.tarball = s.path("fixture.tar.gz")
	Expect(os.WriteFile(s.tarball, buf.Bytes(), 0o600)).To(Succeed())
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:])
}

var pinnedSHA256 = regexp.MustCompile(`sum=[0-9a-f]{64}`)

var _ = Describe("AI Code Editors catalog hooks", func() {
	Describe("Antigravity", func() {
		var (
			app       types.CrossPlatformApp
			sandbox   *hookSandbox
			install   string
			uninstall string
		)

		BeforeEach(func() {
			app = loadShippedApp("applications/ai-agents/antigravity.yaml")
			Expect(app.Linux.PostInstall).To(HaveLen(1))
			Expect(app.Linux.PostInstall[0].Scope).To(Equal("system"))
			sandbox = newHookSandbox()
			install = app.Linux.PostInstall[0].Shell
			uninstall = app.Linux.UninstallCommand
		})

		withFixtureSum := func(script, sum string) string {
			return pinnedSHA256.ReplaceAllString(script, "sum="+sum)
		}

		DescribeTable("installs the pinned build for the machine's architecture",
			func(uname, wantArch string) {
				sandbox.arch = uname
				sum := sandbox.writeAntigravityTarball()
				out, err := sandbox.run(withFixtureSum(install, sum))
				Expect(err).NotTo(HaveOccurred(), out)

				Expect(sandbox.curlCalls()).To(Equal("https://edgedl.me.gvt1.com/edgedl/release2/j0qc3/antigravity/stable/2.8.0-6733682917113856/" + wantArch + "/Antigravity%20IDE.tar.gz\n"))
				dir := sandbox.path("opt/antigravity-ide")
				Expect(filepath.Join(dir, ".devex-managed")).To(BeAnExistingFile())
				sandboxInfo, err := os.Stat(filepath.Join(dir, "chrome-sandbox"))
				Expect(err).NotTo(HaveOccurred())
				Expect(sandboxInfo.Mode() & (fs.ModeSetuid | fs.ModePerm)).To(Equal(fs.ModeSetuid | 0o755))
				launcher, err := os.Stat(filepath.Join(dir, "antigravity-ide"))
				Expect(err).NotTo(HaveOccurred())
				Expect(launcher.Mode().Perm()&0o022).To(BeZero(), "group/world write must be stripped")
				chowns, err := os.ReadFile(sandbox.chownLog)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(chowns)).To(MatchRegexp(`(?m)^-R root:root ` + regexp.QuoteMeta(sandbox.path("opt/.antigravity-ide.")) + `\S+$`))

				target, err := os.Readlink(sandbox.path("usr/local/bin/antigravity-ide"))
				Expect(err).NotTo(HaveOccurred())
				Expect(target).To(Equal(dir + "/bin/antigravity-ide"))
				entry, err := os.ReadFile(sandbox.path("usr/share/applications/antigravity-ide.desktop"))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(entry)).To(ContainSubstring("X-DevEx-Managed=true\n"))

				leftovers, err := filepath.Glob(sandbox.path("opt/.antigravity-ide.*"))
				Expect(err).NotTo(HaveOccurred())
				Expect(leftovers).To(BeEmpty(), "the staging directory must not be left behind")

				// Reinstalling over its own install succeeds.
				out, err = sandbox.run(withFixtureSum(install, sum))
				Expect(err).NotTo(HaveOccurred(), out)
			},
			Entry("x86_64", "x86_64", "linux-x64"),
			Entry("aarch64", "aarch64", "linux-arm"),
		)

		It("refuses an unknown architecture before downloading", func() {
			sandbox.arch = "riscv64"
			out, err := sandbox.run(install)
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring("Antigravity has no Linux build for riscv64."))
			Expect(sandbox.curlCalls()).To(BeEmpty())
		})

		It("fails without publishing when the download does not match the pin", func() {
			sandbox.writeAntigravityTarball()
			out, err := sandbox.run(install)
			Expect(err).To(HaveOccurred(), out)
			Expect(sandbox.path("opt/antigravity-ide")).NotTo(BeAnExistingFile())
			Expect(sandbox.path("usr/local/bin/antigravity-ide")).NotTo(BeAnExistingFile())
		})

		It("refuses an /opt/antigravity-ide it did not create", func() {
			Expect(os.MkdirAll(sandbox.path("opt/antigravity-ide"), 0o755)).To(Succeed())
			Expect(os.WriteFile(sandbox.path("opt/antigravity-ide/mine"), nil, 0o600)).To(Succeed())
			out, err := sandbox.run(install)
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring("exists and was not installed by DevEx"))
			Expect(sandbox.path("opt/antigravity-ide/mine")).To(BeAnExistingFile())
			Expect(sandbox.curlCalls()).To(BeEmpty())
		})

		It("refuses a launcher link that points elsewhere", func() {
			Expect(os.Symlink("/usr/bin/true", sandbox.path("usr/local/bin/antigravity-ide"))).To(Succeed())
			out, err := sandbox.run(install)
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring("belongs to another install"))
			Expect(sandbox.curlCalls()).To(BeEmpty())
		})

		It("refuses a desktop entry that is a symlink, even a dangling one", func() {
			target := sandbox.path("elsewhere/target.desktop")
			Expect(os.Symlink(target, sandbox.path("usr/share/applications/antigravity-ide.desktop"))).To(Succeed())
			out, err := sandbox.run(install)
			Expect(err).To(HaveOccurred())
			Expect(out).To(ContainSubstring("was not created by DevEx"))
			Expect(target).NotTo(BeAnExistingFile())
			Expect(sandbox.curlCalls()).To(BeEmpty())
		})

		It("makes uninstall wait for an install holding the lock", func() {
			Expect(os.MkdirAll(sandbox.path("var/lib/devex"), 0o700)).To(Succeed())
			lock := sandbox.path("var/lib/devex/antigravity-ide.lock")
			holder := exec.CommandContext(context.Background(), "flock", lock, "sleep", "2")
			Expect(holder.Start()).To(Succeed())
			DeferCleanup(func() { _ = holder.Wait() })
			Eventually(lock).Should(BeAnExistingFile())
			start := time.Now()
			out, err := sandbox.run(uninstall)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(time.Since(start)).To(BeNumerically(">=", time.Second), "uninstall must wait for the lock")
		})

		It("uninstalls only what it created and keeps a hand-made install's launcher", func() {
			sum := sandbox.writeAntigravityTarball()
			out, err := sandbox.run(withFixtureSum(install, sum))
			Expect(err).NotTo(HaveOccurred(), out)
			out, err = sandbox.run(uninstall)
			Expect(err).NotTo(HaveOccurred(), out)
			for _, p := range []string{"opt/antigravity-ide", "usr/share/applications/antigravity-ide.desktop"} {
				Expect(sandbox.path(p)).NotTo(BeAnExistingFile())
			}
			_, err = os.Lstat(sandbox.path("usr/local/bin/antigravity-ide"))
			Expect(os.IsNotExist(err)).To(BeTrue(), "the launcher link must be removed")

			// A hand-made install uses the same layout without the marker.
			Expect(os.MkdirAll(sandbox.path("opt/antigravity-ide/bin"), 0o755)).To(Succeed())
			link := sandbox.path("usr/local/bin/antigravity-ide")
			Expect(os.Symlink(sandbox.path("opt/antigravity-ide/bin/antigravity-ide"), link)).To(Succeed())
			out, err = sandbox.run(uninstall)
			Expect(err).NotTo(HaveOccurred(), out)
			Expect(sandbox.path("opt/antigravity-ide/bin")).To(BeADirectory())
			_, err = os.Lstat(link)
			Expect(err).NotTo(HaveOccurred(), "the hand-made launcher must be kept")
		})
	})

	Describe("Kiro", func() {
		var app types.CrossPlatformApp

		BeforeEach(func() {
			app = loadShippedApp("applications/ai-agents/kiro.yaml")
			Expect(app.Linux.PreInstall).To(HaveLen(1))
			Expect(app.Linux.Alternatives).To(HaveLen(1))
			Expect(app.Linux.Alternatives[0].PreInstall).To(HaveLen(1))
			Expect(app.Linux.Alternatives[0].PreInstall[0].Shell).To(Equal(app.Linux.PreInstall[0].Shell), "both architectures must guard the same releases")
		})

		DescribeTable("picks the package for the architecture, whatever the release",
			func(plat platform.DetectionResult, wantURL string) {
				DeferCleanup(types.PinCatalogOSForTesting("linux"))
				DeferCleanup(types.PinCatalogArchForTesting(plat.Architecture))
				Expect(app.LinuxConfigForPlatform(plat).DownloadURL).To(Equal(wantURL))
			},
			Entry("Debian testing on arm64 (no VERSION_ID)", platform.DetectionResult{OS: "linux", Distribution: "debian", Architecture: "arm64"},
				"https://prod.download.desktop.kiro.dev/releases/stable/linux-arm64/signed/1.4.154/deb/kiro-ide-1.4.154-stable-linux-arm64.deb"),
			Entry("Ubuntu 24.04 on arm64", platform.DetectionResult{OS: "linux", Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "arm64"},
				"https://prod.download.desktop.kiro.dev/releases/stable/linux-arm64/signed/1.4.154/deb/kiro-ide-1.4.154-stable-linux-arm64.deb"),
			Entry("Debian 13 on amd64", platform.DetectionResult{OS: "linux", Distribution: "debian", Version: "13", Architecture: "amd64"},
				"https://prod.download.desktop.kiro.dev/releases/stable/linux-x64/signed/1.4.154/deb/kiro-ide-1.4.154-stable-linux-x64.deb"),
		)

		DescribeTable("guards the releases Kiro supports",
			func(osRelease string, wantOK bool, wantMessage string) {
				sandbox := newHookSandbox()
				Expect(os.WriteFile(sandbox.path("etc/os-release"), []byte(osRelease), 0o600)).To(Succeed())
				out, err := sandbox.run(app.Linux.PreInstall[0].Shell)
				if wantOK {
					Expect(err).NotTo(HaveOccurred(), out)
					return
				}
				Expect(err).To(HaveOccurred())
				Expect(out).To(ContainSubstring(wantMessage))
			},
			Entry("Ubuntu 22.04", "ID=ubuntu\nVERSION_ID=\"22.04\"\n", false, "Kiro needs Ubuntu 24.04 or newer (this is 22.04)."),
			Entry("Ubuntu 24.04", "ID=ubuntu\nVERSION_ID=\"24.04\"\n", true, ""),
			Entry("Debian 12", "ID=debian\nVERSION_ID=\"12\"\n", false, "Kiro needs Debian 13 or newer (this is 12)."),
			Entry("Debian 13", "ID=debian\nVERSION_ID=\"13\"\n", true, ""),
			Entry("Debian testing without VERSION_ID", "ID=debian\n", true, ""),
			Entry("Linux Mint 21", "ID=linuxmint\nVERSION_ID=\"21.3\"\nID_LIKE=\"ubuntu debian\"\n", false, "Kiro needs Linux Mint 22 or newer (this is 21.3)."),
			Entry("Linux Mint 22", "ID=linuxmint\nVERSION_ID=\"22\"\n", true, ""),
		)
	})
})
