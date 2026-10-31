package commands

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/recovery"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/apps/cli/internal/version"
)

var _ = Describe("Configuration tree operations", func() {
	var settings config.CrossPlatformSettings
	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")
		settings = config.CrossPlatformSettings{HomeDir: home}
	})
	It("loads init, dry-run remove, remove, and add without recreating legacy files", func() {
		init := NewInitCmd(mocks.NewMockRepository(), settings)
		init.SetArgs([]string{"--profile", "backend", "--interactive=false"})
		Expect(init.Execute()).To(Succeed())
		loaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		original := filepath.Join(settings.GetConfigDir(), "applications/development/git.yaml")
		before, err := os.ReadFile(original)
		Expect(err).NotTo(HaveOccurred())
		Expect(removeSpecificApp("GIT", loaded, false, true, false, true)).To(Succeed())
		Expect(os.ReadFile(original)).To(Equal(before))
		Expect(addSpecificApp("git", loaded, true)).To(MatchError(ContainSubstring("already in your configuration")))
		Expect(NewRemoveModel(loaded).list.Items()).To(ContainElement(WithTransform(func(item interface{}) string { return item.(RemoveAppItem).app.Name }, Equal("git"))))
		Expect(removeSpecificApp("git", loaded, true, true, false, false)).To(Succeed())
		Expect(original).NotTo(BeAnExistingFile())
		removed, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(removed.GetAllApps()).NotTo(ContainElement(HaveField("Name", "git")))
		Expect(addSpecificApp("git", loaded, false)).To(Succeed())
		reloaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(reloaded.GetAllApps()).To(ContainElement(SatisfyAll(HaveField("Name", "git"), HaveField("Linux.InstallCommand", "git"))))
		Expect(filepath.Join(settings.GetConfigDir(), "applications.yaml")).NotTo(BeAnExistingFile())
	})
	It("preserves cross-platform definitions when adding an application", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{{Name: "portable", Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "portable"}, MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "portable"}}}
		Expect(addSpecificApp("portable", settings, false)).To(Succeed())
		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ConsistOf(SatisfyAll(HaveField("Linux.InstallMethod", "apt"), HaveField("MacOS.InstallMethod", "brew"))))
	})
	DescribeTable("rejects add filename collisions without changing the existing application", func(existingName string) {
		path := filepath.Join(settings.GetConfigDir(), "applications/development/node-js.yaml")
		original := types.CrossPlatformApp{Name: existingName, Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "original"}}
		Expect(writeYAMLConfig(original, path, "# Preserve this definition\n")).To(Succeed())
		before, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		settings.ApplicationOverrides = []types.CrossPlatformApp{{Name: "node-js", Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "replacement"}}}

		Expect(addSpecificApp("node-js", settings, false)).To(MatchError(And(ContainSubstring("already exists"), ContainSubstring(path), ContainSubstring("rename the existing file"))))
		Expect(os.ReadFile(path)).To(Equal(before))
		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ConsistOf(HaveField("Name", existingName)))
	}, Entry("simplified names collide", "Node.js"), Entry("filename differs from the application name", "unrelated"))

	It("retains both legacy applications when their simplified filenames collide and supports retry", func() {
		dir := settings.GetConfigDir()
		legacy := filepath.Join(dir, "applications.yaml")
		definitions := []types.CrossPlatformApp{
			{Name: "Node.js", Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "node-original"}},
			{Name: "node-js", Category: "development", MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "node-other"}},
		}
		Expect(writeYAMLConfig(map[string]interface{}{"applications": definitions}, legacy, "")).To(Succeed())
		before, err := os.ReadFile(legacy)
		Expect(err).NotTo(HaveOccurred())

		Expect(migrateApplicationConfig(dir)).To(MatchError(And(ContainSubstring("legacy file retained"), ContainSubstring("already exists"))))
		Expect(os.ReadFile(legacy)).To(Equal(before))
		path := filepath.Join(dir, "applications/development/node-js.yaml")
		written, err := config.LoadApplicationConfigs(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(written).To(ConsistOf(definitions[0], definitions[1]))
		canonical, err := (config.FileApplicationLoader{}).LoadApplications([]string{dir})
		Expect(err).NotTo(HaveOccurred())
		Expect(canonical).To(ConsistOf(definitions[0]))

		Expect(os.Rename(path, filepath.Join(filepath.Dir(path), "node-original.yaml"))).To(Succeed())
		Expect(migrateApplicationConfig(dir)).To(Succeed())
		Expect(legacy).NotTo(BeAnExistingFile())
		Expect(config.LoadApplicationConfigs(dir)).To(ConsistOf(definitions[0], definitions[1]))
	})

	It("retains legacy and existing files when migration collides with a differently named application", func() {
		dir := settings.GetConfigDir()
		legacy := filepath.Join(dir, "applications.yaml")
		Expect(writeYAMLConfig(map[string]interface{}{"applications": []types.CrossPlatformApp{{Name: "node-js", Category: "development"}}}, legacy, "")).To(Succeed())
		path := filepath.Join(dir, "applications/development/node-js.yaml")
		Expect(writeYAMLConfig(types.CrossPlatformApp{Name: "unrelated", Category: "development"}, path, "")).To(Succeed())
		before, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())

		Expect(migrateApplicationConfig(dir)).To(MatchError(ContainSubstring("already exists")))
		Expect(legacy).To(BeAnExistingFile())
		Expect(os.ReadFile(path)).To(Equal(before))
		Expect(config.LoadApplicationConfigs(dir)).To(ConsistOf(HaveField("Name", "node-js"), HaveField("Name", "unrelated")))
	})

	It("keeps the legacy file after a partial migration failure and completes on retry", func() {
		dir := settings.GetConfigDir()
		legacy := filepath.Join(dir, "applications.yaml")
		definitions := []types.CrossPlatformApp{
			{Name: "first", Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "first"}},
			{Name: "second", Category: "blocked", MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "second"}},
		}
		Expect(writeYAMLConfig(map[string]interface{}{"applications": definitions}, legacy, "")).To(Succeed())
		before, err := os.ReadFile(legacy)
		Expect(err).NotTo(HaveOccurred())
		// A file in place of the second category fails even when tests run as root.
		blocked := filepath.Join(dir, "applications/blocked")
		Expect(os.MkdirAll(filepath.Dir(blocked), 0750)).To(Succeed())
		Expect(os.WriteFile(blocked, []byte("obstruction"), 0600)).To(Succeed())

		Expect(migrateApplicationConfig(dir)).To(MatchError(And(ContainSubstring("legacy file retained"), ContainSubstring("second"), ContainSubstring("create configuration directory"))))
		Expect(os.ReadFile(legacy)).To(Equal(before))
		first := filepath.Join(dir, "applications/development/first.yaml")
		Expect(first).To(BeAnExistingFile())
		firstBefore, err := os.ReadFile(first)
		Expect(err).NotTo(HaveOccurred())
		Expect(config.LoadApplicationConfigs(dir)).To(ConsistOf(definitions[0], definitions[1]))

		Expect(os.Remove(blocked)).To(Succeed())
		Expect(migrateApplicationConfig(dir)).To(Succeed())
		Expect(legacy).NotTo(BeAnExistingFile())
		Expect(os.ReadFile(first)).To(Equal(firstBefore))
		Expect(config.LoadApplicationConfigs(dir)).To(ConsistOf(definitions[0], definitions[1]))
	})
	It("resets recovery configuration to a loadable tree", func() {
		base := filepath.Join(settings.HomeDir, ".devex")
		dir := filepath.Join(base, "config")
		Expect(os.MkdirAll(dir, 0750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "applications.yaml"), []byte("broken: ["), 0600)).To(Succeed())
		manager := recovery.NewRecoveryManager(base)
		result, err := manager.ExecuteRecovery(recovery.RecoveryOption{Steps: []recovery.RecoveryStep{{ID: "config-reset"}}}, recovery.RecoveryContext{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(filepath.Join(dir, "applications/development/git.yaml")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, "applications.yaml")).NotTo(BeAnExistingFile())
		loaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "git")))
	})
	It("includes canonical files in custom templates", func() {
		base := filepath.Join(settings.HomeDir, ".devex")
		dir := filepath.Join(base, "config")
		Expect(initializeConfigTree(dir, true)).To(Succeed())
		manager, err := templates.NewCustomTemplateManager(base, dir, version.NewVersionManager(base), backup.NewBackupManager(base), undo.NewUndoManager(base))
		Expect(err).NotTo(HaveOccurred())
		manifest := &templates.CustomTemplateManifest{ID: "tree", Name: "Tree", Version: "1.0.0"}
		Expect(manager.CreateTemplate(manifest, dir)).To(Succeed())
		Expect(manifest.Files).To(ContainElements(filepath.Join("applications", "development", "git.yaml"), filepath.Join("system", "git.yaml")))
		Expect(filepath.Join(manifest.Source.Path, "applications/development/git.yaml")).To(BeAnExistingFile())
	})

	It("migrates legacy application definitions on mutation and removes every matching tree file", func() {
		dir := settings.GetConfigDir()
		Expect(os.MkdirAll(dir, 0750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "applications.yaml"), []byte("applications:\n  - name: legacy\n    category: development\n    all_platforms:\n      install_method: apt\n      install_command: legacy\n"), 0600)).To(Succeed())
		Expect(writeYAMLConfig(types.CrossPlatformApp{Name: "retained", Category: "other", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "retained"}}, filepath.Join(dir, "applications/custom/named-differently.yaml"), "")).To(Succeed())
		Expect(removeSpecificApp("legacy", settings, false, true, false, false)).To(Succeed())
		Expect(filepath.Join(dir, "applications.yaml")).NotTo(BeAnExistingFile())
		apps, err := config.LoadApplicationConfigs(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(ContainElement(HaveField("Name", "retained")))
		Expect(removeSpecificApp("retained", settings, false, true, false, false)).To(Succeed())
		Expect(config.LoadApplicationConfigs(dir)).To(BeEmpty())
	})
	It("creates and copies loadable team and environment trees", func() {
		Expect(initTeamConfig(settings, false, false)).To(Succeed())
		teamDir := settings.GetTeamConfigDir()
		Expect(filepath.Join(teamDir, "applications/development/git.yaml")).To(BeAnExistingFile())
		Expect(filepath.Join(teamDir, "applications.yaml")).NotTo(BeAnExistingFile())
		loaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "git")))
		Expect(initializeConfigTree(settings.GetUserConfigDir(), true)).To(Succeed())
		Expect(initTeamConfig(settings, true, true)).To(Succeed())
		Expect(initEnvironmentConfig(settings, "staging", "user", "", false)).To(Succeed())
		source := filepath.Join(settings.GetUserConfigDir(), "environments/staging")
		Expect(initializeConfigTree(source, true)).To(Succeed())
		Expect(initEnvironmentConfig(settings, "prod", "user", "staging", false)).To(Succeed())
		target := filepath.Join(settings.GetUserConfigDir(), "environments/prod")
		Expect(filepath.Join(target, "applications/development/git.yaml")).To(BeAnExistingFile())
		GinkgoT().Setenv("DEVEX_ENV", "prod")
		Expect(showTeamConfigStatus(settings)).To(Succeed())
		Expect(showEnvironmentStatus(settings)).To(Succeed())
		loaded, err = config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "git")))
		for _, dir := range []string{teamDir, source, target} {
			for _, name := range []string{"applications.yaml", "environment.yaml", "system.yaml"} {
				Expect(filepath.Join(dir, name)).NotTo(BeAnExistingFile())
			}
		}
	})
	It("copies legacy team sources into the tree and retains preferences in metadata", func() {
		source := settings.GetUserConfigDir()
		Expect(os.MkdirAll(source, 0750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(source, "applications.yaml"), []byte("applications:\n  development:\n    - name: old-tool\n      category: development\n      install_method: apt\n      install_command: old-tool\n"), 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(source, "environment.yaml"), []byte("shell: zsh\neditor: vim\n"), 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(source, "system.yaml"), []byte("git: []\n"), 0600)).To(Succeed())
		Expect(initTeamConfig(settings, true, false)).To(Succeed())
		target := settings.GetTeamConfigDir()
		Expect(filepath.Join(target, "applications/development/old-tool.yaml")).To(BeAnExistingFile())
		for _, name := range []string{"applications.yaml", "environment.yaml", "system.yaml"} {
			Expect(filepath.Join(target, name)).NotTo(BeAnExistingFile())
		}
		metadata, err := os.ReadFile(filepath.Join(target, "devex.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(metadata)).To(ContainSubstring("shell: zsh"))
		output := filepath.Join(GinkgoT().TempDir(), "legacy.yaml")
		Expect(exportConfiguration(context.Background(), settings, "yaml", output, nil, nil, false, false)).To(Succeed())
		imported := config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir()}
		Expect(importConfiguration(context.Background(), imported, output, true, false, false, true, false)).To(Succeed())
		Expect(filepath.Join(imported.GetUserConfigDir(), "applications/development/old-tool.yaml")).To(BeAnExistingFile())
		Expect(filepath.Join(imported.GetUserConfigDir(), "environment.yaml")).NotTo(BeAnExistingFile())
	})

	DescribeTable("round trips tree files through export and import", func(bundle bool) {
		Expect(initializeConfigTree(settings.GetUserConfigDir(), true)).To(Succeed())
		output := filepath.Join(GinkgoT().TempDir(), "export.yaml")
		if bundle {
			output += ".tar.gz"
		}
		Expect(exportConfiguration(context.Background(), settings, "yaml", output, nil, nil, bundle, true)).To(Succeed())
		var data map[string]interface{}
		var err error
		if bundle {
			data, err = importFromBundle(context.Background(), output)
		} else {
			content, readErr := os.ReadFile(output)
			Expect(readErr).NotTo(HaveOccurred())
			err = yaml.Unmarshal(content, &data)
		}
		Expect(err).NotTo(HaveOccurred())
		target := config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir()}
		Expect(applyImportedConfigurations(data, target, true, false)).To(Succeed())
		Expect(filepath.Join(target.GetUserConfigDir(), "applications/development/git.yaml")).To(BeAnExistingFile())
		loaded, err := config.LoadCrossPlatformSettings(target.HomeDir)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded.GetAllApps()).To(ContainElement(HaveField("Name", "git")))
	}, Entry("YAML", false), Entry("bundle", true))
	It("propagates cancellation while comparing tree files", func() {
		Expect(initializeConfigTree(settings.GetConfigDir(), true)).To(Succeed())
		backup := filepath.Join(GinkgoT().TempDir(), "backup.yaml")
		Expect(os.WriteFile(backup, []byte("{}"), 0600)).To(Succeed())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(compareConfigurations(ctx, settings, backup, "diff")).To(MatchError(ContainSubstring("context canceled")))
	})

	It("rejects import paths outside the configuration tree", func() {
		for _, key := range []string{"../escape", "/absolute", "applications/../../escape", "unknown/file"} {
			data := map[string]interface{}{"configurations": map[string]interface{}{"user": map[string]interface{}{key: map[string]interface{}{"name": "bad"}}}}
			Expect(applyImportedConfigurations(data, settings, false, false)).NotTo(Succeed())
		}
	})
	DescribeTable("matches category labels and short slugs without case sensitivity", func(filter string) {
		apps := []AvailableApp{{Name: "editor", Category: "Development Tools"}, {Name: "db", Category: "Databases"}}
		Expect(filterAvailableApps(apps, ListCommandOptions{Category: filter})).To(ConsistOf(apps[0]))
	}, Entry("label", "DEVELOPMENT TOOLS"), Entry("slug", "Development-Tools"), Entry("short slug", "development"))
})
