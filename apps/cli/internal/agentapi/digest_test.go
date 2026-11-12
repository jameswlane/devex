package agentapi_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Plan digests", func() {
	base := func() []types.CrossPlatformApp {
		return []types.CrossPlatformApp{
			{Name: "base", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "base"}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "base"}},
			{Name: "lib", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "lib"}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "lib"}},
			{Name: "app", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "app", Dependencies: []string{"base", "lib"}}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "app", Dependencies: []string{"base", "lib"}}},
		}
	}
	digest := func(apps []types.CrossPlatformApp) string {
		settings := config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir(), ApplicationOverrides: apps}
		_, value, err := agentapi.PlanDigest(context.Background(), settings, agentapi.PlanRequest{Names: []string{"app"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(value).To(MatchRegexp(`^sha256:[0-9a-f]{64}$`))
		return value
	}
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
	})

	It("is stable for the same configuration", func() {
		Expect(digest(base())).To(Equal(digest(base())))
	})

	DescribeTable("changes whenever an executed definition changes",
		func(mutate func(app *types.OSConfig)) {
			changed := base()
			mutate(&changed[2].Linux)
			mutate(&changed[2].MacOS)
			Expect(digest(changed)).NotTo(Equal(digest(base())))
		},
		Entry("a post-install hook is added", func(c *types.OSConfig) { c.PostInstall = []types.InstallCommand{{Shell: "curl example.invalid | sh"}} }),
		Entry("the package command changes", func(c *types.OSConfig) { c.InstallCommand = "app-nightly" }),
		Entry("a dependency is removed", func(c *types.OSConfig) { c.Dependencies = []string{"base"} }),
		Entry("the dependency order changes", func(c *types.OSConfig) { c.Dependencies = []string{"lib", "base"} }),
	)

	It("changes when a dependency's own definition changes", func() {
		changed := base()
		changed[0].Linux.InstallCommand = "base-other"
		changed[0].MacOS.InstallCommand = "base-other"
		Expect(digest(changed)).NotTo(Equal(digest(base())))
	})
})

var _ = Describe("Catalog name checks", func() {
	settings := config.CrossPlatformSettings{ApplicationOverrides: []types.CrossPlatformApp{
		{Name: "winonly", Windows: types.OSConfig{InstallMethod: "winget", InstallCommand: "Vendor.WinOnly"}},
		{Name: "linuxonly", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "linux-pkg"}},
	}}

	It("accepts names and install commands defined for any platform", func() {
		Expect(agentapi.CheckCatalogNames(settings, []string{"WinOnly", "Vendor.WinOnly", "linuxonly", "linux-pkg"})).To(Succeed())
	})

	It("names every unknown application", func() {
		err := agentapi.CheckCatalogNames(settings, []string{"winonly", "nope", "missing"})
		Expect(err).To(MatchError(ContainSubstring("nope, missing")))
	})
})
