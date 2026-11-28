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

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
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
	root    string
	bin     string
	curlLog string
	arch    string
	tarball string
}

func newHookSandbox() *hookSandbox {
	if runtime.GOOS != "linux" {
		Skip("catalog hooks target Linux")
	}
	for _, tool := range []string{"flock", "sha256sum", "tar", "mktemp", "readlink"} {
		if _, err := exec.LookPath(tool); err != nil {
			Skip(tool + " is not available")
		}
	}
	root := GinkgoT().TempDir()
	s := &hookSandbox{root: root, bin: filepath.Join(root, "stub-bin"), curlLog: filepath.Join(root, "curl.log"), arch: "x86_64"}
	for _, dir := range []string{s.bin, "opt", "usr/local/bin", "usr/share/applications", "run/lock", "etc"} {
		Expect(os.MkdirAll(filepath.Join(root, strings.TrimPrefix(dir, root)), 0o755)).To(Succeed())
	}
	stubs := map[string]string{
		// curl -fsSL --proto '=https' -o OUT URL
		"curl":  "#!/bin/sh\nout=\"\"; url=\"\"\nwhile [ $# -gt 0 ]; do case \"$1\" in -o) out=\"$2\"; shift ;; https://*) url=\"$1\" ;; esac; shift; done\necho \"$url\" >> \"$CURL_LOG\"\n[ -n \"$FIXTURE_TARBALL\" ] || exit 22\ncp \"$FIXTURE_TARBALL\" \"$out\"\n",
		"uname": "#!/bin/sh\necho \"$STUB_ARCH\"\n",
		"chown": "#!/bin/sh\nexit 0\n",
		"id":    "#!/bin/sh\necho 1000\n",
		"sudo":  "#!/bin/sh\nexec \"$@\"\n",
	}
	for name, body := range stubs {
		Expect(os.WriteFile(filepath.Join(s.bin, name), []byte(body), 0o755)).To(Succeed()) //nolint:gosec // test stubs must be executable
	}
	return s
}

func (s *hookSandbox) path(p string) string { return filepath.Join(s.root, p) }

// rewrite moves the system paths a hook touches under the sandbox root.
func (s *hookSandbox) rewrite(script string) string {
	for _, prefix := range []string{"/opt/", "/usr/local/bin/", "/usr/share/applications/", "/run/lock/", "/etc/os-release"} {
		script = strings.ReplaceAll(script, prefix, s.root+prefix)
	}
	return script
}

func (s *hookSandbox) run(script string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "bash", "-c", s.rewrite(script))
	cmd.Env = append(os.Environ(),
		"PATH="+s.bin+":"+os.Getenv("PATH"),
		"CURL_LOG="+s.curlLog,
		"STUB_ARCH="+s.arch,
		"FIXTURE_TARBALL="+s.tarball,
	)
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
