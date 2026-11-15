package tui

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Organization policy interactive installation", func() {
	var home, policyPath string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		teamDir := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		policyPath = filepath.Join(teamDir, "policy.yaml")
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
	})

	writePolicy := func(contents string) {
		Expect(os.WriteFile(policyPath, []byte(contents), 0o600)).To(Succeed())
	}

	It("refuses a denied app before execution and runs an allowed app", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get")
		settings := config.CrossPlatformSettings{HomeDir: home}
		installer := NewStreamingInstaller(nil, &MockRepository{}, context.Background(), settings)

		writePolicy("version: '1'\napplications:\n  deny: [editor]\n")
		Expect(installer.InstallApp(context.Background(), aptApp("editor", "editor"), settings)).To(MatchError(ContainSubstring("applications.deny")))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())

		writePolicy("version: '1'\napplications:\n  allow: [editor]\n")
		Expect(installer.InstallApp(context.Background(), aptApp("editor", "editor"), settings)).To(Succeed())
		Expect(recorder.Calls("apt-get")).NotTo(BeEmpty())
	})

	It("keeps a denied app out of the shared package transaction", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dpkg-query")
		viper.Set("apt-lists-max-age", "0s")
		DeferCleanup(func() { viper.Set("apt-lists-max-age", nil) })
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		writePolicy("version: '1'\napplications:\n  deny: [fzf]\n")

		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), aptApp("fzf", "fzf"), aptApp("jq", "jq")}
		batch, err := PreparePackageBatch(context.Background(), apps, &MockRepository{}, config.CrossPlatformSettings{HomeDir: home})
		Expect(err).NotTo(HaveOccurred())
		Expect(batch.BatchInstalled).NotTo(HaveKey("fzf"))
		for _, call := range recorder.Calls("apt-get") {
			Expect(call.Args).NotTo(ContainElement("fzf"))
		}
		Expect(recorder.Calls("apt-get")).To(ContainElement(HaveField("Args", Equal([]string{"install", "-y", "bat", "jq"}))))
	})
})
