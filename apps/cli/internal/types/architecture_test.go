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

	DescribeTable("downloads the LazyGit and Neovim release assets for the host architecture",
		func(name, distribution, version, arch, url, sha256 string) {
			app := load(name)
			config := app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: distribution, Version: version, Architecture: arch})
			Expect(config.InstallMethod).To(Equal("curlpipe"))
			Expect(config.DownloadURL).To(Equal(url))
			Expect(config.Verification.SHA256).To(Equal(sha256))
			Expect(config.IsArchitectureSupported(arch)).To(BeTrue())
		},
		Entry("LazyGit on amd64 Ubuntu", "lazygit", "ubuntu", "24.04", "amd64",
			"https://github.com/jesseduffield/lazygit/releases/download/v0.66.3/lazygit_0.66.3_linux_x86_64.tar.gz",
			"0a23f429a95f0d09db298b3c291536d818de066aa5a85c255806f0490ff6656b"),
		Entry("LazyGit on arm64 Ubuntu", "lazygit", "ubuntu", "24.04", "arm64",
			"https://github.com/jesseduffield/lazygit/releases/download/v0.66.3/lazygit_0.66.3_linux_arm64.tar.gz",
			"ac0b48237f3d0283813e7154ec063237e519532feb0470c0881a1793ee78fd42"),
		Entry("LazyGit on aarch64 Debian", "lazygit", "debian", "12", "aarch64",
			"https://github.com/jesseduffield/lazygit/releases/download/v0.66.3/lazygit_0.66.3_linux_arm64.tar.gz",
			"ac0b48237f3d0283813e7154ec063237e519532feb0470c0881a1793ee78fd42"),
		Entry("Neovim on amd64 Ubuntu", "neovim", "ubuntu", "24.04", "amd64",
			"https://github.com/neovim/neovim/releases/download/v0.12.6/nvim-linux-x86_64.tar.gz",
			"474430d53e6264f6d6dd18db42d6dc9df3a1b56ca9e88a325bbf860e1a811d87"),
		Entry("Neovim on arm64 Ubuntu", "neovim", "ubuntu", "24.04", "arm64",
			"https://github.com/neovim/neovim/releases/download/v0.12.6/nvim-linux-arm64.tar.gz",
			"8f1f64a0bdb97247034038c3823c6cbad5bdf9ecd5751b85494b71c3ee04c815"),
		Entry("Neovim on arm64 Debian", "neovim", "debian", "12", "arm64",
			"https://github.com/neovim/neovim/releases/download/v0.12.6/nvim-linux-arm64.tar.gz",
			"8f1f64a0bdb97247034038c3823c6cbad5bdf9ecd5751b85494b71c3ee04c815"),
		Entry("Neovim on amd64 Fedora", "neovim", "fedora", "44", "amd64",
			"https://github.com/neovim/neovim/releases/download/v0.12.6/nvim-linux-x86_64.tar.gz",
			"474430d53e6264f6d6dd18db42d6dc9df3a1b56ca9e88a325bbf860e1a811d87"),
	)

	It("gives arm64 Neovim its PATH entry and the primary config files and themes", func() {
		app := load("neovim")
		config := app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: "ubuntu", Version: "24.04", Architecture: "arm64"})
		Expect(config.PostInstall).To(HaveLen(1))
		Expect(config.PostInstall[0].Path).To(Equal([]string{"/opt/nvim-linux-arm64/bin"}))
		primary := app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: "ubuntu", Version: "24.04", Architecture: "amd64"})
		Expect(config.ConfigFiles).To(Equal(primary.ConfigFiles))
		Expect(config.Themes).To(Equal(primary.Themes))
		Expect(config.SystemRequirements).To(Equal(primary.SystemRequirements))
	})

	It("keeps the packaged LazyGit on arm64 Debian releases that ship it", func() {
		app := load("lazygit")
		config := app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: "debian", Version: "13", Architecture: "arm64"})
		Expect(config.InstallMethod).To(Equal("apt"))
	})

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
		defer types.PinCatalogOSForTesting("linux")()
		defer types.PinCatalogArchForTesting("amd64")()
		app := load("github-cli")
		app.Linux.PlatformRequirements = nil
		Expect(app.GetOSConfig().AptSources[0].SourceRepo).To(HavePrefix("deb [arch=amd64 signed-by="))
	})
})
