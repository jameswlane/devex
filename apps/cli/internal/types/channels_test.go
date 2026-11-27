package types_test

import (
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Application install channels", func() {
	BeforeEach(func() {
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogArchForTesting("amd64"))
		DeferCleanup(types.PinCatalogPlatformForTesting(platform.DetectionResult{
			OS:               "linux",
			Distribution:     "ubuntu",
			DistributionLike: []string{"debian"},
			Version:          "24.04",
			Architecture:     "amd64",
		}))
	})

	Context("with per-architecture variants of one channel", func() {
		variantApp := func() types.CrossPlatformApp {
			return types.CrossPlatformApp{
				Name: "squad",
				Linux: types.OSConfig{
					InstallMethod:        "curlpipe",
					DownloadURL:          "https://example.invalid/squad_amd64.tar.gz",
					OfficialSupport:      true,
					PlatformRequirements: []types.PlatformRequirement{{OS: "linux", Arch: "amd64"}},
					Alternatives: []types.OSConfig{
						{
							InstallMethod:        "curlpipe",
							DownloadURL:          "https://example.invalid/squad_arm64.tar.gz",
							OfficialSupport:      true,
							PlatformRequirements: []types.PlatformRequirement{{OS: "linux", Arch: "arm64"}},
						},
						{InstallMethod: "mise", InstallCommand: "squad@latest"},
					},
				},
			}
		}

		It("lists the channel once and selects the variant for this architecture", func() {
			DeferCleanup(types.PinCatalogArchForTesting("arm64"))
			DeferCleanup(types.PinCatalogPlatformForTesting(platform.DetectionResult{
				OS: "linux", Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "arm64",
			}))
			app := variantApp()
			Expect(app.Channels()).To(HaveLen(2))
			Expect(app.Channels()[0]).To(And(HaveField("Name", "curlpipe"), HaveField("Available", true), HaveField("Default", true)))

			fromMise, err := app.WithChannel("mise")
			Expect(err).NotTo(HaveOccurred())
			back, err := fromMise.WithChannel("curlpipe")
			Expect(err).NotTo(HaveOccurred())
			Expect(back.GetOSConfig().DownloadURL).To(Equal("https://example.invalid/squad_arm64.tar.gz"))
		})

		It("takes official status from the variant available on this architecture", func() {
			DeferCleanup(types.PinCatalogArchForTesting("arm64"))
			DeferCleanup(types.PinCatalogPlatformForTesting(platform.DetectionResult{
				OS: "linux", Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "arm64",
			}))
			app := variantApp()
			app.Linux.Alternatives[0].OfficialSupport = false
			Expect(app.Channels()[0]).To(And(HaveField("Name", "curlpipe"), HaveField("Available", true), HaveField("Official", false)))
		})

		It("keeps the amd64 variant on amd64", func() {
			selected, err := variantApp().WithChannel("curlpipe")
			Expect(err).NotTo(HaveOccurred())
			Expect(selected.GetOSConfig().DownloadURL).To(Equal("https://example.invalid/squad_amd64.tar.gz"))
		})
	})

	newMultiChannelApp := func() types.CrossPlatformApp {
		return types.CrossPlatformApp{
			Name:        "codex",
			Description: "Coding agent CLI",
			Category:    "AI Coding Agents",
			Linux: types.OSConfig{
				InstallMethod:   "curlpipe",
				DownloadURL:     "https://example.invalid/install.sh",
				OfficialSupport: true,
				Alternatives: []types.OSConfig{
					{
						InstallMethod:   "mise",
						InstallCommand:  "codex@latest",
						OfficialSupport: false,
					},
					{
						Channel:         "dnf-fedora",
						InstallMethod:   "dnf",
						InstallCommand:  "codex",
						OfficialSupport: false,
						PlatformRequirements: []types.PlatformRequirement{
							{OS: "fedora"},
						},
					},
				},
			},
			MacOS: types.OSConfig{
				InstallMethod:   "brew",
				InstallCommand:  "codex",
				BrewCask:        true,
				OfficialSupport: true,
				Alternatives: []types.OSConfig{
					{
						InstallMethod:   "mise",
						InstallCommand:  "codex@latest",
						OfficialSupport: false,
					},
				},
			},
		}
	}

	Describe("ChannelName", func() {
		It("uses explicit channel key when set and falls back to install_method", func() {
			Expect((types.OSConfig{InstallMethod: "apt"}).ChannelName()).To(Equal("apt"))
			Expect((types.OSConfig{Channel: "apt-debian", InstallMethod: "apt"}).ChannelName()).To(Equal("apt-debian"))
		})
	})

	Describe("Channels", func() {
		It("lists primary and alternatives with official, default and availability metadata", func() {
			app := newMultiChannelApp()
			Expect(app.Channels()).To(Equal([]types.ChannelInfo{
				{Name: "curlpipe", InstallMethod: "curlpipe", Official: true, Default: true, Available: true},
				{Name: "mise", InstallMethod: "mise", Official: false, Default: false, Available: true},
				{Name: "dnf-fedora", InstallMethod: "dnf", Official: false, Default: false, Available: false},
			}))
		})

		It("keeps default and official metadata intact after WithChannel selects an alternative", func() {
			app := newMultiChannelApp()
			selected, err := app.WithChannel("mise")
			Expect(err).NotTo(HaveOccurred())
			Expect(selected.GetOSConfig().InstallMethod).To(Equal("mise"))
			Expect(selected.SelectedChannel()).To(Equal("mise"))

			channels := selected.Channels()
			Expect(channels).To(HaveLen(3))
			Expect(channels).To(ContainElements(
				types.ChannelInfo{Name: "curlpipe", InstallMethod: "curlpipe", Official: true, Default: true, Available: true},
				types.ChannelInfo{Name: "mise", InstallMethod: "mise", Official: false, Default: false, Available: true},
				types.ChannelInfo{Name: "dnf-fedora", InstallMethod: "dnf", Official: false, Default: false, Available: false},
			))
		})
	})

	Describe("WithChannel", func() {
		It("selects an available alternative channel without mutating the original", func() {
			app := newMultiChannelApp()
			selected, err := app.WithChannel("mise")
			Expect(err).NotTo(HaveOccurred())
			Expect(selected.GetOSConfig().InstallMethod).To(Equal("mise"))
			Expect(selected.GetOSConfig().InstallCommand).To(Equal("codex@latest"))
			Expect(app.GetOSConfig().InstallMethod).To(Equal("curlpipe"))
		})

		It("resolves the special 'official' keyword to the official channel", func() {
			app := newMultiChannelApp()
			miseApp, err := app.WithChannel("mise")
			Expect(err).NotTo(HaveOccurred())

			official, err := miseApp.WithChannel("official")
			Expect(err).NotTo(HaveOccurred())
			Expect(official.GetOSConfig().InstallMethod).To(Equal("curlpipe"))
			Expect(official.SelectedChannel()).To(Equal("curlpipe"))
		})

		It("falls back to the default channel for 'official' when none is marked official", func() {
			app := types.CrossPlatformApp{
				Name: "tool",
				Linux: types.OSConfig{
					InstallMethod:  "apt",
					InstallCommand: "tool",
					Alternatives: []types.OSConfig{
						{InstallMethod: "mise", InstallCommand: "tool@latest"},
					},
				},
			}
			official, err := app.WithChannel("official")
			Expect(err).NotTo(HaveOccurred())
			Expect(official.GetOSConfig().InstallMethod).To(Equal("apt"))
		})

		It("returns a helpful error listing available channels for an unknown channel", func() {
			app := newMultiChannelApp()
			_, err := app.WithChannel("brew")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`codex has no channel "brew" on linux; available channels: curlpipe (official, default), mise`))
		})

		It("returns a helpful error for a channel whose platform_requirements do not match", func() {
			app := newMultiChannelApp()
			_, err := app.WithChannel("dnf-fedora")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`codex`))
			Expect(err.Error()).To(ContainSubstring(`"dnf-fedora"`))
			Expect(err.Error()).To(ContainSubstring(`available channels: curlpipe (official, default), mise`))
		})

		It("preserves architecture placeholder substitution and brew cask selection on chosen channels", func() {
			aptApp := types.CrossPlatformApp{
				Name: "editor",
				Linux: types.OSConfig{
					InstallMethod:  "snap",
					InstallCommand: "editor",
					Alternatives: []types.OSConfig{
						{
							InstallMethod:  "apt",
							InstallCommand: "editor",
							AptSources: []types.AptSource{
								{SourceRepo: "deb [arch=%ARCHITECTURE%] https://example.invalid stable main"},
							},
						},
					},
				},
			}
			selectedApt, err := aptApp.WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(selectedApt.GetOSConfig().AptSources[0].SourceRepo).To(ContainSubstring("arch=amd64"))

			DeferCleanup(types.PinCatalogOSForTesting("darwin"))
			macApp := types.CrossPlatformApp{
				Name: "codex",
				MacOS: types.OSConfig{
					InstallMethod:  "mise",
					InstallCommand: "codex@latest",
					Alternatives: []types.OSConfig{
						{
							InstallMethod:  "brew",
							InstallCommand: "codex",
							BrewCask:       true,
						},
					},
				},
			}
			selectedBrew, err := macApp.WithChannel("brew")
			Expect(err).NotTo(HaveOccurred())
			Expect(selectedBrew.GetOSConfig().Options).To(ContainElement("--cask"))
		})
	})
})
