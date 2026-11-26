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

		agentapply.EnsureUserBinDirsOnPath(home)
		agentapply.EnsureUserBinDirsOnPath(home)

		entries := filepath.SplitList(os.Getenv("PATH"))
		Expect(entries).To(Equal([]string{"/usr/bin", localBin}))
		Expect(strings.Contains(os.Getenv("PATH"), "shims")).To(BeFalse())
	})

	It("reports the agents tool-agents skipped", func() {
		out := "configured: codex\nskipped (not installed): claude\n"
		Expect(agentapply.SkippedAgents(out)).To(Equal([]string{"skipped (not installed): claude"}))
		Expect(agentapply.SkippedAgents("configured: codex\n")).To(BeEmpty())
	})
})
