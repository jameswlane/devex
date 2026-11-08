package commands

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Validation before mutation", func() {
	var settings config.CrossPlatformSettings
	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		settings = config.CrossPlatformSettings{HomeDir: home}
	})
	It("rejects cycles in inline application overrides before selection can execute", func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		settings.ApplicationOverrides = []types.CrossPlatformApp{
			{Name: "A", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "a"}},
			{Name: "B", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "b", Dependencies: []string{"A"}}},
		}
		viper.Set("applications", []map[string]interface{}{{"name": "A", "linux": map[string]interface{}{"install_method": "apt", "install_command": "a", "dependencies": []string{"B"}}}})
		_, _, _, err := configuredApplications(context.Background(), settings)
		Expect(err).To(MatchError(ContainSubstring("A -> B -> A")))
	})

	It("rejects an unsupported added method before creating configuration or undo state", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{{Name: "broken", Linux: types.OSConfig{InstallMethod: "aptt", InstallCommand: "broken"}}}
		Expect(addSpecificApp("broken", settings, false)).To(MatchError(ContainSubstring("unknown install method")))
		entries, err := os.ReadDir(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
	It("rejects cyclic import data before writing destination files even when optional validation is disabled", func() {
		input := filepath.Join(GinkgoT().TempDir(), "import.yaml")
		content := `configurations:
  default:
    applications/tools/a:
      name: A
      linux:
        install_method: apt
        install_command: a
        dependencies: [B]
    applications/tools/b:
      name: B
      linux:
        install_method: apt
        install_command: b
        dependencies: [A]
`
		Expect(os.WriteFile(input, []byte(content), 0600)).To(Succeed())
		err := importConfiguration(context.Background(), settings, input, false, true, true, false, false)
		Expect(err).To(MatchError(ContainSubstring("A -> B -> A")))
		entries, err := os.ReadDir(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
	It("validates disk graph content even when the caller has not loaded settings", func() {
		path := filepath.Join(settings.GetConfigDir(), "applications", "bad.yaml")
		Expect(os.MkdirAll(filepath.Dir(path), 0750)).To(Succeed())
		Expect(os.WriteFile(path, []byte("name: broken\nlinux:\n  install_method: aptt\n  install_command: broken\n"), 0600)).To(Succeed())
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(HaveField("File", path), HaveField("Category", config.CategoryInstallMethodUnknown))))
	})
})
