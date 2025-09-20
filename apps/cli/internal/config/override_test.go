package config_test

import (
	"io"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Config Override Functionality", func() {
	var (
		tempHomeDir       string
		defaultConfigDir  string
		overrideConfigDir string
	)

	BeforeEach(func() {
		// Initialize the logger to discard output during tests
		log.InitDefaultLogger(io.Discard)

		// Create temporary directories for testing
		var err error
		tempHomeDir, err = os.MkdirTemp("", "devex-config-test")
		Expect(err).ToNot(HaveOccurred())

		defaultConfigDir = filepath.Join(tempHomeDir, ".local/share/devex/config")
		overrideConfigDir = filepath.Join(tempHomeDir, ".devex/config")

		err = os.MkdirAll(defaultConfigDir, 0755)
		Expect(err).ToNot(HaveOccurred())

		err = os.MkdirAll(overrideConfigDir, 0755)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		// Clean up temporary directories
		os.RemoveAll(tempHomeDir)
	})

	Context("LoadConfigs Override Functionality", func() {
		It("loads default config when no override exists", func() {
			// Create only default config
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			defaultConfig := `test_setting: "default_value"`

			err := os.WriteFile(defaultConfigPath, []byte(defaultConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Verify default value is loaded
			Expect(viper.GetString("test_setting")).To(Equal("default_value"))
		})

		It("overrides default config with user config", func() {
			// Create default config
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			defaultConfig := `test_setting: "default_value"`

			err := os.WriteFile(defaultConfigPath, []byte(defaultConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override config
			overrideConfigPath := filepath.Join(overrideConfigDir, "test.yaml")
			overrideConfig := `test_setting: "override_value"`

			err = os.WriteFile(overrideConfigPath, []byte(overrideConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Verify override value is used
			Expect(viper.GetString("test_setting")).To(Equal("override_value"))
		})

		It("merges configurations correctly", func() {
			// Create default config with multiple settings
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			defaultConfig := `default_setting: "default_value"
shared_setting: "default_shared"
`

			err := os.WriteFile(defaultConfigPath, []byte(defaultConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override config that only overrides some settings
			overrideConfigPath := filepath.Join(overrideConfigDir, "test.yaml")
			overrideConfig := `shared_setting: "override_shared"
override_only: "override_only_value"
`

			err = os.WriteFile(overrideConfigPath, []byte(overrideConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Verify default values remain for non-overridden settings
			Expect(viper.GetString("default_setting")).To(Equal("default_value"))

			// Verify override values are used for overridden settings
			Expect(viper.GetString("shared_setting")).To(Equal("override_shared"))
			Expect(viper.GetString("override_only")).To(Equal("override_only_value"))
		})

		It("handles missing default config gracefully", func() {
			// Create only override config (no default)
			overrideConfigPath := filepath.Join(overrideConfigDir, "test.yaml")
			overrideConfig := `test_setting: "override_value"`

			err := os.WriteFile(overrideConfigPath, []byte(overrideConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Verify override value is loaded
			Expect(viper.GetString("test_setting")).To(Equal("override_value"))
		})

		It("handles missing override config gracefully", func() {
			// Create only default config (no override)
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			defaultConfig := `test_setting: "default_value"`

			err := os.WriteFile(defaultConfigPath, []byte(defaultConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs (override doesn't exist)
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Verify default value is loaded
			Expect(viper.GetString("test_setting")).To(Equal("default_value"))
		})

		It("loads multiple config files with overrides", func() {
			// Create default configs
			appConfigPath := filepath.Join(defaultConfigDir, "applications.yaml")
			appConfig := `
applications:
  development:
    - name: "default-app"
      category: "Development"
`
			err := os.WriteFile(appConfigPath, []byte(appConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			envConfigPath := filepath.Join(defaultConfigDir, "environment.yaml")
			envConfig := `
environment:
  programming_languages:
    - name: "default-lang"
      category: "Language"
`
			err = os.WriteFile(envConfigPath, []byte(envConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override for only one file
			overrideAppConfigPath := filepath.Join(overrideConfigDir, "applications.yaml")
			overrideAppConfig := `
applications:
  development:
    - name: "override-app"
      category: "Development"
`
			err = os.WriteFile(overrideAppConfigPath, []byte(overrideAppConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs
			viper, err := config.LoadConfigs(tempHomeDir, []string{"applications.yaml", "environment.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Verify applications config is overridden
			apps := viper.Get("applications.development")
			Expect(apps).ToNot(BeNil())

			// Verify environment config uses default (no override)
			langs := viper.Get("environment.programming_languages")
			Expect(langs).ToNot(BeNil())
		})

		It("uses correct directory structure for overrides", func() {
			// Verify the override path is ~/.devex/config/ not ~/.devex/

			// Create default config
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			defaultConfig := `test_setting: "default_value"`

			err := os.WriteFile(defaultConfigPath, []byte(defaultConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override config in the WRONG location (should not be used)
			wrongOverrideDir := filepath.Join(tempHomeDir, ".devex")
			err = os.MkdirAll(wrongOverrideDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			wrongOverridePath := filepath.Join(wrongOverrideDir, "test.yaml")
			wrongOverrideConfig := `test_setting: "wrong_override"`

			err = os.WriteFile(wrongOverridePath, []byte(wrongOverrideConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override config in the CORRECT location
			correctOverridePath := filepath.Join(overrideConfigDir, "test.yaml")
			correctOverrideConfig := `test_setting: "correct_override"`

			err = os.WriteFile(correctOverridePath, []byte(correctOverrideConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Verify the correct override is used (from ~/.devex/config/)
			Expect(viper.GetString("test_setting")).To(Equal("correct_override"))
		})
	})

	Context("LoadCrossPlatformSettings Override Functionality", func() {
		It("applies overrides to cross-platform settings", func() {
			// Create default terminal applications config
			defaultAppPath := filepath.Join(defaultConfigDir, "terminal.yaml")
			defaultAppConfig := `
terminal_applications:
  development:
    - name: "git"
      description: "Version control"
      category: "Development Tools"
      default: true
`
			err := os.WriteFile(defaultAppPath, []byte(defaultAppConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override terminal applications config
			overrideAppPath := filepath.Join(overrideConfigDir, "terminal.yaml")
			overrideAppConfig := `
terminal_applications:
  development:
    - name: "git"
      description: "Custom Git Description"
      category: "Development Tools"
      default: false
    - name: "custom-tool"
      description: "User added tool"
      category: "Custom"
      default: true
`
			err = os.WriteFile(overrideAppPath, []byte(overrideAppConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create minimal other required config files for hybrid structure
			configFiles := map[string]string{
				"terminal-optional.yaml":     "terminal_optional_applications: {}",
				"desktop.yaml":               "desktop_applications: {}",
				"desktop-optional.yaml":      "desktop_optional_applications: {}",
				"databases.yaml":             "database_applications: {}",
				"programming-languages.yaml": "programming_languages: []",
				"fonts.yaml":                 "fonts: []",
				"shell.yaml":                 "shell: []",
				"dotfiles.yaml":              "git: []\nssh: {}\nterminal: {}",
				"gnome.yaml":                 "# GNOME config",
				"kde.yaml":                   "# KDE config",
				"macos.yaml":                 "# macOS config",
				"windows.yaml":               "# Windows config",
			}
			for filename, content := range configFiles {
				path := filepath.Join(defaultConfigDir, filename)
				err = os.WriteFile(path, []byte(content), 0644)
				Expect(err).ToNot(HaveOccurred())
			}

			// Load settings
			settings, err := config.LoadCrossPlatformSettings(tempHomeDir)
			Expect(err).ToNot(HaveOccurred())

			// Verify override was applied to terminal development apps
			terminalDevApps := settings.Terminal.Development
			Expect(len(terminalDevApps)).To(Equal(2)) // git (overridden) + custom-tool (added)

			// Find the git app and verify it was overridden
			var gitApp *types.CrossPlatformApp
			for i := range terminalDevApps {
				if terminalDevApps[i].Name == "git" {
					gitApp = &terminalDevApps[i]
					break
				}
			}
			Expect(gitApp).ToNot(BeNil())
			Expect(gitApp.Description).To(Equal("Custom Git Description"))
			Expect(gitApp.Default).To(BeFalse()) // Should be overridden from true to false

			// Find the custom tool and verify it was added
			var customApp *types.CrossPlatformApp
			for i := range terminalDevApps {
				if terminalDevApps[i].Name == "custom-tool" {
					customApp = &terminalDevApps[i]
					break
				}
			}
			Expect(customApp).ToNot(BeNil())
			Expect(customApp.Description).To(Equal("User added tool"))
			Expect(customApp.Default).To(BeTrue())
		})

		It("handles partial config overrides correctly", func() {
			// Create default programming languages config
			defaultLangPath := filepath.Join(defaultConfigDir, "programming-languages.yaml")
			defaultLangConfig := `programming_languages:
  - name: "node"
    description: "Node.js runtime"
    default: true
`
			err := os.WriteFile(defaultLangPath, []byte(defaultLangConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create default fonts config
			defaultFontsPath := filepath.Join(defaultConfigDir, "fonts.yaml")
			defaultFontsConfig := `fonts:
  - name: "JetBrains Mono"
    description: "Programming font"
`
			err = os.WriteFile(defaultFontsPath, []byte(defaultFontsConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override fonts config (only override fonts, not programming languages)
			overrideFontsPath := filepath.Join(overrideConfigDir, "fonts.yaml")
			overrideFontsConfig := `fonts:
  - name: "Fira Code"
    description: "Custom programming font"
`
			err = os.WriteFile(overrideFontsPath, []byte(overrideFontsConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create minimal other required config files for hybrid structure
			configFiles := map[string]string{
				"terminal.yaml":          "terminal_applications: {}",
				"terminal-optional.yaml": "terminal_optional_applications: {}",
				"desktop.yaml":           "desktop_applications: {}",
				"desktop-optional.yaml":  "desktop_optional_applications: {}",
				"databases.yaml":         "database_applications: {}",
				"shell.yaml":             "shell: []",
				"dotfiles.yaml":          "git: []\nssh: {}\nterminal: {}",
				"gnome.yaml":             "# GNOME config",
				"kde.yaml":               "# KDE config",
				"macos.yaml":             "# macOS config",
				"windows.yaml":           "# Windows config",
			}
			for filename, content := range configFiles {
				path := filepath.Join(defaultConfigDir, filename)
				err = os.WriteFile(path, []byte(content), 0644)
				Expect(err).ToNot(HaveOccurred())
			}

			// Load settings
			settings, err := config.LoadCrossPlatformSettings(tempHomeDir)
			Expect(err).ToNot(HaveOccurred())

			// Verify programming languages use default (no override)
			langs := settings.ProgrammingLanguages
			Expect(len(langs)).To(Equal(1))
			Expect(langs[0].Name).To(Equal("node"))

			// Verify fonts from override are used
			fonts := settings.Fonts
			Expect(len(fonts)).To(Equal(1))
			Expect(fonts[0].Name).To(Equal("Fira Code"))
		})
	})

	Context("Error Handling", func() {
		It("handles invalid YAML in default config", func() {
			// Create invalid default config
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			invalidConfig := `invalid: yaml: content: [unclosed`

			err := os.WriteFile(defaultConfigPath, []byte(invalidConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create valid override config
			overrideConfigPath := filepath.Join(overrideConfigDir, "test.yaml")
			validConfig := `test_setting: "override_value"`

			err = os.WriteFile(overrideConfigPath, []byte(validConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs - should skip invalid default and use override
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Should still load the valid override
			Expect(viper.GetString("test_setting")).To(Equal("override_value"))
		})

		It("handles invalid YAML in override config", func() {
			// Create valid default config
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			validConfig := `test_setting: "default_value"`

			err := os.WriteFile(defaultConfigPath, []byte(validConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create invalid override config
			overrideConfigPath := filepath.Join(overrideConfigDir, "test.yaml")
			invalidConfig := `invalid: yaml: content: [unclosed`

			err = os.WriteFile(overrideConfigPath, []byte(invalidConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Load configs - should skip invalid override and use default
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Should fall back to default when override is invalid
			Expect(viper.GetString("test_setting")).To(Equal("default_value"))
		})

		It("handles permission errors gracefully", func() {
			// Create default config
			defaultConfigPath := filepath.Join(defaultConfigDir, "test.yaml")
			defaultConfig := `test_setting: "default_value"`

			err := os.WriteFile(defaultConfigPath, []byte(defaultConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create override config with restricted permissions
			overrideConfigPath := filepath.Join(overrideConfigDir, "test.yaml")
			overrideConfig := `test_setting: "override_value"`

			err = os.WriteFile(overrideConfigPath, []byte(overrideConfig), 0000) // No permissions
			Expect(err).ToNot(HaveOccurred())

			// Load configs - should handle permission error gracefully
			viper, err := config.LoadConfigs(tempHomeDir, []string{"test.yaml"})
			Expect(err).ToNot(HaveOccurred())
			Expect(viper).ToNot(BeNil())

			// Should fall back to default when override is unreadable
			Expect(viper.GetString("test_setting")).To(Equal("default_value"))

			// Clean up - restore permissions for deletion
			os.Chmod(overrideConfigPath, 0644)
		})
	})

	Context("Real-world Integration Tests", func() {
		It("mimics actual DevEx configuration override workflow", func() {
			// Create realistic default terminal applications config
			defaultTerminalPath := filepath.Join(defaultConfigDir, "terminal.yaml")
			defaultTerminalConfig := `
terminal_applications:
  development:
    - name: "git"
      description: "Version control system"
      category: "Development Tools"
      default: true
      linux:
        install_method: "apt"
        install_command: "git"
        uninstall_command: "git"
    - name: "docker"
      description: "Container platform"
      category: "Development Tools"
      default: false
      linux:
        install_method: "apt"
        install_command: "docker.io"
        uninstall_command: "docker.io"
`
			err := os.WriteFile(defaultTerminalPath, []byte(defaultTerminalConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// User wants to customize: enable docker by default, add custom app
			overrideTerminalPath := filepath.Join(overrideConfigDir, "terminal.yaml")
			overrideTerminalConfig := `
terminal_applications:
  development:
    - name: "git"
      description: "Version control system"
      category: "Development Tools"
      default: true
      linux:
        install_method: "apt"
        install_command: "git"
        uninstall_command: "git"
    - name: "docker"
      description: "Container platform"
      category: "Development Tools"
      default: true  # User enabled this
      linux:
        install_method: "apt"
        install_command: "docker.io"
        uninstall_command: "docker.io"
    - name: "custom-tool"
      description: "User's custom development tool"
      category: "Custom"
      default: true
      linux:
        install_method: "curlpipe"
        install_command: "https://example.com/install.sh"
`
			err = os.WriteFile(overrideTerminalPath, []byte(overrideTerminalConfig), 0644)
			Expect(err).ToNot(HaveOccurred())

			// Create minimal other required config files for hybrid structure
			configFiles := map[string]string{
				"terminal-optional.yaml":     "terminal_optional_applications: {}",
				"desktop.yaml":               "desktop_applications: {}",
				"desktop-optional.yaml":      "desktop_optional_applications: {}",
				"databases.yaml":             "database_applications: {}",
				"programming-languages.yaml": "programming_languages: []",
				"fonts.yaml":                 "fonts: []",
				"shell.yaml":                 "shell: []",
				"dotfiles.yaml":              "git: []\nssh: {}\nterminal: {}",
				"gnome.yaml":                 "# GNOME config",
				"kde.yaml":                   "# KDE config",
				"macos.yaml":                 "# macOS config",
				"windows.yaml":               "# Windows config",
			}
			for filename, content := range configFiles {
				path := filepath.Join(defaultConfigDir, filename)
				err = os.WriteFile(path, []byte(content), 0644)
				Expect(err).ToNot(HaveOccurred())
			}

			// Load cross-platform settings
			settings, err := config.LoadCrossPlatformSettings(tempHomeDir)
			Expect(err).ToNot(HaveOccurred())

			// Verify the configuration was properly loaded with new hybrid structure
			allApps := settings.GetAllApps()
			Expect(len(allApps)).To(BeNumerically(">", 0)) // Should have some apps

			// Check that apps can be retrieved from terminal applications
			terminalDevApps := settings.Terminal.Development
			Expect(len(terminalDevApps)).To(Equal(3)) // git, docker, custom-tool

			// Check that docker was overridden to default: true
			var dockerApp *types.CrossPlatformApp
			for i := range terminalDevApps {
				if terminalDevApps[i].Name == "docker" {
					dockerApp = &terminalDevApps[i]
					break
				}
			}
			Expect(dockerApp).ToNot(BeNil())
			Expect(dockerApp.Default).To(BeTrue()) // This was overridden from false to true

			// Check that custom tool was added
			var customApp *types.CrossPlatformApp
			for i := range terminalDevApps {
				if terminalDevApps[i].Name == "custom-tool" {
					customApp = &terminalDevApps[i]
					break
				}
			}
			Expect(customApp).ToNot(BeNil())
			Expect(customApp.Category).To(Equal("Custom"))

			// Verify GetDefaultApps includes the overridden defaults
			defaultApps := settings.GetDefaultApps()
			defaultAppNames := make([]string, len(defaultApps))
			for i, app := range defaultApps {
				defaultAppNames[i] = app.Name
			}

			Expect(defaultAppNames).To(ContainElement("git"))
			Expect(defaultAppNames).To(ContainElement("docker"))      // Now default due to override
			Expect(defaultAppNames).To(ContainElement("custom-tool")) // User's custom default
		})
	})
})
