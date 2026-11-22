package tui

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Streaming release archive installs", func() {
	DescribeTable("routes catalog release archives past the script allowlist", func(name string) {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", "applications", "development", name+".yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())
		osConfig := app.Linux
		Expect(validateDownloadURL(osConfig.DownloadURL)).To(HaveOccurred(), "the script allowlist rejects GitHub release URLs")
		// Clearing the pin stops the archive installer before any download.
		osConfig.Verification.SHA256 = ""
		osConfig.Destination = GinkgoT().TempDir() // absolute on every OS

		installer := &StreamingInstaller{ctx: context.Background(), config: DefaultInstallerConfig()}
		err = installer.executeCurlPipeInstall(context.Background(), app, &osConfig)
		Expect(err).To(MatchError(ContainSubstring("needs verification.sha256")))
		Expect(err.Error()).NotTo(ContainSubstring("URL validation failed"))
	}, Entry("lazygit", "lazygit"), Entry("neovim", "neovim"))
})
