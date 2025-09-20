package commands_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Config Command", func() {
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
		tempDir, err = os.MkdirTemp("", "devex-config-test")
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

	Describe("Main config command", func() {
		It("should create config command with correct properties", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)

			Expect(cmd.Use).To(Equal("config"))
			Expect(cmd.Short).To(ContainSubstring("Manage DevEx configuration files"))
			Expect(cmd.RunE).ToNot(BeNil())
		})

		It("should have expected subcommands", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)

			subcommands := cmd.Commands()
			subcommandNames := make([]string, len(subcommands))
			for i, subcmd := range subcommands {
				subcommandNames[i] = subcmd.Name()
			}

			Expect(subcommandNames).To(ContainElements("show", "edit", "validate", "diff"))
		})
	})

	Describe("config show subcommand", func() {
		BeforeEach(func() {
			// Create test configuration files
			testConfigs := map[string]string{
				"applications.yaml": `applications:
  - name: git
    category: development`,
				"environment.yaml": `shell: bash
languages:
  - node
  - python`,
				"system.yaml": `configure_git: true
configure_ssh: false`,
			}

			for filename, content := range testConfigs {
				path := filepath.Join(configDir, filename)
				err := os.WriteFile(path, []byte(content), 0600)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("should show configuration status in table format", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			cmd.SetArgs([]string{"show"})

			err := cmd.Execute()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should support different output formats", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)

			// Test YAML format
			cmd.SetArgs([]string{"show", "--format", "yaml"})
			err := cmd.Execute()
			Expect(err).ToNot(HaveOccurred())

			// Test JSON format
			cmd.SetArgs([]string{"show", "--format", "json"})
			err = cmd.Execute()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should support detailed output", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			cmd.SetArgs([]string{"show", "--detailed"})

			err := cmd.Execute()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("config validate subcommand", func() {
		Context("with valid configuration files", func() {
			BeforeEach(func() {
				// Create valid test configuration files
				testConfigs := map[string]interface{}{
					"applications.yaml": map[string]interface{}{
						"applications": []map[string]interface{}{
							{
								"name":        "git",
								"category":    "development",
								"description": "Version control",
							},
						},
					},
					"environment.yaml": map[string]interface{}{
						"shell":     "bash",
						"languages": []string{"node", "python"},
					},
					"system.yaml": map[string]interface{}{
						"configure_git": true,
						"configure_ssh": false,
					},
				}

				for filename, content := range testConfigs {
					data, err := yaml.Marshal(content)
					Expect(err).ToNot(HaveOccurred())

					path := filepath.Join(configDir, filename)
					err = os.WriteFile(path, data, 0600)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should validate successfully", func() {
				cmd := commands.NewConfigCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"validate"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
			})

			It("should support strict validation", func() {
				cmd := commands.NewConfigCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"validate", "--strict"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid configuration files", func() {
			BeforeEach(func() {
				// Create invalid YAML
				invalidYaml := "invalid: yaml: content: ["
				path := filepath.Join(configDir, "applications.yaml")
				err := os.WriteFile(path, []byte(invalidYaml), 0600)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should detect validation errors", func() {
				cmd := commands.NewConfigCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"validate"})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("validation failed"))
			})
		})

		Context("with missing required files", func() {
			It("should detect missing files", func() {
				cmd := commands.NewConfigCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"validate"})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("validation failed"))
			})
		})
	})

	Describe("config edit subcommand", func() {
		It("should have correct usage", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			editCmd := findSubcommand(cmd, "edit")

			Expect(editCmd).ToNot(BeNil())
			Expect(editCmd.Use).To(Equal("edit [config-type]"))
		})

		It("should support editor flag", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			editCmd := findSubcommand(cmd, "edit")

			editorFlag := editCmd.Flags().Lookup("editor")
			Expect(editorFlag).ToNot(BeNil())
			Expect(editorFlag.Shorthand).To(Equal("e"))
		})

		Context("when creating missing config files", func() {
			It("should create files if they don't exist", func() {
				// Set EDITOR to a command that just touches the file and exits
				os.Setenv("EDITOR", "touch")
				defer os.Unsetenv("EDITOR")

				cmd := commands.NewConfigCmd(mockRepo, *settings)
				cmd.SetArgs([]string{"edit", "applications"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				// Verify file was created
				appPath := filepath.Join(configDir, "applications.yaml")
				Expect(appPath).To(BeAnExistingFile())
			})
		})
	})

	Describe("config diff subcommand", func() {
		var backupDir string

		BeforeEach(func() {
			backupDir = filepath.Join(configDir, "backups")
			err := os.MkdirAll(backupDir, 0750)
			Expect(err).ToNot(HaveOccurred())

			// Create a backup file
			backupContent := struct {
				Applications []types.AppConfig `yaml:"applications"`
			}{
				Applications: []types.AppConfig{
					{
						BaseConfig: types.BaseConfig{
							Name:        "git",
							Description: "Version control",
							Category:    "development",
						},
					},
				},
			}

			data, err := yaml.Marshal(backupContent)
			Expect(err).ToNot(HaveOccurred())

			backupPath := filepath.Join(backupDir, "backup-test.yaml")
			err = os.WriteFile(backupPath, data, 0600)
			Expect(err).ToNot(HaveOccurred())

			// Create current config file
			currentPath := filepath.Join(configDir, "applications.yaml")
			err = os.WriteFile(currentPath, data, 0600)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have correct usage", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			diffCmd := findSubcommand(cmd, "diff")

			Expect(diffCmd).ToNot(BeNil())
			Expect(diffCmd.Use).To(Equal("diff [backup-file]"))
		})

		It("should support tool flag", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			diffCmd := findSubcommand(cmd, "diff")

			toolFlag := diffCmd.Flags().Lookup("tool")
			Expect(toolFlag).ToNot(BeNil())
			Expect(toolFlag.Shorthand).To(Equal("t"))
		})

		It("should find backup files automatically", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			cmd.SetArgs([]string{"diff"})

			// This will try to run diff, which might fail in test environment
			// but should not fail due to missing backup files
			err := cmd.Execute()
			// We don't expect success here since diff command might not be available
			// in all test environments, but we should not get backup-related errors
			if err != nil {
				Expect(err.Error()).ToNot(ContainSubstring("no backup files found"))
			}
		})
	})

	Describe("Flag parsing", func() {
		It("should parse format flag for show command", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			showCmd := findSubcommand(cmd, "show")

			err := showCmd.ParseFlags([]string{"--format", "json"})
			Expect(err).ToNot(HaveOccurred())

			formatFlag := showCmd.Flags().Lookup("format")
			Expect(formatFlag.Value.String()).To(Equal("json"))
		})

		It("should parse detailed flag for show command", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			showCmd := findSubcommand(cmd, "show")

			err := showCmd.ParseFlags([]string{"--detailed"})
			Expect(err).ToNot(HaveOccurred())

			detailedFlag := showCmd.Flags().Lookup("detailed")
			Expect(detailedFlag.Value.String()).To(Equal("true"))
		})

		It("should parse fix flag for validate command", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			validateCmd := findSubcommand(cmd, "validate")

			err := validateCmd.ParseFlags([]string{"--fix"})
			Expect(err).ToNot(HaveOccurred())

			fixFlag := validateCmd.Flags().Lookup("fix")
			Expect(fixFlag.Value.String()).To(Equal("true"))
		})

		It("should parse strict flag for validate command", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			validateCmd := findSubcommand(cmd, "validate")

			err := validateCmd.ParseFlags([]string{"--strict"})
			Expect(err).ToNot(HaveOccurred())

			strictFlag := validateCmd.Flags().Lookup("strict")
			Expect(strictFlag.Value.String()).To(Equal("true"))
		})
	})

	Describe("Error handling", func() {
		It("should handle invalid config types for edit command", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			cmd.SetArgs([]string{"edit", "invalid-type"})

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown configuration type"))
		})

		It("should handle missing backup directory for diff command", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			cmd.SetArgs([]string{"diff"})

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no backups directory found"))
		})

		It("should handle invalid format for show command", func() {
			cmd := commands.NewConfigCmd(mockRepo, *settings)
			cmd.SetArgs([]string{"show", "--format", "invalid"})

			err := cmd.Execute()
			// Should fall back to table format without error
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

// Helper function to find a subcommand by name
func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}
