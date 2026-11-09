package installers

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Artifact verification dispatch", func() {
	It("preserves YAML integrity metadata through OS conversion and plugin argv", func() {
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal([]byte(`name: tool
linux:
  install_method: deb
  download_url: https://example.test/tool.deb
  verification:
    policy: required
    sha256: pinned-digest
    signature_url: https://example.test/tool.sig
    public_key: /trusted/key.asc
    fingerprint: trusted-fingerprint
`), &app)).To(Succeed())
		config := app.ToLegacyAppConfig()
		Expect(config.Verification).To(Equal(app.Linux.Verification))
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "deb", pluginBootstrap: executor}
		Expect(installer.InstallArtifact(context.Background(), config)).To(Succeed())
		expected := append([]string{"install"}, config.Verification.Arguments()...)
		expected = append(expected, "https://example.test/tool.deb")
		Expect(executor.args).To(Equal(expected))
	})
	It("keeps AppImage location flags after its positional arguments", func() {
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "appimage", pluginBootstrap: executor}
		Expect(installer.InstallArtifact(context.Background(), types.AppConfig{InstallMethod: "appimage", InstallCommand: "https://example.test/tool.AppImage tool --cli", Verification: sdk.ArtifactVerification{SHA256: "digest"}})).To(Succeed())
		Expect(executor.args).To(Equal([]string{"install", "--sha256", "digest", "https://example.test/tool.AppImage", "tool", "--cli"}))
	})
	It("preserves structured artifact arguments", func() {
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "deb", pluginBootstrap: executor}
		Expect(installer.InstallArtifact(context.Background(), types.AppConfig{InstallMethod: "deb", Packages: []string{"./a file.deb"}, Verification: sdk.ArtifactVerification{Policy: "required", SHA256: "digest"}})).To(Succeed())
		Expect(executor.args).To(Equal([]string{"install", "--verification-policy", "required", "--sha256", "digest", "./a file.deb"}))
	})
})
