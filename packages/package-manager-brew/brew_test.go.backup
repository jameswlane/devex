package brew

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Homebrew Installer", func() {
	var (
		installer *BrewInstaller
		mockExec  *mocks.MockCommandExecutor
		mockRepo  *MockRepository
		ctx       context.Context
	)

	BeforeEach(func() {
		installer = New()
		mockExec = mocks.NewMockCommandExecutor()
		mockRepo = &MockRepository{}
		ctx = context.Background()

		// Store original and replace with mock
		utils.CommandExec = mockExec

		// Reset version cache
		ResetVersionCache()
	})

	AfterEach(func() {
		// Reset mock state
		mockExec.Commands = []string{}
		mockExec.FailingCommand = ""
		mockExec.FailingCommands = make(map[string]bool)
		mockExec.InstallationState = make(map[string]bool)
	})

	Describe("New", func() {
		It("creates a new Homebrew installer instance", func() {
			installer := New()
			Expect(installer).ToNot(BeNil())
			Expect(installer).To(BeAssignableToTypeOf(&BrewInstaller{}))
		})
	})

	Describe("Version Detection", func() {
		Context("when detecting Homebrew version", func() {
			It("detects version correctly", func() {
				// The mock already returns proper Homebrew version by default

				version, err := getBrewVersion()

				Expect(err).NotTo(HaveOccurred())
				Expect(version.Major).To(Equal(4))
				Expect(version.Minor).To(Equal(0))
				Expect(version.Patch).To(Equal(10))
			})

			It("handles version detection failure gracefully", func() {
				mockExec.FailingCommands["brew --version"] = true

				// Install should continue even with version detection failure
				_ = installer.Install("git", mockRepo)

				// Will fail due to validation suite, but should have attempted version detection
				Expect(mockExec.Commands).To(ContainElement("brew --version"))
			})

			It("caches version after first detection", func() {
				// First call
				version1, err1 := getBrewVersion()
				Expect(err1).NotTo(HaveOccurred())

				// Clear commands to verify no additional calls
				mockExec.Commands = []string{}

				// Second call should use cached value
				version2, err2 := getBrewVersion()
				Expect(err2).NotTo(HaveOccurred())

				Expect(version1).To(Equal(version2))
				Expect(mockExec.Commands).To(BeEmpty()) // No additional commands
			})
		})
	})

	Describe("Install", func() {
		Context("with valid Homebrew package", func() {
			It("installs package successfully", func() {
				packageName := "git"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify Homebrew commands were executed
				Expect(mockExec.Commands).To(ContainElement("brew install git"))
			})

			It("installs package with complex name", func() {
				packageName := "node@18"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify commands were executed
				Expect(mockExec.Commands).To(ContainElement("brew install node@18"))
			})

			It("handles package with version specification", func() {
				packageName := "python@3.11"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))
			})
		})

		Context("when package is already installed", func() {
			BeforeEach(func() {
				// Mark the package as already installed
				mockExec.InstallationState["git"] = true
			})

			It("skips installation if package already installed", func() {
				packageName := "git"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				// Repository should not be called for already installed packages
				Expect(mockRepo.AddedApps).To(BeEmpty())

				// The brew install command should NOT be executed
				Expect(mockExec.Commands).ToNot(ContainElement("brew install git"))
			})
		})

		Context("with validation errors", func() {
			It("returns error for invalid package name", func() {
				packageName := ""

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid package name"))
			})

			It("returns error for dangerous package names", func() {
				packageName := ".."

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid package name"))
			})

			It("returns error for package with invalid characters", func() {
				packageName := "package$with!invalid@chars"

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("contains invalid characters"))
			})

			It("returns error when package is not available", func() {
				packageName := "nonexistent-package"
				// Mock search command failing
				mockExec.FailingCommands["brew search nonexistent-package"] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package validation failed"))
			})
		})

		Context("when Homebrew install command fails", func() {
			BeforeEach(func() {
				// Mock Homebrew install command failing
				packageName := "valid-package"
				installCommand := "brew install " + packageName
				mockExec.FailingCommands[installCommand] = true
			})

			It("returns Homebrew install command execution error", func() {
				packageName := "valid-package"

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install Brew package"))
				Expect(err.Error()).To(ContainSubstring(packageName))
				// Repository should not be modified when installation fails
				Expect(mockRepo.AddedApps).To(BeEmpty())
			})
		})

		Context("when repository operations fail", func() {
			BeforeEach(func() {
				mockRepo.ShouldFailAddApp = true
			})

			It("returns repository error after successful installation", func() {
				packageName := "git"

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to add Brew package"))
				Expect(err.Error()).To(ContainSubstring("to repository"))
				Expect(err.Error()).To(ContainSubstring(packageName))

				// Verify that installation was attempted
				Expect(mockExec.Commands).To(ContainElement("brew install git"))
			})
		})

		Context("with edge cases and input validation", func() {
			It("handles package with special Homebrew characters", func() {
				packageName := "node@18.15.0"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command was called with special characters
				Expect(mockExec.Commands).To(ContainElement("brew install node@18.15.0"))
			})

			It("handles package with slashes (tap notation)", func() {
				packageName := "homebrew/cask/firefox"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))
			})

			It("handles package with plus signs", func() {
				packageName := "gtk+3"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))
			})
		})
	})

	Describe("Uninstall", func() {
		Context("when package is installed", func() {
			BeforeEach(func() {
				mockExec.InstallationState["git"] = true
			})

			It("uninstalls package successfully", func() {
				packageName := "git"

				err := installer.Uninstall(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("brew uninstall git"))
				Expect(mockRepo.DeletedApps).To(ContainElement(packageName))
			})

			It("handles complex package names in uninstall", func() {
				packageName := "node@18"
				mockExec.InstallationState[packageName] = true

				err := installer.Uninstall(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("brew uninstall node@18"))
			})
		})

		Context("when package is not installed", func() {
			It("skips uninstall gracefully", func() {
				packageName := "git"

				err := installer.Uninstall(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				// Should not attempt to uninstall
				Expect(mockExec.Commands).ToNot(ContainElement("brew uninstall git"))
				Expect(mockRepo.DeletedApps).To(BeEmpty())
			})
		})
	})

	Describe("IsInstalled", func() {
		It("returns true when package is installed", func() {
			packageName := "git"
			mockExec.InstallationState["git"] = true

			installed, err := installer.IsInstalled(packageName)

			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
		})

		It("returns false when package is not installed", func() {
			packageName := "nonexistent-package"

			installed, err := installer.IsInstalled(packageName)

			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})

		It("handles complex package names", func() {
			packageName := "node@18"
			mockExec.InstallationState[packageName] = true

			installed, err := installer.IsInstalled(packageName)

			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
		})
	})

	Describe("PackageManager Interface", func() {
		Describe("InstallPackages", func() {
			It("installs multiple packages in batch", func() {
				packages := []string{"git", "node", "python"}

				err := installer.InstallPackages(ctx, packages, false)

				Expect(err).NotTo(HaveOccurred())
				// Should update Homebrew first
				Expect(mockExec.Commands).To(ContainElement("brew update"))
				// Should install packages in batch
				Expect(mockExec.Commands).To(ContainElement("brew install git node python"))
			})

			It("handles dry run mode", func() {
				packages := []string{"git", "node"}

				err := installer.InstallPackages(ctx, packages, true)

				Expect(err).NotTo(HaveOccurred())
				// Should not execute any install commands in dry run
				Expect(mockExec.Commands).To(BeEmpty())
			})

			It("handles empty package list", func() {
				packages := []string{}

				err := installer.InstallPackages(ctx, packages, false)

				Expect(err).NotTo(HaveOccurred())
			})

			It("falls back to individual installs on batch failure", func() {
				packages := []string{"git", "failing-package", "node"}
				mockExec.FailingCommands["brew install git failing-package node"] = true

				err := installer.InstallPackages(ctx, packages, false)

				Expect(err).NotTo(HaveOccurred())
				// Should try individual packages after batch failure
				Expect(mockExec.Commands).To(ContainElement("brew install git"))
				Expect(mockExec.Commands).To(ContainElement("brew install node"))
			})
		})

		Describe("IsAvailable", func() {
			It("returns true when Homebrew is available and working", func() {
				// Mock will return success for these commands by default

				available := installer.IsAvailable(ctx)

				Expect(available).To(BeTrue())
				Expect(mockExec.Commands).To(ContainElement("which brew"))
				Expect(mockExec.Commands).To(ContainElement("brew --version"))
			})

			It("returns false when Homebrew is not installed", func() {
				mockExec.FailingCommands["which brew"] = true

				available := installer.IsAvailable(ctx)

				Expect(available).To(BeFalse())
			})

			It("returns false when Homebrew is not working", func() {
				mockExec.FailingCommands["brew --version"] = true

				available := installer.IsAvailable(ctx)

				Expect(available).To(BeFalse())
			})
		})

		Describe("GetName", func() {
			It("returns correct package manager name", func() {
				name := installer.GetName()
				Expect(name).To(Equal("brew"))
			})
		})
	})

	Describe("Helper Functions", func() {
		Describe("validateBrewPackageName", func() {
			It("validates correct package names", func() {
				validNames := []string{"git", "node@18", "homebrew/cask/firefox", "gtk+3", "my-package_v1.0"}

				for _, name := range validNames {
					err := validateBrewPackageName(name)
					Expect(err).NotTo(HaveOccurred(), "Expected %s to be valid", name)
				}
			})

			It("rejects invalid package names", func() {
				invalidCases := []struct {
					name     string
					expected string
				}{
					{"", "cannot be empty"},
					{"  ", "cannot be empty"},
					{"..", "invalid package name"},
					{".", "invalid package name"},
					{"package$with!bad", "contains invalid characters"},
					{"package with spaces", "contains invalid characters"},
				}

				for _, testCase := range invalidCases {
					err := validateBrewPackageName(testCase.name)
					Expect(err).To(HaveOccurred(), "Expected %s to be invalid", testCase.name)
					Expect(err.Error()).To(ContainSubstring(testCase.expected))
				}
			})
		})

		Describe("validateBrewPackageAvailability", func() {
			It("validates available packages", func() {
				packageName := "git"

				err := validateBrewPackageAvailability(packageName)

				Expect(err).NotTo(HaveOccurred())
			})

			It("returns error for unavailable packages", func() {
				packageName := "nonexistent-package"

				err := validateBrewPackageAvailability(packageName)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no installable candidate found"))
			})

			It("handles search command failures", func() {
				packageName := "test-package"
				mockExec.FailingCommands["brew search test-package"] = true

				err := validateBrewPackageAvailability(packageName)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to search"))
				Expect(err.Error()).To(ContainSubstring("internet connection"))
			})
		})

		Describe("performPostInstallationSetup", func() {
			It("runs cleanup after installation", func() {
				err := performPostInstallationSetup("git")

				Expect(err).NotTo(HaveOccurred())
				// Should attempt cleanup
				Expect(mockExec.Commands).To(ContainElement("brew cleanup 2>/dev/null || true"))
			})

			It("handles completion packages", func() {
				err := performPostInstallationSetup("bash-completion")

				Expect(err).NotTo(HaveOccurred())
				// Should check brew prefix for completions
				Expect(mockExec.Commands).To(ContainElement("brew --prefix 2>/dev/null || true"))
			})
		})
	})
})

// MockRepository for testing
type MockRepository struct {
	AddedApps        []string
	DeletedApps      []string
	ShouldFailAddApp bool
	Apps             []types.AppConfig
	KeyValueStore    map[string]string
}

func (m *MockRepository) AddApp(name string) error {
	if m.ShouldFailAddApp {
		return errors.New("mock repository error")
	}
	m.AddedApps = append(m.AddedApps, name)
	return nil
}

func (m *MockRepository) DeleteApp(name string) error {
	m.DeletedApps = append(m.DeletedApps, name)
	return nil
}

func (m *MockRepository) GetApp(name string) (*types.AppConfig, error) {
	for _, app := range m.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, errors.New("app not found")
}

func (m *MockRepository) ListApps() ([]types.AppConfig, error) {
	return m.Apps, nil
}

func (m *MockRepository) SaveApp(app types.AppConfig) error {
	return nil
}

func (m *MockRepository) Set(key string, value string) error {
	if m.KeyValueStore == nil {
		m.KeyValueStore = make(map[string]string)
	}
	m.KeyValueStore[key] = value
	return nil
}

func (m *MockRepository) Get(key string) (string, error) {
	if m.KeyValueStore == nil {
		return "", errors.New("key not found")
	}
	if val, ok := m.KeyValueStore[key]; ok {
		return val, nil
	}
	return "", errors.New("key not found")
}
