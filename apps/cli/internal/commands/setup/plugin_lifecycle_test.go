package setup

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Setup plugin lifecycle", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(func() { installers.InitializeWithPluginBootstrap(nil) })
		viper.Set("offline", true)
		testhelper.SetProfileEnv(GinkgoT(), GinkgoT().TempDir())
		// Keep the isolated profile env authoritative even on elevated runners
		// (such as GitHub Actions Windows hosts or sudo) so ResolveUser does
		// not fall back to the real host account's HomeDir.
		originalResolver := sdk.NewPrivilegeResolver
		resolver := originalResolver()
		resolver.Elevated = false
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
		DeferCleanup(func() { sdk.NewPrivilegeResolver = originalResolver })
	})

	It("reuses the plugin stage bootstrap for later stages without initializing again", func() {
		// Use the resolved plugin directory: on Windows it lives under
		// %LOCALAPPDATA%\DevEx rather than the legacy ~/.devex location.
		Expect(os.MkdirAll(devexpaths.ForHome(os.Getenv("HOME")).Plugins(), 0750)).To(Succeed())
		model := &SetupModel{}
		first, err := model.initializePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(first).NotTo(BeNil())
		// A new bootstrap cannot be created under this HOME. Later stages must reuse the first.
		blockedHome := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(blockedHome, nil, 0600)).To(Succeed())
		testhelper.SetProfileEnv(GinkgoT(), blockedHome)
		second, err := model.initializePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeIdenticalTo(first))
	})

	It("does not retry a failed initialization during later stages", func() {
		blockedHome := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(blockedHome, nil, 0600)).To(Succeed())
		testhelper.SetProfileEnv(GinkgoT(), blockedHome)
		model := &SetupModel{}
		_, firstErr := model.initializePlugins(context.Background())
		Expect(firstErr).To(HaveOccurred())
		testhelper.SetProfileEnv(GinkgoT(), GinkgoT().TempDir())
		_, secondErr := model.initializePlugins(context.Background())
		Expect(secondErr).To(BeIdenticalTo(firstErr))
	})
})
