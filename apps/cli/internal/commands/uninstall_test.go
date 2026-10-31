package commands_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type recordingInstaller struct {
	uninstallCommands []string
	statusErrors      map[string]error
	uninstallErrors   map[string]error
	installed         map[string]bool
}

func (r *recordingInstaller) Install(string, types.Repository) error {
	return nil
}

func (r *recordingInstaller) Uninstall(command string, _ types.Repository) error {
	r.uninstallCommands = append(r.uninstallCommands, command)
	return r.uninstallErrors[command]
}

func (r *recordingInstaller) IsInstalled(command string) (bool, error) {
	if err := r.statusErrors[command]; err != nil {
		return false, err
	}
	if installed, ok := r.installed[command]; ok {
		return installed, nil
	}
	return true, nil
}

type nilLookupRepository struct {
	types.Repository
}

func (r *nilLookupRepository) GetApp(string) (*types.AppConfig, error) {
	return nil, nil
}

type definitionSetErrorRepository struct {
	types.Repository
}

func (r *definitionSetErrorRepository) Set(string, string) error {
	return fmt.Errorf("definition store unavailable")
}

type deleteAppErrorRepository struct {
	types.Repository
}

func (r *deleteAppErrorRepository) DeleteApp(string) error {
	return fmt.Errorf("tracking store unavailable")
}

func useRecordingInstaller(installer *recordingInstaller) {
	DeferCleanup(commands.SetUninstallInstallerForTest(func(context.Context, string) types.BaseInstaller {
		return installer
	}))
}

var _ = Describe("Uninstall Command", func() {
	var (
		mockRepo *mocks.MockRepository
		settings config.CrossPlatformSettings
		tempDir  string
		testApp  types.AppConfig
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
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

	Describe("application resolution", func() {
		It("uses the stored definition for a tracked inline app", func() {
			definition := types.CrossPlatformApp{
				Name:     "inline-app",
				Category: "Custom",
				AllPlatforms: types.OSConfig{
					InstallMethod:    "apt",
					InstallCommand:   "inline-package",
					UninstallCommand: "inline-package",
					Dependencies:     []string{"inline-dependency"},
					ConfigFiles:      []types.ConfigFile{{Destination: filepath.Join(tempDir, "inline.conf")}},
					CleanupFiles:     []string{filepath.Join(tempDir, "inline-data")},
				},
			}
			data, err := json.Marshal(definition)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockRepo.AddApp(definition.Name)).To(Succeed())
			Expect(mockRepo.Set("installed-app-definition:inline-app", string(data))).To(Succeed())

			resolved, err := commands.ResolveUninstallApp(mockRepo, settings, definition.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved.Name).To(Equal(definition.Name))
			Expect(resolved.Category).To(Equal("Custom"))
			Expect(resolved.InstallMethod).To(Equal("apt"))
			Expect(resolved.UninstallCommand).To(Equal("inline-package"))
			Expect(resolved.Dependencies).To(Equal([]string{"inline-dependency"}))
			Expect(resolved.ConfigFiles).To(Equal(definition.AllPlatforms.ConfigFiles))
			Expect(resolved.CleanupFiles).To(Equal(definition.AllPlatforms.CleanupFiles))
		})

		It("merges a tracked name with its bundled catalog definition", func() {
			definition := types.CrossPlatformApp{
				Name:     "bundled-app",
				Category: "Utilities",
				AllPlatforms: types.OSConfig{
					InstallMethod:    "apt",
					InstallCommand:   "bundled-package",
					UninstallCommand: "bundled-package",
					Dependencies:     []string{"bundled-dependency"},
					ConfigFiles:      []types.ConfigFile{{Destination: filepath.Join(tempDir, "bundled.conf")}},
				},
			}
			settings.Terminal.Utilities = []types.CrossPlatformApp{definition}
			Expect(mockRepo.AddApp(definition.Name)).To(Succeed())

			resolved, err := commands.ResolveUninstallApp(mockRepo, settings, definition.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved.Name).To(Equal(definition.Name))
			Expect(resolved.Category).To(Equal("Utilities"))
			Expect(resolved.InstallMethod).To(Equal("apt"))
			Expect(resolved.Dependencies).To(Equal([]string{"bundled-dependency"}))
			Expect(resolved.ConfigFiles).To(Equal(definition.AllPlatforms.ConfigFiles))
		})

		It("keeps support for known catalog apps installed outside DevEx", func() {
			definition := types.CrossPlatformApp{
				Name: "external-app",
				AllPlatforms: types.OSConfig{
					InstallMethod:  "apt",
					InstallCommand: "external-package",
				},
			}
			settings.Terminal.Utilities = []types.CrossPlatformApp{definition}

			resolved, err := commands.ResolveUninstallApp(mockRepo, settings, definition.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved.InstallCommand).To(Equal("external-package"))
		})

		It("handles an exact nil lookup without panicking", func() {
			repo := &nilLookupRepository{Repository: mockRepo}
			_, err := commands.ResolveUninstallApp(repo, settings, "unknown-app")
			Expect(err).To(MatchError(ContainSubstring("not tracked by DevEx")))
		})

		It("reports saved-definition read errors", func() {
			_, err := commands.ResolveUninstallApp(&mocks.FailingMockRepository{}, settings, "test-app")
			Expect(err).To(MatchError(ContainSubstring("failed to read installed definition")))
		})
	})

	Describe("Uninstall execution", func() {
		It("executes a stored inline definition with the install-command fallback and clears its records", func() {
			definition := types.CrossPlatformApp{
				Name: "inline-app",
				AllPlatforms: types.OSConfig{
					InstallMethod:  "apt",
					InstallCommand: "inline-package",
				},
			}
			data, err := json.Marshal(definition)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockRepo.AddApp(definition.Name)).To(Succeed())
			Expect(mockRepo.Set("installed-app-definition:inline-app", string(data))).To(Succeed())
			installer := &recordingInstaller{}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--app", definition.Name, "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(Equal([]string{"inline-package"}))
			_, err = mockRepo.GetApp(definition.Name)
			Expect(err).To(HaveOccurred())
			stored, err := mockRepo.Get("installed-app-definition:inline-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(stored).To(BeEmpty())
		})

		It("executes a catalog definition with the install-command fallback", func() {
			definition := types.CrossPlatformApp{
				Name: "catalog-app",
				AllPlatforms: types.OSConfig{
					InstallMethod:  "apt",
					InstallCommand: "catalog-package",
				},
			}
			settings.Terminal.Utilities = []types.CrossPlatformApp{definition}
			Expect(mockRepo.AddApp(definition.Name)).To(Succeed())
			installer := &recordingInstaller{}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--app", definition.Name, "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(Equal([]string{"catalog-package"}))
		})

		It("resolves a legacy inline app from runtime configuration", func() {
			viper.Reset()
			DeferCleanup(viper.Reset)
			viper.Set("applications", []any{map[string]any{
				"name":     "legacy-inline",
				"category": "Custom",
				"all_platforms": map[string]any{
					"install_method":  "apt",
					"install_command": "legacy-package",
				},
			}})
			Expect(mockRepo.AddApp("legacy-inline")).To(Succeed())
			installer := &recordingInstaller{}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--app", "legacy-inline", "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(Equal([]string{"legacy-package"}))
		})

		It("prefers the saved definition over a runtime inline override", func() {
			definition := types.CrossPlatformApp{Name: "test-app", AllPlatforms: types.OSConfig{
				InstallMethod: "apt", InstallCommand: "saved-package",
			}}
			data, err := json.Marshal(definition)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockRepo.Set("installed-app-definition:test-app", string(data))).To(Succeed())
			viper.Set("applications", []any{map[string]any{
				"name":          "test-app",
				"all_platforms": map[string]any{"install_method": "apt", "install_command": "current-package"},
			}})
			installer := &recordingInstaller{}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--app", "test-app", "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(Equal([]string{"saved-package"}))
		})

		It("retains the tracking and saved-definition records when uninstall fails", func() {
			definition := types.CrossPlatformApp{
				Name: "inline-app",
				AllPlatforms: types.OSConfig{
					InstallMethod:  "apt",
					InstallCommand: "inline-package",
				},
			}
			data, err := json.Marshal(definition)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockRepo.AddApp(definition.Name)).To(Succeed())
			Expect(mockRepo.Set("installed-app-definition:inline-app", string(data))).To(Succeed())
			installer := &recordingInstaller{uninstallErrors: map[string]error{
				"inline-package": fmt.Errorf("remove failed"),
			}}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--app", definition.Name, "--force"})

			Expect(cmd.Execute()).To(MatchError(And(ContainSubstring(definition.Name), ContainSubstring("remove failed"))))
			_, err = mockRepo.GetApp(definition.Name)
			Expect(err).NotTo(HaveOccurred())
			stored, err := mockRepo.Get("installed-app-definition:inline-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(stored).To(Equal(string(data)))
		})

		It("continues after mixed uninstall and status-discovery failures", func() {
			failing := types.AppConfig{
				BaseConfig:       types.BaseConfig{Name: "failing-app"},
				InstallMethod:    "apt",
				InstallCommand:   "failing-package",
				UninstallCommand: "failing-package",
			}
			broken := types.AppConfig{
				BaseConfig:       types.BaseConfig{Name: "broken-app"},
				InstallMethod:    "apt",
				InstallCommand:   "broken-package",
				UninstallCommand: "broken-package",
			}
			Expect(mockRepo.SaveApp(failing)).To(Succeed())
			Expect(mockRepo.SaveApp(broken)).To(Succeed())
			brokenDefinition := types.CrossPlatformApp{Name: broken.Name, AllPlatforms: types.OSConfig{
				InstallMethod: "apt", InstallCommand: broken.InstallCommand,
			}}
			data, err := json.Marshal(brokenDefinition)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockRepo.Set("installed-app-definition:broken-app", string(data))).To(Succeed())
			installer := &recordingInstaller{
				uninstallErrors: map[string]error{"failing-package": fmt.Errorf("remove failed")},
				statusErrors:    map[string]error{"broken-package": fmt.Errorf("discovery failed")},
			}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--apps", "test-app,failing-app,broken-app", "--force"})

			err = cmd.Execute()
			Expect(err).To(MatchError(And(ContainSubstring("remove failed"), ContainSubstring("discovery failed"))))
			Expect(installer.uninstallCommands).To(Equal([]string{"test-app", "failing-package"}))
			_, err = mockRepo.GetApp("test-app")
			Expect(err).To(HaveOccurred())
			_, err = mockRepo.GetApp("failing-app")
			Expect(err).NotTo(HaveOccurred())
			_, err = mockRepo.GetApp("broken-app")
			Expect(err).NotTo(HaveOccurred())
			stored, err := mockRepo.Get("installed-app-definition:broken-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(stored).To(Equal(string(data)))
		})

		It("clears tracking and saved-definition records after confirmed absence", func() {
			definition := types.CrossPlatformApp{
				Name: "test-app",
				AllPlatforms: types.OSConfig{
					InstallMethod:  "apt",
					InstallCommand: "test-app",
				},
			}
			data, err := json.Marshal(definition)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockRepo.Set("installed-app-definition:test-app", string(data))).To(Succeed())
			installer := &recordingInstaller{installed: map[string]bool{"test-app": false}}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--app", "test-app", "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(BeEmpty())
			_, err = mockRepo.GetApp("test-app")
			Expect(err).To(HaveOccurred())
			stored, err := mockRepo.Get("installed-app-definition:test-app")
			Expect(err).NotTo(HaveOccurred())
			Expect(stored).To(BeEmpty())
		})

		DescribeTable("reports saved-definition persistence errors",
			func(installed bool) {
				definition := types.CrossPlatformApp{
					Name: "test-app",
					AllPlatforms: types.OSConfig{
						InstallMethod:  "apt",
						InstallCommand: "test-app",
					},
				}
				data, err := json.Marshal(definition)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.Set("installed-app-definition:test-app", string(data))).To(Succeed())
				installer := &recordingInstaller{installed: map[string]bool{"test-app": installed}}
				useRecordingInstaller(installer)
				repo := &definitionSetErrorRepository{Repository: mockRepo}
				cmd := commands.NewUninstallCmd(repo, settings)
				cmd.SetArgs([]string{"--app", "test-app", "--force"})

				Expect(cmd.Execute()).To(MatchError(ContainSubstring("failed to clear saved application definition")))
				stored, err := mockRepo.Get("installed-app-definition:test-app")
				Expect(err).NotTo(HaveOccurred())
				Expect(stored).To(Equal(string(data)))
			},
			Entry("after removal", true),
			Entry("after confirmed absence", false),
		)

		DescribeTable("reports tracking-record deletion errors",
			func(installed bool) {
				installer := &recordingInstaller{installed: map[string]bool{"test-app": installed}}
				useRecordingInstaller(installer)
				repo := &deleteAppErrorRepository{Repository: mockRepo}
				cmd := commands.NewUninstallCmd(repo, settings)
				cmd.SetArgs([]string{"--app", "test-app", "--force"})

				Expect(cmd.Execute()).To(MatchError(ContainSubstring("tracking store unavailable")))
				_, err := mockRepo.GetApp("test-app")
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("after removal", true),
			Entry("after confirmed absence", false),
		)

		It("rejects a definition without any effective removal command before mutation", func() {
			app := types.AppConfig{
				BaseConfig:    types.BaseConfig{Name: "commandless-app"},
				InstallMethod: "apt",
			}
			Expect(mockRepo.SaveApp(app)).To(Succeed())
			installer := &recordingInstaller{}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--app", app.Name, "--force"})

			Expect(cmd.Execute()).To(MatchError(ContainSubstring("no uninstall or install command")))
			Expect(installer.uninstallCommands).To(BeEmpty())
			_, err := mockRepo.GetApp(app.Name)
			Expect(err).NotTo(HaveOccurred())
		})

		It("ignores resolution failures for apps known to be outside the requested category", func() {
			target := testApp
			target.Name = "development-app"
			target.Category = "Development"
			target.InstallCommand = "development-package"
			target.UninstallCommand = "development-package"
			other := types.AppConfig{
				BaseConfig:    types.BaseConfig{Name: "other-app", Category: "Utilities"},
				InstallMethod: "apt",
			}
			Expect(mockRepo.SaveApp(target)).To(Succeed())
			Expect(mockRepo.SaveApp(other)).To(Succeed())
			Expect(mockRepo.Set("installed-app-definition:other-app", "not-json")).To(Succeed())
			installer := &recordingInstaller{}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--category", "Development", "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(Equal([]string{"development-package"}))
			_, err := mockRepo.GetApp(other.Name)
			Expect(err).NotTo(HaveOccurred())
		})

		It("uninstalls explicitly selected dependents in cascade mode", func() {
			dependent := types.AppConfig{
				BaseConfig:    types.BaseConfig{Name: "dependent-app"},
				InstallMethod: "apt", InstallCommand: "dependent-package",
				Dependencies: []string{"test-app"},
			}
			Expect(mockRepo.SaveApp(dependent)).To(Succeed())
			installer := &recordingInstaller{}
			useRecordingInstaller(installer)
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--apps", "dependent-app,test-app", "--cascade", "--force"})

			Expect(cmd.Execute()).To(Succeed())
			Expect(installer.uninstallCommands).To(Equal([]string{"dependent-package", "test-app"}))
			_, err := mockRepo.GetApp(dependent.Name)
			Expect(err).To(HaveOccurred())
		})

		It("returns an error for an unknown app without panicking", func() {
			cmd := commands.NewUninstallCmd(&nilLookupRepository{Repository: mockRepo}, settings)
			cmd.SetArgs([]string{"--app", "non-existent-app", "--force"})

			Expect(cmd.Execute()).To(MatchError(ContainSubstring("non-existent-app")))
		})

		It("reports unresolved apps with unknown categories", func() {
			Expect(mockRepo.AddApp("legacy-unknown-category")).To(Succeed())
			cmd := commands.NewUninstallCmd(mockRepo, settings)
			cmd.SetArgs([]string{"--category", "Development", "--force"})

			Expect(cmd.Execute()).To(MatchError(ContainSubstring("legacy-unknown-category")))
			_, err := mockRepo.GetApp("legacy-unknown-category")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when removing orphans", func() {
			var orphanCalls int

			BeforeEach(func() {
				orphanCalls = 0
				DeferCleanup(commands.SetOrphanRemovalForTest(func(context.Context) {
					orphanCalls++
				}))
			})

			It("runs orphan cleanup after a successful uninstall", func() {
				installer := &recordingInstaller{}
				useRecordingInstaller(installer)
				cmd := commands.NewUninstallCmd(mockRepo, settings)
				cmd.SetArgs([]string{"--app", "test-app", "--force", "--remove-orphans"})

				Expect(cmd.Execute()).To(Succeed())
				Expect(orphanCalls).To(Equal(1))
			})

			It("does not run orphan cleanup without a successful uninstall", func() {
				installer := &recordingInstaller{uninstallErrors: map[string]error{
					"test-app": fmt.Errorf("remove failed"),
				}}
				useRecordingInstaller(installer)
				cmd := commands.NewUninstallCmd(mockRepo, settings)
				cmd.SetArgs([]string{"--app", "test-app", "--force", "--remove-orphans"})

				Expect(cmd.Execute()).To(HaveOccurred())
				Expect(orphanCalls).To(BeZero())
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

		It("resolves non-critical dependency conflicts in cascade mode", func() {
			conflicts := []commands.UninstallConflict{{
				Type:     commands.ConflictTypeDependency,
				AppName:  "dependency",
				Severity: "warning",
			}}
			remaining, err := conflictDetector.ResolveConflicts(conflicts, commands.UninstallOptions{Cascade: true})
			Expect(err).NotTo(HaveOccurred())
			Expect(remaining).To(BeEmpty())
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
