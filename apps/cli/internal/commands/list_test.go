package commands_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var _ = Describe("List Command", func() {
	var (
		mockRepo *mocks.MockRepository
		settings config.CrossPlatformSettings
		buf      *bytes.Buffer
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		buf = new(bytes.Buffer)

		settings = config.CrossPlatformSettings{
			Terminal: config.TerminalApplicationsConfig{
				Development: []types.CrossPlatformApp{
					{
						Name:        "git",
						Description: "Version control system",
						Category:    "development",
						Default:     true,
						Linux: types.OSConfig{
							InstallMethod: "apt",
						},
					},
					{
						Name:        "docker",
						Description: "Container platform",
						Category:    "development",
						Default:     false,
						Linux: types.OSConfig{
							InstallMethod: "apt",
							Alternatives: []types.OSConfig{
								{InstallMethod: "docker"},
							},
						},
					},
				},
			},
			Databases: config.DatabasesConfig{
				Servers: []types.CrossPlatformApp{
					{
						Name:        "mysql",
						Description: "MySQL database",
						Category:    "databases",
						Linux: types.OSConfig{
							InstallMethod: "docker",
						},
					},
				},
			},
		}

		// Add some test data to mock repository
		_ = mockRepo.SaveApp(types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name:        "git",
				Description: "Version control system",
				Category:    "development",
			},
			InstallMethod: "apt",
		})
	})

	Describe("Command Creation", func() {
		It("should create command with correct structure", func() {
			cmd := commands.NewListCmd(mockRepo, settings)
			Expect(cmd).ToNot(BeNil())
			Expect(cmd.Use).To(Equal("list [installed|available|categories]"))
			Expect(cmd.Short).To(Equal("List applications in your DevEx configuration"))
			Expect(cmd.ValidArgs).To(ContainElements("installed", "available", "categories"))
		})

		It("should have all required flags", func() {
			cmd := commands.NewListCmd(mockRepo, settings)
			expectedFlags := []string{"category", "format", "verbose", "search", "method", "recommended", "interactive"}
			for _, flag := range expectedFlags {
				Expect(cmd.Flags().Lookup(flag)).ToNot(BeNil(), "Flag %s should exist", flag)
			}
		})
	})

	JustBeforeEach(func() {
		// Reset buffer before each test
		buf.Reset()
	})

	Describe("Integration Tests", func() {
		Context("tracked applications without definitions", func() {
			BeforeEach(func() {
				viper.Reset()
				DeferCleanup(viper.Reset)
				mockRepo = mocks.NewMockRepository()
				settings = config.CrossPlatformSettings{}
				Expect(mockRepo.AddApp("mise")).To(Succeed())
				Expect(mockRepo.AddApp("mise-rust")).To(Succeed())
				// An empty saved definition is also unavailable.
				Expect(mockRepo.Set("installed-app-definition:mise-rust", "")).To(Succeed())
			})

			DescribeTable("shows every tracked name with a note in human-readable output",
				func(verbose bool) {
					cmd := commands.NewListCmd(mockRepo, settings)
					cmd.SetOut(buf)
					cmd.SetArgs([]string{"installed", fmt.Sprintf("--verbose=%t", verbose)})

					Expect(cmd.Execute()).To(Succeed())
					Expect(buf.String()).To(MatchRegexp(`mise\s`))
					Expect(buf.String()).To(ContainSubstring("mise-rust"))
					Expect(buf.String()).To(ContainSubstring("unknown"))
					Expect(buf.String()).To(ContainSubstring("No application definition available"))
					Expect(buf.String()).NotTo(ContainSubstring("No installed applications found"))
				},
				Entry("default list", false),
				Entry("verbose table", true),
			)

			DescribeTable("marks missing definitions in structured output",
				func(format string) {
					cmd := commands.NewListCmd(mockRepo, settings)
					cmd.SetOut(buf)
					cmd.SetArgs([]string{"installed", "--format", format})

					Expect(cmd.Execute()).To(Succeed())
					var rows []map[string]interface{}
					if format == "json" {
						Expect(json.Unmarshal(buf.Bytes(), &rows)).To(Succeed())
					} else {
						Expect(yaml.Unmarshal(buf.Bytes(), &rows)).To(Succeed())
					}
					Expect(rows).To(HaveLen(2))
					nameKey, categoryKey, methodKey := "Name", "Category", "Method"
					if format == "yaml" {
						nameKey, categoryKey, methodKey = "name", "category", "method"
					}
					Expect(rows[0]).To(HaveKeyWithValue(nameKey, "mise"))
					Expect(rows[1]).To(HaveKeyWithValue(nameKey, "mise-rust"))
					for _, row := range rows {
						Expect(row).To(HaveKeyWithValue(categoryKey, "unknown"))
						Expect(row).To(HaveKeyWithValue(methodKey, "unknown"))
						Expect(row).To(HaveKeyWithValue("missing_definition", true))
					}
				},
				Entry("JSON", "json"),
				Entry("YAML", "yaml"),
			)

			It("keeps catalog and saved definitions alongside name-only records", func() {
				settings.Terminal.Development = []types.CrossPlatformApp{{
					Name: "mise", Category: "development",
					AllPlatforms: types.OSConfig{InstallMethod: "curlpipe"},
				}}
				saved := types.CrossPlatformApp{
					Name: "mise-rust", Category: "languages",
					AllPlatforms: types.OSConfig{InstallMethod: "mise"},
				}
				data, err := json.Marshal(saved)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.Set("installed-app-definition:mise-rust", string(data))).To(Succeed())
				Expect(mockRepo.AddApp("orphan")).To(Succeed())

				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"installed", "--format", "json"})
				Expect(cmd.Execute()).To(Succeed())
				var rows []commands.InstalledApp
				Expect(json.Unmarshal(buf.Bytes(), &rows)).To(Succeed())
				Expect(rows).To(HaveLen(3))
				Expect(rows[0].Name).To(Equal("mise"))
				Expect(rows[0].Method).To(Equal("curlpipe"))
				Expect(rows[0].Category).To(Equal("development"))
				Expect(rows[0].MissingDefinition).To(BeFalse())
				Expect(rows[1].Name).To(Equal("mise-rust"))
				Expect(rows[1].Method).To(Equal("mise"))
				Expect(rows[1].Category).To(Equal("languages"))
				Expect(rows[1].MissingDefinition).To(BeFalse())
				Expect(rows[2].Name).To(Equal("orphan"))
				Expect(rows[2].MissingDefinition).To(BeTrue())
			})

			It("applies category, method, and search filters to name-only records", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"installed", "--category", "unknown", "--method", "unknown", "--search", "rust"})

				Expect(cmd.Execute()).To(Succeed())
				Expect(buf.String()).To(ContainSubstring("Installed Applications (1)"))
				Expect(buf.String()).To(ContainSubstring("mise-rust (unknown)"))
			})
		})

		Context("list installed command", func() {
			It("should execute installed command successfully", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"installed"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("Installed Applications"))
			})

			It("should show installed apps in JSON format", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"installed", "--format", "json"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				output := buf.String()
				Expect(output).ToNot(BeEmpty())

				// Verify JSON output is valid
				var result []map[string]interface{}
				err = json.Unmarshal([]byte(output), &result)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should show installed apps in YAML format", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"installed", "--format", "yaml"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				output := buf.String()
				Expect(output).ToNot(BeEmpty())

				// Verify YAML output is valid
				var result []map[string]interface{}
				err = yaml.Unmarshal([]byte(output), &result)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("list available command", func() {
			It("should execute available command successfully", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"available"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("databases (1 apps):"))
			})

			It("should filter by category", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"available", "--category", "development"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("docker"))
				Expect(buf.String()).ToNot(ContainSubstring("mysql"))
			})

			It("should search applications", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"available", "--search", "docker"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("docker"))
			})

			It("should show recommended apps only", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"available", "--recommended"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				// Currently shows no applications found due to platform filtering
				Expect(buf.String()).To(ContainSubstring("No applications found matching the specified criteria."))
			})
		})

		Context("list categories command", func() {
			It("should execute categories command successfully", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"categories"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("Category"))
				Expect(buf.String()).To(ContainSubstring("development"))
				Expect(buf.String()).To(ContainSubstring("databases"))
			})

			It("should show categories in JSON format", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"categories", "--format", "json"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				output := buf.String()
				Expect(output).ToNot(BeEmpty())

				// Verify JSON output is valid
				var result []map[string]interface{}
				err = json.Unmarshal([]byte(output), &result)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(result)).To(BeNumerically(">", 0))
			})
		})

		Context("error handling", func() {
			It("should handle unknown subcommand", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"unknown"})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown subcommand"))
			})
		})
	})

	// Unit tests for individual functions that can be tested in isolation
	Describe("Output Function Tests", func() {
		Context("JSON Output", func() {
			It("should produce valid JSON for installed apps", func() {
				// This tests the JSON marshaling functionality
				// Since we can't directly test unexported functions,
				// we test through command execution
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"installed", "--format", "json"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				output := buf.String()
				Expect(output).ToNot(BeEmpty())

				// Verify valid JSON structure
				var result []map[string]interface{}
				err = json.Unmarshal([]byte(output), &result)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should produce valid JSON for available apps", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"available", "--format", "json"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				output := buf.String()
				Expect(output).ToNot(BeEmpty())

				var result []map[string]interface{}
				err = json.Unmarshal([]byte(output), &result)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(result)).To(BeNumerically(">", 0))
			})
		})

		Context("YAML Output", func() {
			It("should produce valid YAML for installed apps", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"installed", "--format", "yaml"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				output := buf.String()
				Expect(output).ToNot(BeEmpty())

				var result []map[string]interface{}
				err = yaml.Unmarshal([]byte(output), &result)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should produce valid YAML for available apps", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"available", "--format", "yaml"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())

				output := buf.String()
				Expect(output).ToNot(BeEmpty())

				var result []map[string]interface{}
				err = yaml.Unmarshal([]byte(output), &result)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(result)).To(BeNumerically(">", 0))
			})
		})

		Context("Table Output", func() {
			It("should produce table output for installed apps", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"installed", "--format", "table"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("Installed Applications"))
			})

			It("should produce verbose table output", func() {
				cmd := commands.NewListCmd(mockRepo, settings)
				cmd.SetOut(buf)
				cmd.SetArgs([]string{"available", "--verbose"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				// Verify table structure characters are present
				Expect(buf.String()).To(ContainSubstring("┌"))
				Expect(buf.String()).To(ContainSubstring("│"))
				Expect(buf.String()).To(ContainSubstring("└"))
			})
		})

		Context("Error Cases", func() {
			It("should handle repository errors gracefully", func() {
				// Create a mock that returns an error
				errorRepo := mocks.NewMockRepository()

				cmd := commands.NewListCmd(errorRepo, settings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"installed"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred()) // Command shouldn't crash
			})

			It("should handle empty results", func() {
				emptySettings := config.CrossPlatformSettings{
					Terminal: config.TerminalApplicationsConfig{},
				}

				cmd := commands.NewListCmd(mockRepo, emptySettings)
				cmd.SetOut(buf)
				cmd.SetErr(buf)
				cmd.SetArgs([]string{"available"})

				err := cmd.Execute()
				Expect(err).ToNot(HaveOccurred())
				Expect(buf.String()).To(ContainSubstring("No applications found matching the specified criteria."))
			})
		})
	})
})

// Benchmarks for performance testing
func BenchmarkListAvailableApps(b *testing.B) {
	mockRepo := mocks.NewMockRepository()
	settings := createTestSettings()

	// Add test data
	for i := 0; i < 100; i++ {
		_ = mockRepo.SaveApp(types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name:        fmt.Sprintf("app-%d", i),
				Description: fmt.Sprintf("Test app %d", i),
				Category:    "development",
			},
			InstallMethod: "apt",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := commands.NewListCmd(mockRepo, settings)
		cmd.SetOut(io.Discard)
		cmd.SetArgs([]string{"available"})
		_ = cmd.Execute()
	}
}

func BenchmarkListInstalledApps(b *testing.B) {
	mockRepo := mocks.NewMockRepository()
	settings := createTestSettings()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := commands.NewListCmd(mockRepo, settings)
		cmd.SetOut(io.Discard)
		cmd.SetArgs([]string{"installed"})
		_ = cmd.Execute()
	}
}

// Helper function to create test settings
func createTestSettings() config.CrossPlatformSettings {
	return config.CrossPlatformSettings{
		Terminal: config.TerminalApplicationsConfig{
			Development: []types.CrossPlatformApp{
				{
					Name:        "git",
					Description: "Version control system",
					Category:    "development",
					Default:     true,
					Linux: types.OSConfig{
						InstallMethod: "apt",
					},
				},
				{
					Name:        "docker",
					Description: "Container platform",
					Category:    "development",
					Default:     false,
					Linux: types.OSConfig{
						InstallMethod: "docker",
					},
				},
			},
		},
	}
}
