package config_test

import (
	"github.com/jameswlane/devex/apps/cli/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Legacy download verification metadata", func() {
	It("preserves expected digests when resolving a legacy application", func() {
		previous := viper.Get("apps")
		DeferCleanup(func() { viper.Set("apps", previous) })
		viper.Set("apps", []any{map[string]any{"name": "tool", "description": "tool", "install_method": "curlpipe", "install_command": "https://example.test/install.sh", "verification": map[string]any{"policy": "required", "sha256": "expected"}}})
		app, err := config.GetAppInfo("tool")
		Expect(err).NotTo(HaveOccurred())
		Expect(app.Verification.Policy).To(Equal("required"))
		Expect(app.Verification.SHA256).To(Equal("expected"))
		converted := config.CrossPlatformAppFromLegacy(*app)
		Expect(converted.AllPlatforms.Verification).To(Equal(app.Verification))
	})
})
