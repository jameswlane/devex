package commands

import (
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/catalog"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var _ = Describe("Catalog opt-ins", func() {
	It("inherits a group's opt-in without inheriting the root setup opt-in", func() {
		root := &cobra.Command{Use: "root", Annotations: map[string]string{catalogAnnotation: catalogRequired}}
		unrelated := &cobra.Command{Use: "install"}
		group := &cobra.Command{Use: "renamed-group", Annotations: map[string]string{catalogAnnotation: catalogRequired}}
		child := &cobra.Command{Use: "child"}
		root.AddCommand(unrelated, group)
		group.AddCommand(child)
		Expect(requiresCatalog(root)).To(BeTrue())
		Expect(requiresCatalog(unrelated)).To(BeFalse())
		Expect(requiresCatalog(group)).To(BeTrue())
		Expect(requiresCatalog(child)).To(BeTrue())
	})

	It("plans a template's catalog applications on a fresh home before devex init (#466)", func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, key := range []string{"DEVEX_DATA_DIR", "DEVEX_CONFIG_DIR", "DEVEX_CACHE_DIR", "DEVEX_STATE_DIR"} {
			GinkgoT().Setenv(key, filepath.Join(home, key))
		}
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", "http://127.0.0.1:1")
		settings, err := config.LoadCrossPlatformSettings(home)
		Expect(err).NotTo(HaveOccurred())
		settings.Catalog = &catalog.Result{}
		root := NewRootCmd("test", mocks.NewMockRepository(), settings)
		root.SetArgs([]string{"template", "plan", "backend", "--action", "install", "--non-interactive", "--offline", "--dry-run"})
		Expect(root.Execute()).To(Succeed())
		Expect(settings.Catalog.Source).To(Equal("bundled"))
	})
})
