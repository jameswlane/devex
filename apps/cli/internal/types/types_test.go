package types_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("AppConfig", func() {
	Context("Validate", func() {
		It("returns an error if the name is empty", func() {
			app := types.AppConfig{
				InstallMethod: "apt",
			}
			err := app.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("app name is required"))
		})

		It("returns an error if the install method is empty", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "TestApp",
				},
			}
			err := app.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("install method is required"))
		})

		It("returns an error if APT sources are invalid", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "TestApp",
				},
				InstallMethod: "apt",
				AptSources: []types.AptSource{
					{SourceRepo: "", SourceName: ""},
				},
			}
			err := app.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("APT source at index 0 must have repo and list_file defined"))
		})

		It("validates successfully with valid fields", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "TestApp",
				},
				InstallMethod: "curlpipe",
				DownloadURL:   "https://example.com",
			}
			err := app.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("DockerOptions", func() {
	Context("Validate", func() {
		It("returns an error if ContainerName is empty", func() {
			options := types.DockerOptions{}
			err := options.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("container name is required"))
		})

		It("validates successfully with a valid ContainerName", func() {
			options := types.DockerOptions{
				ContainerName: "test-container",
			}
			err := options.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("Font", func() {
	Context("Validate", func() {
		It("returns an error if Name is empty", func() {
			font := types.Font{
				Method: "url",
			}
			err := font.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("font name is required"))
		})

		It("returns an error if Method is empty", func() {
			font := types.Font{
				Name: "TestFont",
			}
			err := font.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("install method is required"))
		})

		It("validates successfully with valid fields", func() {
			font := types.Font{
				Name:   "TestFont",
				Method: "url",
			}
			err := font.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("AptSource", func() {
	It("allows creation with all fields", func() {
		source := types.AptSource{
			KeySource:      "https://example.com/key",
			KeyName:        "example-key",
			SourceRepo:     "http://example.com/repo",
			SourceName:     "example-repo",
			RequireDearmor: true,
		}

		Expect(source.KeySource).To(Equal("https://example.com/key"))
		Expect(source.KeyName).To(Equal("example-key"))
		Expect(source.SourceRepo).To(Equal("http://example.com/repo"))
		Expect(source.SourceName).To(Equal("example-repo"))
		Expect(source.RequireDearmor).To(BeTrue())
	})
})

var _ = Describe("Linux catalog selection", func() {
	It("preserves an unrestricted primary even when an alternative matches", func() {
		app := types.CrossPlatformApp{Linux: types.OSConfig{InstallMethod: "apt", Alternatives: []types.OSConfig{{InstallMethod: "dnf", PlatformRequirements: []types.PlatformRequirement{{OS: "fedora"}}}}}}
		Expect(app.LinuxConfigForDistribution("fedora")).To(Equal(app.Linux))
		Expect(app.LinuxConfigForDistribution("unmatched")).To(Equal(app.Linux))
	})
	It("matches the exact ID before parent IDs, then parents in order", func() {
		app := types.CrossPlatformApp{Linux: types.OSConfig{InstallMethod: "apt", PlatformRequirements: []types.PlatformRequirement{{OS: "alpine"}}, Alternatives: []types.OSConfig{
			{InstallMethod: "ubuntu-apt", PlatformRequirements: []types.PlatformRequirement{{OS: "ubuntu"}}},
			{InstallMethod: "mint-apt", PlatformRequirements: []types.PlatformRequirement{{OS: "linuxmint"}}},
			{InstallMethod: "snap"},
		}}}
		Expect(app.LinuxConfigForDistribution("linuxmint", "ubuntu", "debian").InstallMethod).To(Equal("mint-apt"))
		Expect(app.LinuxConfigForDistribution("pop", "ubuntu", "debian").InstallMethod).To(Equal("ubuntu-apt"))
		Expect(app.LinuxConfigForDistribution("unmatched")).To(Equal(app.Linux))
	})
	It("keeps a primary matching ID_LIKE before considering an exact-ID alternative", func() {
		app := types.CrossPlatformApp{Linux: types.OSConfig{InstallMethod: "apt", PlatformRequirements: []types.PlatformRequirement{{OS: "debian"}}, Alternatives: []types.OSConfig{
			{InstallMethod: "other", PlatformRequirements: []types.PlatformRequirement{{OS: "linuxmint"}}},
		}}}
		Expect(app.LinuxConfigForDistribution("linuxmint", "ubuntu", "debian")).To(Equal(app.Linux))
	})

	DescribeTable("honors version and architecture requirements on alternatives", func(version, arch, requirement string, parents []string, matches bool) {
		app := types.CrossPlatformApp{Linux: types.OSConfig{InstallMethod: "primary", PlatformRequirements: []types.PlatformRequirement{{OS: "alpine"}}, Alternatives: []types.OSConfig{
			{InstallMethod: "apt", PlatformRequirements: []types.PlatformRequirement{{OS: "ubuntu", Version: requirement, Arch: "x86_64"}}},
		}}}
		distro := "ubuntu"
		if len(parents) > 0 {
			distro = "pop"
		}
		selected := app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: distro, DistributionLike: parents, Version: version, Architecture: arch})
		if matches {
			Expect(selected.InstallMethod).To(Equal("apt"))
		} else {
			Expect(selected).To(Equal(app.Linux))
		}
	},
		Entry("too old", "24.04", "amd64", "25.10+", nil, false),
		Entry("minimum", "25.10", "amd64", "25.10+", nil, true),
		Entry("newer", "26.04", "x86_64", "25.10+", nil, true),
		Entry("wrong architecture", "25.10", "arm64", "25.10+", nil, false),
		Entry("unknown architecture", "25.10", "", "25.10+", nil, false),
		Entry("unknown version", "", "amd64", "25.10+", nil, false),
		Entry("invalid constraint", "25.10", "amd64", "invalid", nil, false),
		Entry("comparison constraint", "24.04", "amd64", ">=22.04", nil, true),
		Entry("exact version", "24.04", "amd64", "22.04", nil, false),
		Entry("parent match", "25.10", "amd64", "25.10+", []string{"ubuntu", "debian"}, true),
		Entry("parent too old", "24.04", "amd64", "25.10+", []string{"ubuntu", "debian"}, false),
	)

	It("checks primary version and architecture before selecting the first matching alternative", func() {
		app := types.CrossPlatformApp{Linux: types.OSConfig{InstallMethod: "primary", PlatformRequirements: []types.PlatformRequirement{{OS: "ubuntu", Version: "24.04+", Arch: "amd64"}}, Alternatives: []types.OSConfig{
			{InstallMethod: "fallback", PlatformRequirements: []types.PlatformRequirement{{OS: "linux"}}},
			{InstallMethod: "later", PlatformRequirements: []types.PlatformRequirement{{OS: "linux"}}},
		}}}
		Expect(app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: "ubuntu", Version: "24.04", Architecture: "amd64"})).To(Equal(app.Linux))
		Expect(app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: "ubuntu", Version: "22.04", Architecture: "amd64"}).InstallMethod).To(Equal("fallback"))
		Expect(app.LinuxConfigForPlatform(platform.DetectionResult{Distribution: "ubuntu", Version: "24.04", Architecture: "arm64"}).InstallMethod).To(Equal("fallback"))
	})

})
