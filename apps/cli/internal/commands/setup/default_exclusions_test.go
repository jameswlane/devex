package setup

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Default exclusion configuration", func() {
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		viper.Reset()
		settings = config.CrossPlatformSettings{ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "bat", Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "bat"}},
			{Name: "gh", Default: true, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "gh"}},
			{Name: "macos-only", Default: true, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "macos-only"}},
		}}
		DeferCleanup(viper.Reset)
	})

	It("plans only defaults compatible with the detected platform in an automated run", func() {
		settings.ApplicationOverrides = append(settings.ApplicationOverrides, types.CrossPlatformApp{
			Name: "everywhere", Default: true, AllPlatforms: types.OSConfig{InstallMethod: "mise", InstallCommand: "everywhere@latest"},
		})
		model, err := createAutomatedSetupModel(mocks.NewMockRepository(), settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(model.system.detectedPlatform.OS).To(Equal(platform.DetectPlatform().OS))

		defaults := model.getDefaultApps()
		names := make([]string, 0, len(defaults))
		for _, app := range defaults {
			names = append(names, app.Name)
		}
		Expect(names).To(ContainElement("everywhere"))
		if model.system.detectedPlatform.OS == "linux" {
			Expect(names).To(ConsistOf("bat", "gh", "everywhere"))
		}
	})

	It("seeds automated exclusions case-insensitively from setup.skip_defaults", func() {
		viper.Set("setup.skip_defaults", []string{"BAT"})

		model, err := createAutomatedSetupModel(mocks.NewMockRepository(), settings)

		Expect(err).NotTo(HaveOccurred())
		Expect(model.selections.excludedDefaults).To(HaveKeyWithValue("bat", true))
		Expect(model.buildAppList()).NotTo(ContainElement(HaveField("Name", "bat")))
		Expect(model.buildAppList()).To(ContainElement(HaveField("Name", "gh")))
	})

	It("seeds interactive exclusions before setup starts", func() {
		viper.Set("setup.skip_defaults", []string{"Gh"})

		model := NewSetupModel(mocks.NewMockRepository(), settings, platform.DetectionResult{OS: "linux"})

		Expect(model.selections.excludedDefaults).To(HaveKeyWithValue("gh", true))
	})

	It("rejects unknown or unavailable names with the valid platform defaults", func() {
		viper.Set("setup.skip_defaults", []string{"missing"})

		_, err := createAutomatedSetupModel(mocks.NewMockRepository(), settings)

		Expect(err).To(MatchError(And(
			ContainSubstring("unknown default tool \"missing\""),
			ContainSubstring("bat, gh"),
		)))
		Expect(err.Error()).NotTo(ContainSubstring("macos-only"))
	})

	It("accepts the comma-separated value supplied through the environment", func() {
		viper.SetEnvPrefix("DEVEX")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
		viper.AutomaticEnv()
		GinkgoT().Setenv("DEVEX_SETUP_SKIP_DEFAULTS", "GH, bat,")

		model, err := createAutomatedSetupModel(mocks.NewMockRepository(), settings)

		Expect(err).NotTo(HaveOccurred())
		Expect(model.selections.excludedDefaults).To(Equal(map[string]bool{"bat": true, "gh": true}))
	})
})
