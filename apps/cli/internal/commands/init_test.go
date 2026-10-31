package commands_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Init command configuration layout", func() {
	var homeDir string

	BeforeEach(func() {
		homeDir = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", homeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
	})

	It("writes the shipped tree and loads it immediately", func() {
		settings := config.CrossPlatformSettings{HomeDir: homeDir}
		cmd := commands.NewInitCmd(mocks.NewMockRepository(), settings)
		cmd.SetArgs([]string{"--profile", "backend", "--interactive=false"})
		Expect(cmd.Execute()).To(Succeed())

		configDir := settings.GetConfigDir()
		Expect(filepath.Join(configDir, "applications", "development", "git.yaml")).To(BeAnExistingFile())
		Expect(filepath.Join(configDir, "environments", "programming-languages", "go.yaml")).To(BeAnExistingFile())
		Expect(filepath.Join(configDir, "applications.yaml")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(configDir, "environment.yaml")).NotTo(BeAnExistingFile())

		loaded, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "git")))
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "PostgreSQL")))

		validate := commands.NewInitCmd(mocks.NewMockRepository(), loaded)
		validate.SetArgs([]string{"--validate"})
		Expect(validate.Execute()).To(Succeed())
	})

	It("imports applications into the canonical tree", func() {
		settings := config.CrossPlatformSettings{HomeDir: homeDir}
		importPath := filepath.Join(homeDir, "import.yaml")
		content := `applications:
  - name: imported-tool
    description: Imported tool
    category: development
    install_method: apt
    install_command: imported-tool
environment:
  programming_languages: []
  fonts: []
  shell: []
system:
  git:
    - aliases:
        st: status
`
		Expect(os.WriteFile(importPath, []byte(content), 0600)).To(Succeed())
		cmd := commands.NewInitCmd(mocks.NewMockRepository(), settings)
		cmd.SetArgs([]string{"--import", importPath})
		Expect(cmd.Execute()).To(Succeed())

		configDir := settings.GetConfigDir()
		Expect(filepath.Join(configDir, "applications", "development", "imported-tool.yaml")).To(BeAnExistingFile())
		Expect(filepath.Join(configDir, "applications.yaml")).NotTo(BeAnExistingFile())
		loaded, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "imported-tool")))
		Expect(loaded.Dotfiles.Git).To(HaveLen(1))
		Expect(loaded.Dotfiles.Git[0].Aliases).To(HaveKeyWithValue("st", "status"))
		validate := commands.NewConfigCmd(mocks.NewMockRepository(), loaded)
		validate.SetArgs([]string{"validate"})
		Expect(validate.Execute()).To(Succeed())
	})
})
