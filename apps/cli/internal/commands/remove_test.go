package commands_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Remove Command", func() {
	var (
		mockRepo  *mocks.MockRepository
		settings  *config.CrossPlatformSettings
		tempDir   string
		configDir string
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()

		// Create temporary directory for config
		var err error
		tempDir, err = os.MkdirTemp("", "devex-remove-test")
		Expect(err).ToNot(HaveOccurred())

		settings = &config.CrossPlatformSettings{
			HomeDir: tempDir,
		}
		configDir = settings.GetConfigDir()

		// Ensure the config directory exists
		err = os.MkdirAll(configDir, 0750)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("Command Creation", func() {
		It("should create remove command with correct properties", func() {
			cmd := commands.NewRemoveCmd(mockRepo, *settings)

			Expect(cmd.Use).To(Equal("remove [application-name]"))
			Expect(cmd.Short).To(ContainSubstring("Remove applications from your DevEx configuration"))
			Expect(cmd.RunE).ToNot(BeNil())
		})

		It("should have expected flags", func() {
			cmd := commands.NewRemoveCmd(mockRepo, *settings)

			forceFlag := cmd.Flags().Lookup("force")
			Expect(forceFlag).ToNot(BeNil())
			Expect(forceFlag.Shorthand).To(Equal("f"))

			noBackupFlag := cmd.Flags().Lookup("no-backup")
			Expect(noBackupFlag).ToNot(BeNil())

			cascadeFlag := cmd.Flags().Lookup("cascade")
			Expect(cascadeFlag).ToNot(BeNil())

			dryRunFlag := cmd.Flags().Lookup("dry-run")
			Expect(dryRunFlag).ToNot(BeNil())
		})
	})

	Describe("Application removal", func() {
		var appsConfigPath string

		BeforeEach(func() {
			appsConfigPath = filepath.Join(configDir, "applications.yaml")

			// Create initial configuration with test apps
			initialConfig := struct {
				Applications []types.AppConfig `yaml:"applications"`
			}{
				Applications: []types.AppConfig{
					{
						BaseConfig: types.BaseConfig{
							Name:        "git",
							Description: "Version control system",
							Category:    "development",
						},
						Default: true,
					},
					{
						BaseConfig: types.BaseConfig{
							Name:        "docker",
							Description: "Container platform",
							Category:    "container",
						},
						Dependencies: []string{"git"},
						Default:      false,
					},
					{
						BaseConfig: types.BaseConfig{
							Name:        "vscode",
							Description: "Code editor",
							Category:    "development",
						},
						Dependencies: []string{"git"},
						Default:      false,
					},
				},
			}

			data, err := yaml.Marshal(initialConfig)
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile(appsConfigPath, data, 0600)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when removing an application without dependencies", func() {
			It("should remove the application successfully", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"vscode", "--no-backup"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				// Verify the application was removed
				data, err := os.ReadFile(appsConfigPath)
				Expect(err).ToNot(HaveOccurred())

				var config struct {
					Applications []types.AppConfig `yaml:"applications"`
				}
				err = yaml.Unmarshal(data, &config)
				Expect(err).ToNot(HaveOccurred())

				Expect(config.Applications).To(HaveLen(2))
				appNames := make([]string, len(config.Applications))
				for i, app := range config.Applications {
					appNames[i] = app.Name
				}
				Expect(appNames).To(ContainElements("git", "docker"))
				Expect(appNames).ToNot(ContainElement("vscode"))
			})
		})

		Context("when removing an application with dependencies", func() {
			It("should fail without force flag", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"git", "--no-backup"})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("required by"))
				Expect(err.Error()).To(ContainSubstring("docker"))
			})

			It("should succeed with force flag", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"git", "--force", "--no-backup"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				// Verify the application was removed
				data, err := os.ReadFile(appsConfigPath)
				Expect(err).ToNot(HaveOccurred())

				var config struct {
					Applications []types.AppConfig `yaml:"applications"`
				}
				err = yaml.Unmarshal(data, &config)
				Expect(err).ToNot(HaveOccurred())

				Expect(config.Applications).To(HaveLen(2))
				appNames := make([]string, len(config.Applications))
				for i, app := range config.Applications {
					appNames[i] = app.Name
				}
				Expect(appNames).ToNot(ContainElement("git"))
			})

			It("should remove dependents with cascade flag", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"git", "--cascade", "--no-backup"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				// Verify all dependent applications were removed
				data, err := os.ReadFile(appsConfigPath)
				Expect(err).ToNot(HaveOccurred())

				var config struct {
					Applications []types.AppConfig `yaml:"applications"`
				}
				err = yaml.Unmarshal(data, &config)
				Expect(err).ToNot(HaveOccurred())

				// Should only have apps without git dependency
				Expect(config.Applications).To(HaveLen(0))
			})
		})

		Context("when application doesn't exist", func() {
			It("should return an error", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"nonexistent-app", "--no-backup"})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found in configuration"))
			})
		})

		Context("with dry-run flag", func() {
			It("should show what would be removed without making changes", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"vscode", "--dry-run"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				// Verify no changes were made
				data, err := os.ReadFile(appsConfigPath)
				Expect(err).ToNot(HaveOccurred())

				var config struct {
					Applications []types.AppConfig `yaml:"applications"`
				}
				err = yaml.Unmarshal(data, &config)
				Expect(err).ToNot(HaveOccurred())

				// Should still have all 3 applications
				Expect(config.Applications).To(HaveLen(3))
			})

			It("should show cascade information in dry-run", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"git", "--cascade", "--dry-run"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Backup functionality", func() {
		var appsConfigPath string

		BeforeEach(func() {
			appsConfigPath = filepath.Join(configDir, "applications.yaml")

			// Create initial configuration
			initialConfig := struct {
				Applications []types.AppConfig `yaml:"applications"`
			}{
				Applications: []types.AppConfig{
					{
						BaseConfig: types.BaseConfig{
							Name:        "test-app",
							Description: "Test application",
							Category:    "test",
						},
					},
				},
			}

			data, err := yaml.Marshal(initialConfig)
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile(appsConfigPath, data, 0600)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when backup is enabled (default)", func() {
			It("should create a backup before removal", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"test-app"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				// Check if backup directory exists
				backupDir := filepath.Join(configDir, "backups")
				Expect(backupDir).To(BeADirectory())

				// Check if backup file was created
				files, err := os.ReadDir(backupDir)
				Expect(err).ToNot(HaveOccurred())
				Expect(files).To(HaveLen(1))
				Expect(files[0].Name()).To(ContainSubstring("applications_"))
				Expect(files[0].Name()).To(HaveSuffix(".yaml"))
			})
		})

		Context("when backup is disabled", func() {
			It("should not create a backup", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"test-app", "--no-backup"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				// Check if backup directory doesn't exist or is empty
				backupDir := filepath.Join(configDir, "backups")
				if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
					files, err := os.ReadDir(backupDir)
					Expect(err).ToNot(HaveOccurred())
					Expect(files).To(HaveLen(0))
				}
			})
		})
	})

	Describe("Flag parsing", func() {
		Context("with force flag", func() {
			It("should accept the force flag", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)

				err := cmd.ParseFlags([]string{"--force"})
				Expect(err).ToNot(HaveOccurred())

				forceFlag := cmd.Flags().Lookup("force")
				Expect(forceFlag.Value.String()).To(Equal("true"))
			})

			It("should accept the force shorthand", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)

				err := cmd.ParseFlags([]string{"-f"})
				Expect(err).ToNot(HaveOccurred())

				forceFlag := cmd.Flags().Lookup("force")
				Expect(forceFlag.Value.String()).To(Equal("true"))
			})
		})

		Context("with no-backup flag", func() {
			It("should accept the no-backup flag", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)

				err := cmd.ParseFlags([]string{"--no-backup"})
				Expect(err).ToNot(HaveOccurred())

				noBackupFlag := cmd.Flags().Lookup("no-backup")
				Expect(noBackupFlag.Value.String()).To(Equal("true"))
			})
		})

		Context("with cascade flag", func() {
			It("should accept the cascade flag", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)

				err := cmd.ParseFlags([]string{"--cascade"})
				Expect(err).ToNot(HaveOccurred())

				cascadeFlag := cmd.Flags().Lookup("cascade")
				Expect(cascadeFlag.Value.String()).To(Equal("true"))
			})
		})

		Context("with dry-run flag", func() {
			It("should accept the dry-run flag", func() {
				cmd := commands.NewRemoveCmd(mockRepo, *settings)

				err := cmd.ParseFlags([]string{"--dry-run"})
				Expect(err).ToNot(HaveOccurred())

				dryRunFlag := cmd.Flags().Lookup("dry-run")
				Expect(dryRunFlag.Value.String()).To(Equal("true"))
			})
		})
	})

	Describe("Case sensitivity", func() {
		var appsConfigPath string

		BeforeEach(func() {
			appsConfigPath = filepath.Join(configDir, "applications.yaml")

			// Create initial configuration
			initialConfig := struct {
				Applications []types.AppConfig `yaml:"applications"`
			}{
				Applications: []types.AppConfig{
					{
						BaseConfig: types.BaseConfig{
							Name:        "Test-App",
							Description: "Test application",
							Category:    "test",
						},
					},
				},
			}

			data, err := yaml.Marshal(initialConfig)
			Expect(err).ToNot(HaveOccurred())

			err = os.WriteFile(appsConfigPath, data, 0600)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle case-insensitive application names", func() {
			cmd := commands.NewRemoveCmd(mockRepo, *settings)
			cmd.SetArgs([]string{"test-app", "--no-backup"}) // lowercase

			err := cmd.Execute()
			Expect(err).ToNot(HaveOccurred())

			// Verify the application was removed
			data, err := os.ReadFile(appsConfigPath)
			Expect(err).ToNot(HaveOccurred())

			var config struct {
				Applications []types.AppConfig `yaml:"applications"`
			}
			err = yaml.Unmarshal(data, &config)
			Expect(err).ToNot(HaveOccurred())

			Expect(config.Applications).To(HaveLen(0))
		})
	})
})
