package agentapply_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/agentapply"
)

var _ = Describe("agentapply helpers", func() {
	It("adds existing user-scope binary directories to PATH once", func() {
		home := GinkgoT().TempDir()
		localBin := filepath.Join(home, ".local", "bin")
		Expect(os.MkdirAll(localBin, 0o750)).To(Succeed())
		GinkgoT().Setenv("PATH", "/usr/bin")
		GinkgoT().Setenv("MISE_DATA_DIR", "")
		GinkgoT().Setenv("XDG_DATA_HOME", "")
		GinkgoT().Setenv("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))

		agentapply.EnsureUserBinDirsOnPath(home)
		agentapply.EnsureUserBinDirsOnPath(home)

		entries := filepath.SplitList(os.Getenv("PATH"))
		Expect(entries).To(Equal([]string{"/usr/bin", localBin}))
		Expect(strings.Contains(os.Getenv("PATH"), "shims")).To(BeFalse())
	})

	It("adds OpenCode's install directory and mise shims under XDG_DATA_HOME", func() {
		home := GinkgoT().TempDir()
		dataHome := filepath.Join(home, "data")
		openCodeBin := filepath.Join(home, ".opencode", "bin")
		shims := filepath.Join(dataHome, "mise", "shims")
		Expect(os.MkdirAll(openCodeBin, 0o750)).To(Succeed())
		Expect(os.MkdirAll(shims, 0o750)).To(Succeed())
		GinkgoT().Setenv("PATH", "/usr/bin")
		GinkgoT().Setenv("MISE_DATA_DIR", "")
		GinkgoT().Setenv("XDG_DATA_HOME", dataHome)

		agentapply.EnsureUserBinDirsOnPath(home)

		Expect(filepath.SplitList(os.Getenv("PATH"))).To(Equal([]string{"/usr/bin", openCodeBin, shims}))
	})

	It("prefers MISE_DATA_DIR for mise shims", func() {
		home := GinkgoT().TempDir()
		miseData := filepath.Join(home, "custom-mise")
		Expect(os.MkdirAll(filepath.Join(miseData, "shims"), 0o750)).To(Succeed())
		GinkgoT().Setenv("PATH", "/usr/bin")
		GinkgoT().Setenv("MISE_DATA_DIR", miseData)
		GinkgoT().Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))

		agentapply.EnsureUserBinDirsOnPath(home)

		Expect(filepath.SplitList(os.Getenv("PATH"))).To(Equal([]string{"/usr/bin", filepath.Join(miseData, "shims")}))
	})

	It("resolves the mise data directory per platform", func() {
		env := func(values map[string]string) func(string) string {
			return func(key string) string { return values[key] }
		}
		Expect(agentapply.MiseDataDir("windows", env(map[string]string{"LOCALAPPDATA": "/appdata"}), "/home/u")).To(Equal(filepath.Join("/appdata", "mise")))
		Expect(agentapply.MiseDataDir("linux", env(map[string]string{"LOCALAPPDATA": "/appdata"}), "/home/u")).To(Equal(filepath.Join("/home/u", ".local", "share", "mise")))
		Expect(agentapply.MiseDataDir("windows", env(map[string]string{"XDG_DATA_HOME": "/xdg", "LOCALAPPDATA": "/appdata"}), "/home/u")).To(Equal(filepath.Join("/xdg", "mise")))
		Expect(agentapply.MiseDataDir("windows", env(map[string]string{"MISE_DATA_DIR": "/m", "XDG_DATA_HOME": "/xdg"}), "/home/u")).To(Equal("/m"))
	})

	It("reports the agents tool-agents skipped", func() {
		out := "configured: codex\nskipped (not installed): claude\n"
		Expect(agentapply.SkippedAgents(out)).To(Equal([]string{"skipped (not installed): claude"}))
		Expect(agentapply.SkippedAgents("configured: codex\n")).To(BeEmpty())
	})
})
