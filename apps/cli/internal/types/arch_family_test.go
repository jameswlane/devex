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

var (
	archLinux   = platform.DetectionResult{Distribution: "arch", Architecture: "amd64"}
	manjaro     = platform.DetectionResult{Distribution: "manjaro", DistributionLike: []string{"arch"}, Version: "25.0.10", Architecture: "amd64"}
	endeavourOS = platform.DetectionResult{Distribution: "endeavouros", DistributionLike: []string{"arch"}, Architecture: "amd64"}
	debian13    = platform.DetectionResult{Distribution: "debian", Version: "13", Architecture: "amd64"}
	ubuntu2404  = platform.DetectionResult{Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "amd64"}
	fedora44    = platform.DetectionResult{Distribution: "fedora", Version: "44", Architecture: "amd64"}
)

var _ = Describe("OSConfig.IsPlatformSupported", func() {
	supported := func(detected platform.DetectionResult, reqs ...types.PlatformRequirement) bool {
		defer types.PinCatalogOSForTesting("linux")()
		defer types.PinCatalogArchForTesting("amd64")()
		defer types.PinCatalogPlatformForTesting(detected)()
		config := types.OSConfig{InstallMethod: "pacman", PlatformRequirements: reqs}
		return config.IsPlatformSupported()
	}
	arch := types.PlatformRequirement{OS: "arch"}

	DescribeTable("matches an Arch rule only on Arch-based systems",
		func(detected platform.DetectionResult, want bool) {
			Expect(supported(detected, arch)).To(Equal(want))
		},
		Entry("Arch", archLinux, true),
		Entry("Manjaro through ID_LIKE", manjaro, true),
		Entry("EndeavourOS through ID_LIKE", endeavourOS, true),
		Entry("Debian", debian13, false),
		Entry("Ubuntu", ubuntu2404, false),
		Entry("Fedora", fedora44, false),
	)

	It("checks the detected version", func() {
		noble := types.PlatformRequirement{OS: "ubuntu", Version: ">=24.04"}
		Expect(supported(ubuntu2404, noble)).To(BeTrue())
		Expect(supported(platform.DetectionResult{Distribution: "ubuntu", Version: "22.04", Architecture: "amd64"}, noble)).To(BeFalse())
	})

	It("checks the detected architecture", func() {
		Expect(supported(archLinux, types.PlatformRequirement{OS: "arch", Arch: "x86_64"})).To(BeTrue())
		Expect(supported(archLinux, types.PlatformRequirement{OS: "arch", Arch: "arm64"})).To(BeFalse())
	})

	It("picks the Arch alternative only on Arch-based systems", func() {
		defer types.PinCatalogOSForTesting("linux")()
		app := types.CrossPlatformApp{Linux: types.OSConfig{
			InstallMethod: "apt", PlatformRequirements: []types.PlatformRequirement{{OS: "debian"}},
			Alternatives: []types.OSConfig{
				{InstallMethod: "pacman", PlatformRequirements: []types.PlatformRequirement{arch}},
			},
		}}
		for detected, method := range map[*platform.DetectionResult]string{
			&archLinux: "pacman", &manjaro: "pacman", &endeavourOS: "pacman",
			&debian13: "apt", &ubuntu2404: "apt",
		} {
			restore := types.PinCatalogPlatformForTesting(*detected)
			Expect(app.GetBestOSConfig().InstallMethod).To(Equal(method), detected.Distribution)
			restore()
		}
	})
})

// Every shipped entry with Arch or Manjaro rules selects the same way on Arch,
// Manjaro and EndeavourOS, and never selects them on other distributions.
var _ = Describe("Shipped catalog Arch-family selection", func() {
	var apps map[string]types.CrossPlatformApp

	BeforeEach(func() {
		files, err := filepath.Glob("../../config/applications/*/*.yaml")
		Expect(err).NotTo(HaveOccurred())
		Expect(files).NotTo(BeEmpty())
		apps = map[string]types.CrossPlatformApp{}
		for _, file := range files {
			data, err := os.ReadFile(file)
			Expect(err).NotTo(HaveOccurred())
			var app types.CrossPlatformApp
			Expect(yaml.Unmarshal(data, &app)).To(Succeed(), file)
			apps[file] = app
		}
	})

	archOnly := func(config types.OSConfig) bool {
		if len(config.PlatformRequirements) == 0 {
			return false
		}
		for _, req := range config.PlatformRequirements {
			if req.OS != "arch" && req.OS != "manjaro" && req.OS != "endeavouros" {
				return false
			}
		}
		return true
	}

	selected := func(app types.CrossPlatformApp, detected platform.DetectionResult) types.OSConfig {
		defer types.PinCatalogOSForTesting("linux")()
		defer types.PinCatalogArchForTesting("amd64")()
		defer types.PinCatalogPlatformForTesting(detected)()
		return app.GetBestOSConfig()
	}

	It("selects identically on Arch, Manjaro and EndeavourOS", func() {
		pacman := 0
		for file, app := range apps {
			onArch := selected(app, archLinux)
			Expect(selected(app, manjaro).InstallMethod).To(Equal(onArch.InstallMethod), file)
			Expect(selected(app, endeavourOS).InstallMethod).To(Equal(onArch.InstallMethod), file)
			if onArch.InstallMethod == "pacman" {
				pacman++
			}
		}
		Expect(pacman).To(BeNumerically(">", 10))
	})

	It("never selects an Arch-only method on Debian, Ubuntu or Fedora", func() {
		for file, app := range apps {
			for _, detected := range []platform.DetectionResult{debian13, ubuntu2404, fedora44} {
				config := selected(app, detected)
				Expect(archOnly(config)).To(BeFalse(), "%s on %s", file, detected.Distribution)
				Expect(config.InstallMethod).NotTo(BeElementOf("pacman", "yay"), "%s on %s", file, detected.Distribution)
			}
		}
	})
})
