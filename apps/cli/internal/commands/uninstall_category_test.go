package commands_test

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Uninstall category selection", func() {
	var (
		repo      *mocks.MockRepository
		settings  config.CrossPlatformSettings
		installer *recordingInstaller
		target    types.CrossPlatformApp
		other     types.CrossPlatformApp
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		repo = mocks.NewMockRepository()
		settings = config.CrossPlatformSettings{}
		installer = &recordingInstaller{}
		useRecordingInstaller(installer)
		target = types.CrossPlatformApp{
			Name: "development-app", Category: "Development Tools",
			AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "development-package"},
		}
		other = types.CrossPlatformApp{
			Name: "other-app", Category: "Utilities",
			AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "other-package"},
		}
	})

	DescribeTable("resolves name-only records before matching categories case-insensitively",
		func(source, category string) {
			// AddApp mirrors SQLite's name-only rows, unlike SaveApp's full-config mock.
			Expect(repo.AddApp(target.Name)).To(Succeed())
			Expect(repo.AddApp(other.Name)).To(Succeed())
			untracked := target
			untracked.Name = "untracked-app"
			settings.Terminal.Utilities = []types.CrossPlatformApp{untracked}
			switch source {
			case "saved":
				for _, definition := range []types.CrossPlatformApp{target, other} {
					data, err := json.Marshal(definition)
					Expect(err).NotTo(HaveOccurred())
					Expect(repo.Set("installed-app-definition:"+definition.Name, string(data))).To(Succeed())
				}
				// The saved category takes precedence over current configuration.
				target.Category = "Utilities"
				settings.Terminal.Utilities = append(settings.Terminal.Utilities, target)
			case "runtime":
				viper.Set("applications", []any{map[string]any{
					"name": target.Name, "category": target.Category,
					"all_platforms": map[string]any{"install_method": "apt", "install_command": "development-package"},
				}})
				settings.Terminal.Utilities = append(settings.Terminal.Utilities, other)
			case "catalog":
				settings.Terminal.Utilities = append(settings.Terminal.Utilities, target, other)
			}
			tracked, err := repo.ListApps()
			Expect(err).NotTo(HaveOccurred())
			Expect(tracked).To(HaveLen(2))
			for _, app := range tracked {
				Expect(app.Category).To(BeEmpty())
				Expect(app.InstallMethod).To(BeEmpty())
			}
			cmd := commands.NewUninstallCmd(repo, settings)
			cmd.SetArgs([]string{"--category", category, "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(Equal([]string{"development-package"}))
			_, err = repo.GetApp(target.Name)
			Expect(err).To(HaveOccurred())
			_, err = repo.GetApp(other.Name)
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("from saved definitions", "saved", "development tools"),
		Entry("from runtime configuration", "runtime", "DEVELOPMENT TOOLS"),
		Entry("from the catalog", "catalog", "Development Tools"),
	)

	DescribeTable("limits resolution failures to the requested category",
		func(knownCategory string, shouldFail bool) {
			Expect(repo.AddApp("broken-app")).To(Succeed())
			Expect(repo.Set("installed-app-definition:broken-app", "not-json")).To(Succeed())
			settings.Terminal.Utilities = []types.CrossPlatformApp{{Name: "broken-app", Category: knownCategory}}
			cmd := commands.NewUninstallCmd(repo, settings)
			cmd.SetArgs([]string{"--category", "development tools", "--force"})

			err := cmd.Execute()
			if shouldFail {
				Expect(err).To(MatchError(ContainSubstring("invalid installed definition for broken-app")))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(installer.uninstallCommands).To(BeEmpty())
			_, err = repo.GetApp("broken-app")
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("reports failures for matching current metadata", "Development Tools", true),
		Entry("ignores failures for other categories", "Utilities", false),
	)

	DescribeTable("warns about unresolved apps with unknown categories without failing",
		func(hasMatch bool) {
			Expect(repo.AddApp("legacy-unknown-category")).To(Succeed())
			if hasMatch {
				Expect(repo.AddApp(target.Name)).To(Succeed())
				settings.Terminal.Utilities = []types.CrossPlatformApp{target}
			}
			cmd := commands.NewUninstallCmd(repo, settings)
			cmd.SetArgs([]string{"--category", "development tools", "--force"})
			output, err := os.CreateTemp(GinkgoT().TempDir(), "category-output-*")
			Expect(err).NotTo(HaveOccurred())
			originalStdout := os.Stdout
			DeferCleanup(func() {
				os.Stdout = originalStdout
				Expect(output.Close()).To(Succeed())
			})
			os.Stdout = output

			Expect(cmd.Execute()).To(Succeed())
			os.Stdout = originalStdout
			data, err := os.ReadFile(output.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("Skipping legacy-unknown-category: category is unknown"))
			Expect(string(data)).To(ContainSubstring("select it with --app"))
			if hasMatch {
				Expect(installer.uninstallCommands).To(Equal([]string{"development-package"}))
			} else {
				Expect(installer.uninstallCommands).To(BeEmpty())
				Expect(string(data)).To(ContainSubstring("No installed applications found in category"))
			}
			_, err = repo.GetApp("legacy-unknown-category")
			Expect(err).NotTo(HaveOccurred())
		},
		Entry("when another app matches", true),
		Entry("when no apps match", false),
	)
})
