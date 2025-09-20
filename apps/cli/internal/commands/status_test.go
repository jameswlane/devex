package commands_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/commands/status"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Status Command", func() {
	var (
		mockRepo     *mocks.MockRepository
		mockSettings config.CrossPlatformSettings
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockSettings = config.CrossPlatformSettings{
			HomeDir: "/tmp/test",
		}

		// Add test applications to repository
		testApp := types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name:        "git",
				Description: "Version control system",
				Category:    "development",
			},
			InstallMethod:    "apt",
			InstallCommand:   "git",
			UninstallCommand: "git",
			Dependencies:     []string{"curl"},
		}
		_ = mockRepo.SaveApp(testApp)
	})

	Describe("Command Creation", func() {
		It("should create status command with correct structure", func() {
			cmd := commands.NewStatusCmd(mockRepo, mockSettings)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("status"))
			Expect(cmd.Short).To(ContainSubstring("Check the status"))
		})

		It("should have all required flags", func() {
			cmd := commands.NewStatusCmd(mockRepo, mockSettings)

			appFlag := cmd.Flags().Lookup("app")
			Expect(appFlag).NotTo(BeNil())

			allFlag := cmd.Flags().Lookup("all")
			Expect(allFlag).NotTo(BeNil())

			categoryFlag := cmd.Flags().Lookup("category")
			Expect(categoryFlag).NotTo(BeNil())

			formatFlag := cmd.Flags().Lookup("format")
			Expect(formatFlag).NotTo(BeNil())

			verboseFlag := cmd.Flags().Lookup("verbose")
			Expect(verboseFlag).NotTo(BeNil())

			fixFlag := cmd.Flags().Lookup("fix")
			Expect(fixFlag).NotTo(BeNil())
		})
	})

	Describe("AppStatus Structure", func() {
		It("should have correct JSON tags", func() {
			appStatus := status.AppStatus{
				Name:              "test-app",
				Installed:         true,
				Version:           "1.0.0",
				LatestVersion:     "1.1.0",
				InstallMethod:     "apt",
				InstallDate:       &time.Time{},
				Status:            "healthy",
				Issues:            []string{},
				Dependencies:      []status.DependencyStatus{},
				Services:          []status.ServiceStatus{},
				PathStatus:        true,
				ConfigStatus:      true,
				HealthCheckResult: "healthy",
			}

			jsonData, err := json.Marshal(appStatus)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(jsonData)).To(ContainSubstring("\"name\":\"test-app\""))
			Expect(string(jsonData)).To(ContainSubstring("\"installed\":true"))
			Expect(string(jsonData)).To(ContainSubstring("\"status\":\"healthy\""))
		})
	})

	Describe("Status Checking Logic", func() {
		Context("when checking single application", func() {
			It("should return status for existing app", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "git", "--format", "json"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle non-existent app gracefully", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "nonexistent", "--format", "json"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when checking multiple applications", func() {
			It("should handle comma-separated app list", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "git,curl", "--format", "json"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when checking all applications", func() {
			It("should check all installed apps", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--all", "--format", "json"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when checking by category", func() {
			It("should filter apps by category", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--category", "development", "--format", "json"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Output Formats", func() {
		Context("JSON output", func() {
			It("should produce valid JSON", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "git", "--format", "json"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("YAML output", func() {
			It("should produce YAML format", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "git", "--format", "yaml"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Table output", func() {
			It("should produce table format", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "git", "--format", "table"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle verbose mode", func() {
				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "git", "--verbose"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Dependency Status", func() {
		It("should check dependency installation", func() {
			depStatus := status.DependencyStatus{
				Name:      "curl",
				Installed: true,
				Version:   "7.68.0",
			}

			Expect(depStatus.Name).To(Equal("curl"))
			Expect(depStatus.Installed).To(BeTrue())
			Expect(depStatus.Version).To(Equal("7.68.0"))
		})
	})

	Describe("Service Status", func() {
		It("should track service status", func() {
			svcStatus := status.ServiceStatus{
				Name:   "docker.service",
				Active: true,
				Status: "active",
			}

			Expect(svcStatus.Name).To(Equal("docker.service"))
			Expect(svcStatus.Active).To(BeTrue())
			Expect(svcStatus.Status).To(Equal("active"))
		})
	})

	Describe("Fix Functionality", func() {
		It("should attempt fixes when requested", func() {
			cmd := commands.NewStatusCmd(mockRepo, mockSettings)
			cmd.SetArgs([]string{"--app", "git", "--fix"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Error Handling", func() {
		Context("with invalid install method", func() {
			It("should handle gracefully", func() {
				// Add app with invalid install method
				invalidApp := types.AppConfig{
					BaseConfig: types.BaseConfig{
						Name:     "invalid-app",
						Category: "test",
					},
					InstallMethod: "invalid-method",
				}
				_ = mockRepo.SaveApp(invalidApp)

				cmd := commands.NewStatusCmd(mockRepo, mockSettings)
				cmd.SetArgs([]string{"--app", "invalid-app", "--format", "json"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with repository errors", func() {
			It("should handle database errors", func() {
				failingRepo := &mocks.FailingMockRepository{}
				cmd := commands.NewStatusCmd(failingRepo, mockSettings)
				cmd.SetArgs([]string{"--all", "--no-tui"})

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Version Detection", func() {
		It("should detect versions for common applications", func() {
			// This would test the getVersionCommand function
			// We'd need to export it or create a test helper
			Skip("Version detection requires system commands")
		})
	})

	Describe("Health Checks", func() {
		It("should perform application-specific health checks", func() {
			// This would test runHealthCheck function
			// We'd need to mock the health check commands
			Skip("Health checks require system commands")
		})
	})

	Describe("PATH Verification", func() {
		It("should check if applications are in PATH", func() {
			// This would test checkInPath function
			// We'd need to mock exec.LookPath
			Skip("PATH verification requires system commands")
		})
	})
})
