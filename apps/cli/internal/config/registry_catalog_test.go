package config_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Registry settings integration", func() {
	BeforeEach(func() { viper.Reset(); viper.SetEnvPrefix("DEVEX"); viper.AutomaticEnv() })
	AfterEach(func() { viper.Reset() })
	It("keeps bundled definitions online, merges local definitions over them and preserves custom applications", func() {
		fixture, err := os.ReadFile("../catalog/testdata/applications.json")
		Expect(err).NotTo(HaveOccurred())
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(fixture) }))
		defer server.Close()
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)
		GinkgoT().Setenv("DEVEX_OFFLINE", "false")
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("DEVEX_CACHE_DIR", filepath.Join(home, "cache"))
		GinkgoT().Setenv("DEVEX_DATA_DIR", filepath.Join(home, "data"))
		GinkgoT().Setenv("DEVEX_CONFIG_DIR", filepath.Join(home, "config"))
		var dir string
		// GetUserConfigDir identifies the canonical override root.
		settings := config.CrossPlatformSettings{HomeDir: home}
		dir = filepath.Join(settings.GetUserConfigDir(), "applications", "development")
		Expect(os.MkdirAll(dir, 0700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "git.yaml"), []byte("name: GIT\ndescription: local override\nlinux:\n  install_method: apt\n  install_command: custom-git\n"), 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "custom.yaml"), []byte("name: custom\nall_platforms:\n  install_method: mise\n  install_command: custom\n"), 0600)).To(Succeed())
		loaded, err := config.LoadCatalogSettings(context.Background(), home)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Catalog.Source).To(Equal("registry"))
		Expect(loaded.GetAllApps()).To(ContainElement(SatisfyAll(HaveField("Name", "GIT"), HaveField("Linux.InstallCommand", "custom-git"))))
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "custom")))
		// The registry fixture's apps are all bundled, so the bundled
		// definitions are kept rather than replaced by the lossy registry ones.
		Expect(loaded.GetAllApps()).To(ContainElement(SatisfyAll(HaveField("Name", "Docker"), HaveField("Linux.InstallMethod", Not(BeEmpty())))))
		Expect(loaded.GetAllApps()).To(ContainElement(SatisfyAll(HaveField("Name", "Eza"), HaveField("Linux.InstallMethod", "apt"))))
	})
	It("provides embedded definitions on a fresh offline home without network", func() {
		GinkgoT().Setenv("DEVEX_OFFLINE", "true")
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", "http://127.0.0.1:1")
		home := GinkgoT().TempDir()
		for _, key := range []string{"DEVEX_DATA_DIR", "DEVEX_CONFIG_DIR", "DEVEX_CACHE_DIR"} {
			GinkgoT().Setenv(key, filepath.Join(home, key))
		}
		loaded, err := config.LoadCatalogSettings(context.Background(), home)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.Catalog.Source).To(Equal("bundled"))
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "git")))
	})
})
