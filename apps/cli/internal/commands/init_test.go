package commands_test

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

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

var _ = Describe("Init profiles from lifecycle templates", func() {
	var homeDir string

	BeforeEach(func() {
		homeDir = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", homeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
	})

	readInitConfig := func(settings config.CrossPlatformSettings) commands.InitConfig {
		data, err := os.ReadFile(filepath.Join(settings.GetConfigDir(), "devex.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var written commands.InitConfig
		Expect(yaml.Unmarshal(data, &written)).To(Succeed())
		return written
	}

	DescribeTable("takes the profile's tools and languages from the template it names",
		func(profile string, tools []string, languages []string, category string) {
			settings := config.CrossPlatformSettings{HomeDir: homeDir}
			cmd := commands.NewInitCmd(mocks.NewMockRepository(), settings)
			cmd.SetArgs([]string{"--profile", profile, "--interactive=false"})
			Expect(cmd.Execute()).To(Succeed())

			written := readInitConfig(settings)
			Expect(written.Profile).To(Equal(profile))
			Expect(written.Template).To(Equal(profile))
			Expect(written.Applications.Custom).To(Equal(tools))
			Expect(written.Environment.Languages).To(Equal(languages))
			Expect(written.Applications.Categories).To(Equal([]string{category}))
		},
		Entry("devops", "devops", []string{"kubectl", "Helm", "kind", "Terraform", "OpenTofu", "AWS CLI"}, []string{"go", "python"}, "infrastructure"),
		Entry("data-science", "data-science", []string{"JupyterLab", "MLflow"}, []string{"python", "r"}, "science"),
		Entry("mobile", "mobile", []string{"Node.js", "Java JDK", "Flutter", "Android Studio", "CocoaPods"}, []string{"node", "java"}, "mobile"),
		Entry("game-development", "game-development", []string{"Godot", "Godot .NET"}, []string{}, "gaming"),
	)

	It("keeps static profiles that have no template", func() {
		settings := config.CrossPlatformSettings{HomeDir: homeDir}
		cmd := commands.NewInitCmd(mocks.NewMockRepository(), settings)
		cmd.SetArgs([]string{"--profile", "frontend", "--interactive=false"})
		Expect(cmd.Execute()).To(Succeed())
		written := readInitConfig(settings)
		Expect(written.Template).To(BeEmpty())
		Expect(written.Applications.Custom).To(ContainElement("chrome"))
	})

	It("lists template profiles when the profile is unknown", func() {
		settings := config.CrossPlatformSettings{HomeDir: homeDir}
		cmd := commands.NewInitCmd(mocks.NewMockRepository(), settings)
		cmd.SetArgs([]string{"--profile", "quantum", "--interactive=false"})
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		err := cmd.Execute()
		Expect(err).To(MatchError(And(
			ContainSubstring(`unknown profile "quantum"`),
			ContainSubstring("data-science, devops, game-development, mobile"),
		)))
	})
})
