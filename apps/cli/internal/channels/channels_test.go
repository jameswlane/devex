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
