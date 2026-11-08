package config_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shared semantic validation", func() {
	var settings config.CrossPlatformSettings
	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		settings = config.CrossPlatformSettings{HomeDir: home}
	})
	write := func(path, content string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0750)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
	}
	It("reports unknown nested and top-level application fields with source locations", func() {
		path := filepath.Join(settings.GetUserConfigDir(), "applications", "tools", "docker.yaml")
		write(path, "name: docker\ndescription: Docker\ndependencies: [git]\nlinux:\n  install_method: apt\n  install_command: docker\n  typo: true\n")
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Errors).To(Equal(2))
		Expect(result.Diagnostics).To(ContainElement(And(HaveField("File", path), HaveField("Field", "dependencies"), HaveField("Message", ContainSubstring("line 3")))))
		result, err = config.ValidateConfig(context.Background(), settings, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue())
		Expect(result.Warnings).To(Equal(2))
	})
	It("checks unknown fields through YAML aliases", func() {
		path := filepath.Join(settings.GetConfigDir(), "applications", "aliased.yaml")
		write(path, "name: aliased\nlinux: &platform\n  install_method: curlpipe\n  install_command: installer\n  typo: true\nwindows: *platform\n")
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Diagnostics).To(ContainElement(And(HaveField("Field", "windows.typo"), HaveField("Severity", config.SeverityError))))
	})

	It("rejects malformed YAML in environment overlays", func() {
		dirs := config.EffectiveConfigDirs(settings)
		path := filepath.Join(dirs[1], "applications", "broken.yaml")
		write(path, "name: [\n")
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(HaveField("File", path)))
	})
	It("checks cycles after layered overrides and retains the effective file location", func() {
		dirs := config.EffectiveConfigDirs(settings)
		a := "name: A\ndescription: A\nlinux:\n  install_method: apt\n  install_command: a\n  dependencies: [B]\n"
		b := "name: B\ndescription: B\nlinux:\n  install_method: apt\n  install_command: b\n  dependencies: [A]\n"
		write(filepath.Join(dirs[0], "applications", "a.yaml"), a)
		write(filepath.Join(dirs[0], "applications", "b.yaml"), b)
		_, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).To(MatchError(ContainSubstring("A -> B -> A")))
		write(filepath.Join(dirs[4], "applications", "b.yaml"), "name: B\ndescription: B\nlinux:\n  install_method: apt\n  install_command: b\n")
		loaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		result, err := config.ValidateConfig(context.Background(), loaded, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "%v", result.Diagnostics)
	})
	It("keeps planned methods and native dependencies as warnings even in strict mode", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{{Name: "terminal", Windows: types.OSConfig{InstallMethod: "winget", InstallCommand: "terminal", Dependencies: []string{"native-package"}}}}
		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue())
		Expect(result.Diagnostics).To(ContainElements(HaveField("Category", config.CategoryDependencyNative), HaveField("Category", config.CategoryInstallMethodPlanned)))
	})

	It("validates every runtime schema in the source config tree strictly", func() {
		validator := config.NewConfigValidator(settings.HomeDir, true)
		source, err := filepath.Abs(filepath.Join("..", "..", "config"))
		Expect(err).NotTo(HaveOccurred())
		Expect(validator.ValidateDirectoryStructure(source)).To(Succeed(), "%v", validator.GetErrors())
		Expect(validator.GetErrors()).To(BeEmpty())
	})

	It("validates the shipped catalog strictly", func() {
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())
		loaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		result, err := config.ValidateConfig(context.Background(), loaded, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "%v", result.Diagnostics)
	})
})
