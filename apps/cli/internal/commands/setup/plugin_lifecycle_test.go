package setup

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/installers"
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
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	})

	It("reuses the plugin stage bootstrap for later stages without initializing again", func() {
		Expect(os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".devex", "plugins"), 0750)).To(Succeed())
		model := &SetupModel{}
		first, err := model.initializePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(first).NotTo(BeNil())
		// A new bootstrap cannot be created under this HOME. Later stages must reuse the first.
		blockedHome := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(blockedHome, nil, 0600)).To(Succeed())
		GinkgoT().Setenv("HOME", blockedHome)
		second, err := model.initializePlugins(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(BeIdenticalTo(first))
	})

	It("does not retry a failed initialization during later stages", func() {
		blockedHome := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(blockedHome, nil, 0600)).To(Succeed())
		GinkgoT().Setenv("HOME", blockedHome)
		model := &SetupModel{}
		_, firstErr := model.initializePlugins(context.Background())
		Expect(firstErr).To(HaveOccurred())
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		_, secondErr := model.initializePlugins(context.Background())
		Expect(secondErr).To(BeIdenticalTo(firstErr))
	})
})
