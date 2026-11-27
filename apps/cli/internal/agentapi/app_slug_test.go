package agentapi_test

import (
	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Application lookup by kebab-case name", func() {
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		settings = config.CrossPlatformSettings{
			HomeDir: GinkgoT().TempDir(),
			ApplicationOverrides: []types.CrossPlatformApp{
				{Name: "Claude Code", Linux: types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://claude.ai/install.sh"}},
				{Name: "Codex CLI", Linux: types.OSConfig{InstallMethod: "mise", InstallCommand: "codex@latest"}},
				{Name: "Node.js", Linux: types.OSConfig{InstallMethod: "mise", InstallCommand: "node@24"}},
			},
		}
	})

	It("turns names into catalog file names", func() {
		Expect(types.AppSlug("Claude Code")).To(Equal("claude-code"))
		Expect(types.AppSlug(" GitHub Copilot CLI ")).To(Equal("github-copilot-cli"))
		Expect(types.AppSlug("Node.js")).To(Equal("node-js"))
		Expect(types.AppSlug("--")).To(BeEmpty())
	})

	It("resolves kebab-case names alongside exact names and install commands", func() {
		apps, err := agentapi.ResolveAppsByName(settings, []string{"claude-code", "Codex-CLI", "node-js", "codex@latest"})
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(apps))
		for _, app := range apps {
			names = append(names, app.Name)
		}
		Expect(names).To(Equal([]string{"Claude Code", "Codex CLI", "Node.js"}))
	})

	It("still reports names that match nothing", func() {
		_, err := agentapi.ResolveAppsByName(settings, []string{"claude"})
		Expect(err).To(MatchError(ContainSubstring("applications not found: claude")))
	})

	It("maps a kebab-case name to its catalog name", func() {
		Expect(settings.CanonicalAppName("claude-code")).To(Equal("Claude Code"))
		Expect(settings.CanonicalAppName("unknown-app")).To(Equal("unknown-app"))
	})
})
