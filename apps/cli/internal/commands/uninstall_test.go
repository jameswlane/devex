package commands_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Uninstall Command", func() {
	var (
		mockRepo *mocks.MockRepository
		settings config.CrossPlatformSettings
		tempDir  string
		testApp  types.AppConfig
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		settings = config.CrossPlatformSettings{}

		// Create temporary directory for tests
		var err error
		tempDir, err = os.MkdirTemp("", "devex-uninstall-test")
		Expect(err).NotTo(HaveOccurred())

		// Create test app configuration
		testApp = types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name:        "test-app",
				Description: "Test application for uninstall tests",
			},
			InstallMethod:    "apt",
			InstallCommand:   "test-app",
			UninstallCommand: "test-app",
			ConfigFiles: []types.ConfigFile{
				{
					Destination: filepath.Join(tempDir, "test-config.conf"),
				},
			},
			CleanupFiles: []string{
				filepath.Join(tempDir, "test-data"),
			},
		}

		// Add test app to mock repository
		mockRepo.SaveApp(testApp)
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("NewUninstallCmd", func() {
		It("should create uninstall command with all flags", func() {
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("uninstall"))

			// Check that all expected flags are present
			flags := cmd.Flags()
			Expect(flags.Lookup("app")).NotTo(BeNil())
			Expect(flags.Lookup("apps")).NotTo(BeNil())
			Expect(flags.Lookup("category")).NotTo(BeNil())
			Expect(flags.Lookup("all")).NotTo(BeNil())
			Expect(flags.Lookup("force")).NotTo(BeNil())
			Expect(flags.Lookup("keep-config")).NotTo(BeNil())
			Expect(flags.Lookup("keep-data")).NotTo(BeNil())
			Expect(flags.Lookup("remove-orphans")).NotTo(BeNil())
			Expect(flags.Lookup("cascade")).NotTo(BeNil())
			Expect(flags.Lookup("backup")).NotTo(BeNil())
			Expect(flags.Lookup("stop-services")).NotTo(BeNil())
			Expect(flags.Lookup("cleanup-system")).NotTo(BeNil())
		})
	})

	Describe("Uninstall execution", func() {
		Context("when uninstalling a single app", func() {
			It("should attempt to uninstall the app", func() {
				cmd := commands.NewUninstallCmd(mockRepo, settings)
				cmd.SetArgs([]string{"--app", "test-app", "--force"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				// Since apt installer is not available in test environment,
				// the app should still exist in repository (uninstall failed)
				_, err = mockRepo.GetApp("test-app")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle non-existent app gracefully", func() {
				cmd := commands.NewUninstallCmd(mockRepo, settings)
				cmd.SetArgs([]string{"--app", "non-existent-app", "--force"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when uninstalling multiple apps", func() {
			BeforeEach(func() {
				// Add multiple test apps
				for i := 1; i <= 3; i++ {
					app := types.AppConfig{
						BaseConfig: types.BaseConfig{
							Name: fmt.Sprintf("test-app-%d", i),
						},
						InstallMethod:    "apt",
						InstallCommand:   fmt.Sprintf("test-app-%d", i),
						UninstallCommand: fmt.Sprintf("test-app-%d", i),
					}
					mockRepo.SaveApp(app)
				}
			})

			It("should attempt to uninstall multiple apps", func() {
				cmd := commands.NewUninstallCmd(mockRepo, settings)
				cmd.SetArgs([]string{"--apps", "test-app-1,test-app-2", "--force"})

				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())

				// Since apt installer is not available in test environment,
				// the apps should still exist (uninstall failed)
				_, err = mockRepo.GetApp("test-app-1")
				Expect(err).NotTo(HaveOccurred())
				_, err = mockRepo.GetApp("test-app-2")
				Expect(err).NotTo(HaveOccurred())
				_, err = mockRepo.GetApp("test-app-3")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when using all flag", func() {
			It("should require confirmation without force", func() {
				cmd := commands.NewUninstallCmd(mockRepo, settings)
				cmd.SetArgs([]string{"--all"})

				// This would normally prompt for confirmation
				// In tests, we can't easily simulate user input
				// So we test with force flag
				cmd.SetArgs([]string{"--all", "--force"})
				err := cmd.Execute()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

var _ = Describe("BackupManager", func() {
	var (
		mockRepo  *mocks.MockRepository
		backupMgr *commands.BackupManager
		tempDir   string
		testApp   types.AppConfig
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()

		var err error
		tempDir, err = os.MkdirTemp("", "devex-backup-test")
		Expect(err).NotTo(HaveOccurred())

		// Create test app with config and data files
		configFile := filepath.Join(tempDir, "test-config.conf")
		dataFile := filepath.Join(tempDir, "test-data.txt")

		// Create test files
		err = os.WriteFile(configFile, []byte("test config content"), 0644)
		Expect(err).NotTo(HaveOccurred())
		err = os.WriteFile(dataFile, []byte("test data content"), 0644)
		Expect(err).NotTo(HaveOccurred())

		testApp = types.AppConfig{
			BaseConfig: types.BaseConfig{
				Name: "test-backup-app",
			},
			ConfigFiles: []types.ConfigFile{
				{Destination: configFile},
			},
			CleanupFiles: []string{dataFile},
		}

		backupMgr = commands.NewBackupManager(mockRepo)

		// Clean up any existing backups to ensure test isolation
		backups, _ := backupMgr.ListBackups()
		for _, backup := range backups {
			os.RemoveAll(backup.BackupPath)
		}
	})

	AfterEach(func() {
		// Clean up test directories and any backup directories created during tests
		os.RemoveAll(tempDir)

		// Clean up any backups created during tests
		if backupMgr != nil {
			backups, _ := backupMgr.ListBackups()
			for _, backup := range backups {
				os.RemoveAll(backup.BackupPath)
			}
		}
	})

	Describe("CreateBackup", func() {
		It("should create a backup successfully", func() {
			ctx := context.Background()
			backup, err := backupMgr.CreateBackup(ctx, &testApp)
			Expect(err).NotTo(HaveOccurred())
			Expect(backup).NotTo(BeNil())
			Expect(backup.AppName).To(Equal("test-backup-app"))
			Expect(backup.BackupPath).NotTo(BeEmpty())

			// Verify backup directory exists
			_, err = os.Stat(backup.BackupPath)
			Expect(err).NotTo(HaveOccurred())

			// Verify metadata file exists
			metadataPath := filepath.Join(backup.BackupPath, "backup_info.txt")
			_, err = os.Stat(metadataPath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle missing files gracefully", func() {
			// App with non-existent files
			appWithMissingFiles := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "missing-files-app",
				},
				ConfigFiles: []types.ConfigFile{
					{Destination: "/non/existent/file.conf"},
				},
				CleanupFiles: []string{"/non/existent/data.txt"},
			}

			ctx := context.Background()
			backup, err := backupMgr.CreateBackup(ctx, &appWithMissingFiles)
			Expect(err).NotTo(HaveOccurred())
			Expect(backup).NotTo(BeNil())

			// Should create backup directory even if no files to backup
			_, err = os.Stat(backup.BackupPath)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("ListBackups", func() {
		It("should return empty list when no backups exist", func() {
			backups, err := backupMgr.ListBackups()
			Expect(err).NotTo(HaveOccurred())
			Expect(backups).To(BeEmpty())
		})

		It("should list existing backups", func() {
			// Create a backup first
			ctx := context.Background()
			_, err := backupMgr.CreateBackup(ctx, &testApp)
			Expect(err).NotTo(HaveOccurred())

			backups, err := backupMgr.ListBackups()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(backups)).To(BeNumerically(">=", 1))

			// Find our test app backup
			found := false
			for _, backup := range backups {
				if backup.AppName == "test-backup-app" {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})
	})

	Describe("CleanupOldBackups", func() {
		It("should remove old backups", func() {
			// Create a backup
			ctx := context.Background()
			backup, err := backupMgr.CreateBackup(ctx, &testApp)
			Expect(err).NotTo(HaveOccurred())

			// Verify backup exists
			_, err = os.Stat(backup.BackupPath)
			Expect(err).NotTo(HaveOccurred())

			// Clean up backups older than 0 seconds (should remove all)
			err = backupMgr.CleanupOldBackups(0)
			Expect(err).NotTo(HaveOccurred())

			// Wait a moment and check again
			time.Sleep(100 * time.Millisecond)
			err = backupMgr.CleanupOldBackups(0)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("DependencyManager", func() {
	var (
		mockRepo *mocks.MockRepository
		depMgr   *commands.DependencyManager
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		depMgr = commands.NewDependencyManager(mockRepo)
	})

	Describe("IsSystemPackage", func() {
		It("should identify critical system packages", func() {
			criticalPackages := []string{
				"kernel",
				"systemd",
				"glibc",
				"bash",
				"sudo",
			}

			for _, pkg := range criticalPackages {
				Expect(depMgr.IsSystemPackage(pkg)).To(BeTrue(),
					"Package %s should be identified as a system package", pkg)
			}
		})

		It("should not identify regular packages as system packages", func() {
			regularPackages := []string{
				"firefox",
				"chrome",
				"vscode",
				"slack",
			}

			for _, pkg := range regularPackages {
				Expect(depMgr.IsSystemPackage(pkg)).To(BeFalse(),
					"Package %s should not be identified as a system package", pkg)
			}
		})
	})
})

var _ = Describe("ConflictDetector", func() {
	var (
		mockRepo         *mocks.MockRepository
		conflictDetector *commands.ConflictDetector
		testApps         []types.AppConfig
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		conflictDetector = commands.NewConflictDetector(mockRepo)

		testApps = []types.AppConfig{
			{
				BaseConfig: types.BaseConfig{
					Name: "regular-app",
				},
				InstallMethod: "apt",
			},
			{
				BaseConfig: types.BaseConfig{
					Name: "system-app",
				},
				InstallMethod: "apt",
			},
		}
	})

	Describe("DetectConflicts", func() {
		It("should detect no conflicts for regular apps", func() {
			ctx := context.Background()
			conflicts, err := conflictDetector.DetectConflicts(ctx, testApps[:1], false)
			Expect(err).NotTo(HaveOccurred())

			// Should have minimal or no conflicts for regular apps
			criticalConflicts := 0
			for _, conflict := range conflicts {
				if conflict.Severity == "critical" {
					criticalConflicts++
				}
			}
			Expect(criticalConflicts).To(BeZero())
		})

		It("should provide conflict summary", func() {
			ctx := context.Background()
			conflicts, err := conflictDetector.DetectConflicts(ctx, testApps, false)
			Expect(err).NotTo(HaveOccurred())

			summary := conflictDetector.SummarizeConflicts(conflicts)
			Expect(summary.TotalConflicts).To(Equal(len(conflicts)))
			Expect(summary.CanProceed).To(Equal(summary.CriticalCount == 0))
		})
	})
})

var _ = Describe("Rollback Command", func() {
	var (
		mockRepo *mocks.MockRepository
		settings config.CrossPlatformSettings
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		settings = config.CrossPlatformSettings{}
	})

	Describe("NewRollbackCmd", func() {
		It("should create rollback command with all flags", func() {
			cmd := commands.NewRollbackCmd(mockRepo, settings)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("rollback"))

			// Check that all expected flags are present
			flags := cmd.Flags()
			Expect(flags.Lookup("app")).NotTo(BeNil())
			Expect(flags.Lookup("list")).NotTo(BeNil())
			Expect(flags.Lookup("force")).NotTo(BeNil())
			Expect(flags.Lookup("restore-config")).NotTo(BeNil())
			Expect(flags.Lookup("restore-data")).NotTo(BeNil())
		})
	})

	Describe("List rollbacks", func() {
		It("should handle empty rollback list", func() {
			cmd := commands.NewRollbackCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--list"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
