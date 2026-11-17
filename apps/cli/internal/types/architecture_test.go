package types_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("APT source architecture placeholder", func() {
	source := "deb [arch=%ARCHITECTURE% signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main"
	app := func() types.CrossPlatformApp {
		return types.CrossPlatformApp{AllPlatforms: types.OSConfig{
			InstallMethod: "apt", InstallCommand: "gh",
			AptSources: []types.AptSource{{SourceRepo: source, SourceName: "/etc/apt/sources.list.d/github-cli.list"}},
		}}
	}

	DescribeTable("writes the dpkg architecture into the source line",
		func(goarch, dpkg string) {
			defer types.PinCatalogArchForTesting(goarch)()
			a := app()
			Expect(a.GetOSConfig().AptSources[0].SourceRepo).To(Equal(
				"deb [arch=" + dpkg + " signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main"))
		},
		Entry("amd64", "amd64", "amd64"),
		Entry("arm64", "arm64", "arm64"),
		Entry("arm", "arm", "armhf"),
		Entry("386", "386", "i386"),
	)

	It("does not modify the stored configuration", func() {
		a := app()
		a.GetOSConfig()
		Expect(a.AllPlatforms.AptSources[0].SourceRepo).To(Equal(source))
	})
})

var _ = Describe("Shipped catalog architecture values", func() {
	load := func(name string) types.CrossPlatformApp {
		data, err := os.ReadFile(filepath.Join("../../config/applications/development", name+".yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())
		return app
	}

	DescribeTable("downloads the fastfetch release asset for the host architecture",
		func(arch, asset string) {
			app := load("fastfetch")
			config := app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: "debian", Version: "12", Architecture: arch})
			Expect(config.InstallMethod).To(Equal("deb"))
			Expect(config.DownloadURL).To(Equal("https://github.com/fastfetch-cli/fastfetch/releases/latest/download/" + asset))
		},
		Entry("amd64", "amd64", "fastfetch-linux-amd64.deb"),
		Entry("x86_64", "x86_64", "fastfetch-linux-amd64.deb"),
		Entry("arm64", "arm64", "fastfetch-linux-aarch64.deb"),
	)

	It("uses the placeholder only in APT source lines, where it is replaced", func() {
		files, err := filepath.Glob("../../config/applications/*/*.yaml")
		Expect(err).NotTo(HaveOccurred())
		Expect(files).NotTo(BeEmpty())
		for _, file := range files {
			data, err := os.ReadFile(file)
			Expect(err).NotTo(HaveOccurred())
			for _, line := range strings.Split(string(data), "\n") {
				if strings.Contains(line, types.ArchitecturePlaceholder) {
					Expect(strings.TrimSpace(line)).To(HavePrefix("source_repo:"), file)
				}
			}
		}
	})

	It("points the GitHub CLI source at the host architecture", func() {
		defer types.PinCatalogArchForTesting("amd64")()
		app := load("github-cli")
		app.Linux.PlatformRequirements = nil
		Expect(app.GetOSConfig().AptSources[0].SourceRepo).To(HavePrefix("deb [arch=amd64 signed-by="))
	})
})
