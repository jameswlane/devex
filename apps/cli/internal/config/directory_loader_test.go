package config_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

var _ = Describe("Directory application loader", func() {
	var homeDir string

	BeforeEach(func() {
		homeDir = GinkgoT().TempDir()
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "test")
	})

	AfterEach(func() {
		log.InitTestLogger()
	})

	It("loads nested applications and applies tier overrides by name", func() {
		defaultDir := filepath.Join(homeDir, ".local/share/devex/config")
		userDir := filepath.Join(homeDir, ".devex/config")
		writeConfigFile(filepath.Join(defaultDir, "applications/development/git.yaml"), `
name: git
description: Default Git
category: Development Tools
linux:
  install_method: apt
  install_command: git
`)
		writeConfigFile(filepath.Join(defaultDir, "applications/databases/postgresql.yaml"), `
name: postgresql
description: PostgreSQL database
category: Databases
linux:
  install_method: apt
  install_command: postgresql
`)
		writeConfigFile(filepath.Join(userDir, "applications/development/git.yaml"), `
name: git
description: User Git
category: Development Tools
linux:
  install_method: apt
  install_command: git-custom
`)

		settings, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		apps := settings.GetAllApps()
		Expect(apps).To(HaveLen(2))
		Expect(apps).To(ContainElement(SatisfyAll(
			HaveField("Name", "git"),
			HaveField("Description", "User Git"),
			HaveField("Linux.InstallCommand", "git-custom"),
		)))
		Expect(apps).To(ContainElement(HaveField("Name", "postgresql")))
	})

	It("loads the shipped tree without warnings", func() {
		configDir := filepath.Join(homeDir, ".local/share/devex/config")
		for _, name := range []string{"applications", "environments", "system", "desktop"} {
			Expect(os.MkdirAll(configDir, 0750)).To(Succeed())
			Expect(os.CopyFS(filepath.Join(configDir, name), os.DirFS(filepath.Join("../../config", name)))).To(Succeed())
		}
		for _, name := range []string{"dotfiles.yaml", "security.yaml"} {
			data, err := os.ReadFile(filepath.Join("../../config", name))
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(configDir, name), data, 0600)).To(Succeed())
		}
		var output bytes.Buffer
		log.InitDefaultLogger(&output)

		settings, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.GetAllApps()).To(ContainElement(HaveField("Name", "git")))
		Expect(settings.GetAllApps()).To(ContainElement(HaveField("Name", "Go")))
		Expect(settings.Fonts).To(ContainElement(HaveField("Name", "Fira Code")))
		Expect(settings.DesktopEnvironments.GNOME.Themes).NotTo(BeEmpty())
		validator := config.NewConfigValidator(homeDir, true)
		Expect(validator.ValidateDirectoryStructure(configDir)).To(Succeed())
		Expect(validator.GetWarnings()).To(BeEmpty())
		Expect(output.String()).NotTo(ContainSubstring("WARN"))
	})

	It("honors all six tiers even when higher tiers use legacy files", func() {
		dirs := []string{
			".local/share/devex/config",
			".local/share/devex/config/environments/test",
			".devex/team",
			".devex/team/environments/test",
			".devex/config",
			".devex/config/environments/test",
		}
		for index, dir := range dirs {
			name := fmt.Sprintf("tier-%d", index)
			if index%2 == 0 {
				writeConfigFile(filepath.Join(homeDir, dir, "applications/development/git.yaml"), fmt.Sprintf(`
name: git
description: %s
linux:
  install_method: apt
  install_command: git
`, name))
			} else {
				writeConfigFile(filepath.Join(homeDir, dir, "terminal.yaml"), fmt.Sprintf(`
terminal_applications:
  development:
    - name: git
      description: %s
      linux:
        install_method: apt
        install_command: git
`, name))
			}
			settings, err := config.LoadCrossPlatformSettings(homeDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(settings.GetAllApps()).To(ConsistOf(SatisfyAll(HaveField("Name", "git"), HaveField("Description", name))))
		}
	})

	It("loads old init preferences without treating the shell name as an application", func() {
		writeConfigFile(filepath.Join(homeDir, ".local/share/devex/config/environment.yaml"), "shell: zsh\neditor: vim\nlanguages: [go]\n")
		settings, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.Shell).To(BeEmpty())
	})

	It("loads canonical environment applications and preserves their platform configuration", func() {
		writeConfigFile(filepath.Join(homeDir, ".local/share/devex/config/environments/shell/zsh.yaml"), `
name: zsh
description: Z shell
all_platforms:
  install_method: apt
  install_command: zsh
`)
		settings, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.GetAllApps()).To(ContainElement(SatisfyAll(HaveField("Name", "zsh"), HaveField("AllPlatforms.InstallCommand", "zsh"))))
	})

	It("reports malformed nested application YAML with its path", func() {
		path := filepath.Join(homeDir, ".local/share/devex/config/applications/development/broken.yaml")
		writeConfigFile(path, "name: [broken")
		_, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(path))
	})

	It("keeps legacy monolithic applications loading with a deprecation warning", func() {
		configDir := filepath.Join(homeDir, ".local/share/devex/config")
		writeConfigFile(filepath.Join(configDir, "applications.yaml"), `
applications:
  - name: git
    description: Legacy Git
    category: development
    install_method: apt
    install_command: git
`)
		writeConfigFile(filepath.Join(configDir, "terminal.yaml"), `
terminal_applications:
  development:
    - name: docker
      description: Legacy Docker
      category: Development Tools
      linux:
        install_method: apt
        install_command: docker.io
`)
		var output bytes.Buffer
		log.InitDefaultLogger(&output)

		settings, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(settings.GetAllApps()).To(ConsistOf(
			HaveField("Name", "git"),
			HaveField("Name", "docker"),
		))
		Expect(output.String()).To(ContainSubstring("Deprecated configuration file"))
	})
})

func writeConfigFile(path, content string) {
	Expect(os.MkdirAll(filepath.Dir(path), 0750)).To(Succeed())
	Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
}
