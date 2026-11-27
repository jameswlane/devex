package channels_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/channels"
	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Channel resolution", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
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

	newKimi := func() types.CrossPlatformApp {
		return types.CrossPlatformApp{
			Name: "Kimi Code",
			Linux: types.OSConfig{
				InstallMethod:   "curlpipe",
				DownloadURL:     "https://code.kimi.com/kimi-code/install.sh",
				OfficialSupport: true,
				Alternatives: []types.OSConfig{
					{
						InstallMethod:   "mise",
						InstallCommand:  "node@lts npm:@moonshot-ai/kimi-code@latest",
						OfficialSupport: false,
					},
					{
						Channel:         "dnf-fedora",
						InstallMethod:   "dnf",
						InstallCommand:  "kimi-code",
						OfficialSupport: false,
						PlatformRequirements: []types.PlatformRequirement{
							{OS: "fedora"},
						},
					},
				},
			},
		}
	}

	newGit := func() types.CrossPlatformApp {
		return types.CrossPlatformApp{
			Name: "git",
			Linux: types.OSConfig{
				InstallMethod:   "apt",
				InstallCommand:  "git",
				OfficialSupport: true,
			},
		}
	}

	It("keeps default selection unchanged when no preference is set", func() {
		app := newKimi()
		selected, ch, err := channels.Resolve(app, channels.Preferences{}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("curlpipe"))
		Expect(selected).To(Equal(app))
	})

	It("applies explicit channel override above per-app and global order", func() {
		app := newKimi()
		prefs := channels.Preferences{
			Channel:     "mise",
			AppChannels: map[string]string{"kimi code": "curlpipe"},
			Order:       []string{"official"},
		}
		selected, ch, err := channels.Resolve(app, prefs, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("mise"))
		Expect(selected.GetOSConfig().InstallMethod).To(Equal("mise"))
	})

	It("applies per-app config override by name or slug above global order", func() {
		app := newKimi()
		prefs := channels.Preferences{
			AppChannels: map[string]string{"kimi-code": "mise"},
			Order:       []string{"official"},
		}
		selected, ch, err := channels.Resolve(app, prefs, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("mise"))
		Expect(selected.GetOSConfig().InstallMethod).To(Equal("mise"))
	})

	It("picks the first offered and available entry from channel_order", func() {
		app := newKimi()
		prefs := channels.Preferences{
			Order: []string{"dnf-fedora", "brew", "mise", "official"},
		}
		selected, ch, err := channels.Resolve(app, prefs, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("mise"))
		Expect(selected.GetOSConfig().InstallMethod).To(Equal("mise"))
	})

	It("supports 'official' in channel_order and explicit overrides", func() {
		app := newKimi()
		selected, ch, err := channels.Resolve(app, channels.Preferences{Order: []string{"official", "mise"}}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("curlpipe"))
		Expect(selected.GetOSConfig().OfficialSupport).To(BeTrue())
	})

	It("skips 'official' in channel_order when no available channel has official_support: true, while keeping explicit --channel official fallback", func() {
		unofficialApp := types.CrossPlatformApp{
			Name: "ToolNoOfficial",
			Linux: types.OSConfig{
				InstallMethod:   "apt",
				InstallCommand:  "tool",
				OfficialSupport: false,
				Alternatives: []types.OSConfig{
					{
						InstallMethod:   "mise",
						InstallCommand:  "tool@latest",
						OfficialSupport: false,
					},
				},
			},
		}

		selected, ch, err := channels.Resolve(unofficialApp, channels.Preferences{Order: []string{"official", "mise"}}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("mise"))
		Expect(selected.GetOSConfig().InstallMethod).To(Equal("mise"))

		// Explicit --channel official keeps its fallback-to-default behaviour.
		explicitSelected, explicitCh, err := channels.Resolve(unofficialApp, channels.Preferences{Channel: "official"}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(explicitCh).To(Equal("apt"))
		Expect(explicitSelected.GetOSConfig().InstallMethod).To(Equal("apt"))
	})

	It("falls back to default when no channel_order entry matches", func() {
		app := newGit()
		selected, ch, err := channels.Resolve(app, channels.Preferences{Order: []string{"mise", "brew"}}, "")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("apt"))
		Expect(selected).To(Equal(app))
	})

	It("fails with all failing apps when ResolveAll applies --channel across multiple apps", func() {
		_, err := channels.ResolveAll(
			[]types.CrossPlatformApp{newKimi(), newGit()},
			channels.Preferences{Channel: "mise"},
			nil,
		)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(`git has no channel "mise" on linux`))
		Expect(err.Error()).To(ContainSubstring(`available channels: apt (official, default)`))
	})

	It("lets recorded channel win over all preferences", func() {
		app := newKimi()
		prefs := channels.Preferences{
			Channel:     "mise",
			AppChannels: map[string]string{"kimi code": "mise"},
			Order:       []string{"mise"},
		}
		selected, ch, err := channels.Resolve(app, prefs, "curlpipe")
		Expect(err).NotTo(HaveOccurred())
		Expect(ch).To(Equal("curlpipe"))
		Expect(selected.GetOSConfig().InstallMethod).To(Equal("curlpipe"))
	})

	Describe("ResolveForInstall", func() {
		It("keeps the recorded channel when no preference is set", func() {
			_, ch, err := channels.ResolveForInstall(newKimi(), channels.Preferences{}, "mise")
			Expect(err).NotTo(HaveOccurred())
			Expect(ch).To(Equal("mise"))
		})

		It("refuses instead of switching to the default when the recorded channel is no longer offered", func() {
			_, _, err := channels.ResolveForInstall(newKimi(), channels.Preferences{}, "retired-snap")
			Expect(err).To(MatchError(And(
				ContainSubstring(`Kimi Code is installed through channel "retired-snap", which the catalog no longer offers here`),
				ContainSubstring(`devex uninstall --app "Kimi Code"`),
				ContainSubstring("curlpipe"),
			)))
		})

		It("lets an explicit channel replace a retired recorded channel", func() {
			_, ch, err := channels.ResolveForInstall(newKimi(), channels.Preferences{Channel: "curlpipe"}, "retired-snap")
			Expect(err).NotTo(HaveOccurred())
			Expect(ch).To(Equal("curlpipe"))
		})
	})

	Describe("installed records", func() {
		newMultiDnf := func() types.CrossPlatformApp {
			return types.CrossPlatformApp{
				Name: "Tool",
				Linux: types.OSConfig{
					InstallMethod: "apt", InstallCommand: "tool", OfficialSupport: true,
					Alternatives: []types.OSConfig{
						{Channel: "dnf-a", InstallMethod: "dnf", InstallCommand: "tool-a"},
						{Channel: "dnf-b", InstallMethod: "dnf", InstallCommand: "tool-b"},
					},
				},
			}
		}

		It("keeps catalog order for a manager-only receipt whichever channel is selected (M2)", func() {
			for _, selectedName := range []string{"dnf-a", "dnf-b"} {
				selected, err := newMultiDnf().WithChannel(selectedName)
				Expect(err).NotTo(HaveOccurred())
				Expect(channels.InferRecordedChannel(inventory.Receipt{Manager: "dnf"}, selected)).To(Equal("dnf-a"), selectedName)
			}
		})

		It("ignores a saved definition that does not decode (Q1)", func() {
			repo := mocks.NewMockRepository()
			Expect(repo.Set("installed-app-definition:tool", `{"Name":"Tool","Linux":{"InstallMethod":"dnf","InstallCommand":7}}`)).To(Succeed())
			Expect(repo.Set(inventory.ReceiptKey("Tool"), `{"manager":"dnf"}`)).To(Succeed())
			Expect(channels.InstalledChannelForApp(repo, newMultiDnf())).To(Equal("dnf-a"))
		})

		It("inspects the saved definition when it agrees with the recorded channel, even after a package rename (Q2)", func() {
			repo := mocks.NewMockRepository()
			saved := newGit()
			saved.Linux.InstallCommand = "old-git"
			data, err := json.Marshal(saved)
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Set("installed-app-definition:git", string(data))).To(Succeed())
			Expect(repo.Set(inventory.ReceiptKey("git"), `{"manager":"apt","channel":"apt","packages":["old-git"]}`)).To(Succeed())

			definition, found, err := channels.StatusDefinition(repo, newGit(), true, "git")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(definition.GetOSConfig().InstallCommand).To(Equal("old-git"))
		})

		It("reports a recorded channel that neither the saved definition nor the catalog has as unresolved (M4)", func() {
			repo := mocks.NewMockRepository()
			data, err := json.Marshal(newGit())
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Set("installed-app-definition:git", string(data))).To(Succeed())
			Expect(repo.Set(inventory.ReceiptKey("git"), `{"manager":"flatpak","channel":"flatpak"}`)).To(Succeed())

			_, found, err := channels.StatusDefinition(repo, newGit(), true, "git")
			Expect(found).To(BeTrue())
			Expect(err).To(MatchError(ContainSubstring(`"flatpak"`)))
		})
	})

	Describe("Legacy receipt inference", func() {
		It("uses receipt.Channel when present", func() {
			app := newKimi()
			Expect(channels.InferRecordedChannel(inventory.Receipt{Manager: "curlpipe", Channel: "mise"}, app)).To(Equal("mise"))
		})

		It("infers channel from receipt.Manager when receipt.Channel is empty", func() {
			app := newKimi()
			repo := mocks.NewMockRepository()
			data, err := json.Marshal(inventory.Receipt{Manager: "mise"})
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Set(inventory.ReceiptKey("Kimi Code"), string(data))).To(Succeed())

			Expect(channels.RecordedChannelForApp(repo, app)).To(Equal("mise"))
		})

		Context("on Rocky Linux, where Fedora's and RHEL's dnf entries both apply", func() {
			BeforeEach(func() {
				DeferCleanup(types.PinCatalogPlatformForTesting(platform.DetectionResult{
					OS: "linux", Distribution: "rocky", DistributionLike: []string{"rhel", "centos", "fedora"}, Version: "9.4", Architecture: "amd64",
				}))
			})

			newToolchain := func() types.CrossPlatformApp {
				return types.CrossPlatformApp{
					Name: "Build Essential",
					Linux: types.OSConfig{
						InstallMethod:        "apt",
						InstallCommand:       "build-essential",
						OfficialSupport:      true,
						PlatformRequirements: []types.PlatformRequirement{{OS: "debian"}, {OS: "ubuntu"}},
						Alternatives: []types.OSConfig{
							{InstallMethod: "dnf", InstallCommand: "@development-tools gcc", OfficialSupport: true, PlatformRequirements: []types.PlatformRequirement{{OS: "fedora"}}},
							{Channel: "dnf-rhel", InstallMethod: "dnf", InstallCommand: "@development gcc", OfficialSupport: true, PlatformRequirements: []types.PlatformRequirement{{OS: "centos", Version: "8+"}, {OS: "rhel", Version: "8+"}}},
						},
					},
				}
			}

			It("infers the platform's default entry for a manager-only receipt", func() {
				app := newToolchain()
				Expect(app.SelectedChannel()).To(Equal("dnf-rhel"))
				Expect(channels.InferRecordedChannel(inventory.Receipt{Manager: "dnf"}, app)).To(Equal("dnf-rhel"))
			})

			It("maps a pre-channel saved definition to its RHEL variant, as switch checks do (M1)", func() {
				app := newToolchain()
				legacy := types.CrossPlatformApp{Name: "Build Essential", Linux: types.OSConfig{InstallMethod: "dnf", InstallCommand: "@development gcc"}}
				data, err := json.Marshal(legacy)
				Expect(err).NotTo(HaveOccurred())
				repo := mocks.NewMockRepository()
				Expect(repo.Set("installed-app-definition:build essential", string(data))).To(Succeed())
				Expect(repo.Set(inventory.ReceiptKey("Build Essential"), `{"manager":"dnf","packages":["@development","gcc"]}`)).To(Succeed())

				Expect(channels.InstalledChannelForApp(repo, app)).To(Equal("dnf-rhel"))
				_, ch, err := channels.ResolveForInstall(app, channels.Preferences{}, channels.InstalledChannelForApp(repo, app))
				Expect(err).NotTo(HaveOccurred())
				Expect(ch).To(Equal("dnf-rhel"))
				Expect(channels.CheckRefusedChannelSwitch(repo, app)).To(Succeed())
			})

			It("infers the same installed channel whichever channel is selected (M2)", func() {
				fedora, err := newToolchain().WithChannel("dnf")
				Expect(err).NotTo(HaveOccurred())
				Expect(channels.InferRecordedChannel(inventory.Receipt{Manager: "dnf"}, fedora)).To(Equal("dnf-rhel"))
			})

			It("breaks a package-match tie with the platform default (M2)", func() {
				app := newToolchain()
				app.Linux.Alternatives[0].InstallCommand = "@development gcc"
				fedora, err := app.WithChannel("dnf")
				Expect(err).NotTo(HaveOccurred())
				Expect(channels.InferRecordedChannel(inventory.Receipt{Manager: "dnf", Packages: []string{"@development", "gcc"}}, fedora)).To(Equal("dnf-rhel"))
			})

			It("prefers the entry whose packages match the receipt", func() {
				app := newToolchain()
				Expect(channels.InferRecordedChannel(inventory.Receipt{Manager: "dnf", Packages: []string{"@development-tools", "gcc"}}, app)).To(Equal("dnf"))
			})
		})

		It("returns empty string so callers fall back to default when legacy manager does not match any channel", func() {
			app := newKimi()
			Expect(channels.InferRecordedChannel(inventory.Receipt{Manager: "flatpak"}, app)).To(BeEmpty())
		})
	})

	Describe("PreferencesFromViper", func() {
		It("reads channel, channels map, and comma-separated or slice channel_order", func() {
			viper.Set("channel", "mise")
			viper.Set("channels", map[string]any{"Kimi Code": "curlpipe"})
			viper.Set("channel_order", "official, mise, brew")

			prefs := channels.PreferencesFromViper()
			Expect(prefs.Channel).To(Equal("mise"))
			Expect(prefs.AppChannels).To(HaveKeyWithValue("kimi code", "curlpipe"))
			Expect(prefs.Order).To(Equal([]string{"official", "mise", "brew"}))
		})
	})
})
