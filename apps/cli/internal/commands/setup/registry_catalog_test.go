package setup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Registry setup selection", func() {
	It("offers supported recommended registry applications through the shared snapshot", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"items":[{"name":"registry-tool","default":true,"platforms":{"linux":{"installMethod":"apt","installCommand":"registry-tool"},"macos":null,"windows":null}}],"pagination":{"page":1,"hasNext":false}}`))
		}))
		defer server.Close()
		viper.Reset()
		defer viper.Reset()
		home := GinkgoT().TempDir()
		for _, key := range []string{"DEVEX_DATA_DIR", "DEVEX_CONFIG_DIR", "DEVEX_CACHE_DIR"} {
			GinkgoT().Setenv(key, filepath.Join(home, key))
		}
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)
		settings, err := config.LoadCrossPlatformSettings(home)
		Expect(err).NotTo(HaveOccurred())
		loaded, err := config.LoadCatalogSettings(context.Background(), home)
		Expect(err).NotTo(HaveOccurred())
		settings.Catalog = loaded.Catalog
		model := NewSetupModel(nil, settings, platform.DetectionResult{OS: "linux"})
		Expect(model.getDefaultApps()).To(ContainElement(HaveField("Name", "registry-tool")))
	})
})
