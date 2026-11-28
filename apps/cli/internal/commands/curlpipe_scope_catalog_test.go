package commands_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("curlpipe install scope in the catalog", func() {
	It("runs every script that installs into $HOME as the invoking user", func() {
		paths, err := filepath.Glob(filepath.Join("..", "..", "config", "applications", "*", "*.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(paths).NotTo(BeEmpty())

		checked := 0
		for _, path := range paths {
			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			var app types.CrossPlatformApp
			Expect(yaml.Unmarshal(data, &app)).To(Succeed(), path)

			check := func(label string, cfg types.OSConfig) {
				if cfg.InstallMethod != "curlpipe" || cfg.IsHookDrivenCurlpipe() {
					return
				}
				if cfg.DownloadURL != "" && installers.IsReleaseArchive(cfg.DownloadURL) {
					return
				}
				if !strings.Contains(cfg.UninstallCommand, "$HOME") {
					return
				}
				checked++
				Expect(cfg.InstallScope).To(Equal("user"), "%s (%s) installs into $HOME, so its curlpipe script must use install_scope: user", app.Name, label)
			}
			for label, cfg := range map[string]types.OSConfig{"linux": app.Linux, "macos": app.MacOS, "windows": app.Windows, "all_platforms": app.AllPlatforms} {
				check(label, cfg)
				for _, alt := range cfg.Alternatives {
					check(label+"-alt", alt)
				}
			}
		}
		Expect(checked).To(BeNumerically(">", 0))
	})

	It("installs LazyDocker on Linux as the invoking user", func() {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", "applications", "development", "lazydocker.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())
		Expect(app.Linux.InstallMethod).To(Equal("curlpipe"))
		Expect(app.Linux.InstallScope).To(Equal("user"))
		Expect(app.Linux.DownloadURL).To(HavePrefix("https://"))
		Expect(app.Linux.InstallCommand).To(BeEmpty())
	})
})
