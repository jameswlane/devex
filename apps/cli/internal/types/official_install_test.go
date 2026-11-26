package types_test

import (
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

// Each distro selects the method the app's upstream project documents for it.
var _ = Describe("Shipped catalog official Linux install methods", func() {
	load := func(name string) types.CrossPlatformApp {
		data, err := os.ReadFile(filepath.Join("../../config/applications/development", name+".yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())
		return app
	}

	DescribeTable("selects the upstream-documented method",
		func(name, distro, version, method string, like ...string) {
			app := load(name)
			config := app.LinuxConfigForPlatform(platform.DetectionResult{
				Distribution: distro, DistributionLike: like, Version: version, Architecture: "amd64",
			})
			Expect(config.InstallMethod).To(Equal(method))
		},
		Entry("LazyGit on Fedora", "lazygit", "fedora", "44", "dnf"),
		Entry("LazyGit on CentOS Stream", "lazygit", "centos", "9", "dnf", "rhel", "fedora"),
		Entry("LazyGit on Arch", "lazygit", "arch", "", "pacman"),
		Entry("LazyGit on Debian 13", "lazygit", "debian", "13", "apt"),
		Entry("LazyGit on Ubuntu 25.10", "lazygit", "ubuntu", "25.10", "apt", "debian"),
		Entry("LazyGit on Debian 12", "lazygit", "debian", "12", "curlpipe"),
		Entry("LazyGit on Ubuntu 24.04", "lazygit", "ubuntu", "24.04", "curlpipe", "debian"),
		Entry("LazyGit on an unknown distro", "lazygit", "unknown", "1", "curlpipe"),
		Entry("Ghostty on Fedora", "ghostty", "fedora", "44", "dnf"),
		Entry("Ghostty on Arch", "ghostty", "arch", "", "pacman"),
		Entry("Ghostty on Void", "ghostty", "void", "", "xbps"),
		Entry("Ghostty on Ubuntu 26.04", "ghostty", "ubuntu", "26.04", "apt", "debian"),
	)

	It("adds no third-party APT repository for Ghostty on older Debian and Ubuntu", func() {
		app := load("ghostty")
		for _, detected := range []platform.DetectionResult{
			{Distribution: "debian", Version: "12", Architecture: "amd64"},
			{Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "22.04", Architecture: "amd64"},
		} {
			config := app.LinuxConfigForPlatform(detected)
			Expect(config.InstallMethod).To(Equal("apt"), detected.Distribution)
			Expect(config.AptSources).To(BeEmpty(), detected.Distribution)
		}
	})

	It("adds the COPR before installing Ghostty and LazyGit on Fedora", func() {
		fedora := platform.DetectionResult{Distribution: "fedora", Version: "44", Architecture: "amd64"}
		for name, copr := range map[string]string{
			"lazygit": "dnf copr enable -y dejan/lazygit",
			"ghostty": "dnf copr enable -y scottames/ghostty",
		} {
			app := load(name)
			config := app.LinuxConfigForPlatform(fedora)
			Expect(config.PreInstall).NotTo(BeEmpty(), name)
			Expect(config.PreInstall[0].Shell).To(Equal(copr), name)
		}
	})
})
