package pip

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Pip Installer", func() {
	var (
		installer *PIPInstaller
		mockExec  *mocks.MockCommandExecutor
		mockRepo  *MockRepository
	)

	BeforeEach(func() {
		installer = New()
		mockExec = mocks.NewMockCommandExecutor()
		mockRepo = &MockRepository{}

		// Store original and replace with mock
		utils.CommandExec = mockExec
	})

	AfterEach(func() {
		// Reset mock state
		mockExec.Commands = []string{}
		mockExec.FailingCommand = ""
		mockExec.FailingCommands = make(map[string]bool)
		mockExec.InstallationState = make(map[string]bool)
	})

	Describe("New", func() {
		It("creates a new Pip installer instance", func() {
			installer := New()
			Expect(installer).ToNot(BeNil())
			Expect(installer).To(BeAssignableToTypeOf(&PIPInstaller{}))
		})
	})

	Describe("Install", func() {
		Context("with valid pip package", func() {
			It("installs pip package successfully", func() {
				packageName := "requests"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify pip show command was called for installation check
				Expect(mockExec.Commands).To(ContainElement("pip show " + packageName))
				// Verify pip install command was called
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("installs pip package with hyphenated name", func() {
				packageName := "flask-restful"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify commands were executed
				Expect(mockExec.Commands).To(ContainElement("pip show " + packageName))
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("installs pip package with underscores", func() {
				packageName := "some_package_name"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify commands were executed
				Expect(mockExec.Commands).To(ContainElement("pip show " + packageName))
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("installs pip package with version specification", func() {
				packageName := "django==4.2.0"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify pip install command includes version specification
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("installs pip package with version range", func() {
				packageName := "numpy>=1.20,<2.0"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify pip install command includes version range
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})
		})

		Context("when package is already installed", func() {
			BeforeEach(func() {
				// Mark the package as already installed in the mock executor
				// This will be returned by "pip show package-name"
				mockExec.InstallationState["requests"] = true
			})

			It("skips installation if package already installed", func() {
				packageName := "requests"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				// Repository should not be called for already installed packages
				Expect(mockRepo.AddedApps).To(BeEmpty())

				// Verify that pip show command was called for installation check
				Expect(mockExec.Commands).To(ContainElement("pip show " + packageName))
				// The pip install command should NOT be executed since package already exists
				Expect(mockExec.Commands).ToNot(ContainElement("pip install " + packageName))
			})

			It("handles package with version specification when base package is installed", func() {
				// Test that version-specific requests don't interfere with base package detection
				packageName := "requests==2.28.0"
				// Only mark the base package as installed
				mockExec.InstallationState["requests"] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command was called for the version-specific package
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("handles similar package names correctly", func() {
				// Test that partial matches don't cause false positives
				packageName := "requests-oauthlib"
				// Only mark the base requests as installed, not the oauth extension
				mockExec.InstallationState["requests"] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command was called for the oauth extension
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})
		})

		Context("when pip show command fails", func() {
			BeforeEach(func() {
				// Mock pip show command failing - this will make the installer think the package is not installed
				// and try to install it, so we also need to make the install command fail to simulate the scenario
				// where pip itself is not available
				mockExec.FailingCommands["pip show numpy"] = true
				mockExec.FailingCommands["pip install numpy"] = true
			})

			It("returns error when pip is not available on system", func() {
				packageName := "numpy"

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
				// Repository should not be modified when installation fails
				Expect(mockRepo.AddedApps).To(BeEmpty())
			})
		})

		Context("when installation check fails but installation succeeds", func() {
			BeforeEach(func() {
				// Mock only the pip show command failing, but installation should work
				mockExec.FailingCommands["pip show scipy"] = true
			})

			It("continues with installation when check fails but package is not installed", func() {
				packageName := "scipy"

				err := installer.Install(packageName, mockRepo)

				// Should succeed because pip show failing just means package not installed
				// and the installation command succeeds
				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify that pip show was called for installation check
				Expect(mockExec.Commands).To(ContainElement("pip show " + packageName))
				// Verify that pip install was called since check indicated not installed
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})
		})

		Context("when pip install command fails", func() {
			BeforeEach(func() {
				// Mock pip install command failing
				packageName := "failing-package"
				installCommand := "pip install " + packageName
				mockExec.FailingCommands[installCommand] = true
			})

			It("returns pip install command execution error", func() {
				packageName := "failing-package"

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
				Expect(err.Error()).To(ContainSubstring(packageName))
				// Repository should not be modified when installation fails
				Expect(mockRepo.AddedApps).To(BeEmpty())
			})

			It("handles network connection failures", func() {
				packageName := "network-failing-package"
				installCommand := "pip install " + packageName
				mockExec.FailingCommands[installCommand] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
			})

			It("handles package not found in PyPI", func() {
				packageName := "non-existent-package"
				installCommand := "pip install " + packageName
				mockExec.FailingCommands[installCommand] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
			})

			It("handles permission errors", func() {
				packageName := "permission-denied-package"
				installCommand := "pip install " + packageName
				mockExec.FailingCommands[installCommand] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
			})

			It("handles dependency resolution failures", func() {
				packageName := "conflicting-deps-package"
				installCommand := "pip install " + packageName
				mockExec.FailingCommands[installCommand] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
			})
		})

		Context("when repository operations fail", func() {
			BeforeEach(func() {
				mockRepo.ShouldFailAddApp = true
			})

			It("returns repository error after successful installation", func() {
				packageName := "matplotlib"

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to add pip package"))
				Expect(err.Error()).To(ContainSubstring("to repository"))
				Expect(err.Error()).To(ContainSubstring(packageName))

				// Verify that installation was attempted
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})
		})

		Context("with edge cases and input validation", func() {
			It("handles empty package name", func() {
				packageName := ""

				err := installer.Install(packageName, mockRepo)

				// With empty package name, IsAppInstalled returns true (no commands to check)
				// so installer thinks package is already installed and skips installation
				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(BeEmpty())
				// No commands should be executed since it thinks the package is already installed
				Expect(mockExec.Commands).To(BeEmpty())
			})

			It("handles package name with special characters", func() {
				packageName := "package-with-dashes_and_underscores.dots"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command was called with special characters
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("handles very long package names", func() {
				packageName := "very-long-package-name-with-many-segments-that-should-still-work-properly"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))
			})

			It("handles package with git URL specification", func() {
				packageName := "git+https://github.com/user/repo.git"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command includes git URL
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("handles package with local file path", func() {
				packageName := "./local/path/to/package"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command includes local path
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("handles package with extras specification", func() {
				packageName := "requests[security,socks]"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command includes extras
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})

			It("handles package with complex version constraints", func() {
				packageName := "Django>=3.2,!=3.2.5,<4.0"

				err := installer.Install(packageName, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify install command includes complex version constraints
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})
		})

		Context("when pip is not available", func() {
			BeforeEach(func() {
				// Mock pip not being available by making all pip commands fail
				mockExec.FailingCommands["pip show pandas"] = true
				mockExec.FailingCommands["pip install pandas"] = true
			})

			It("handles pip not being installed on system", func() {
				packageName := "pandas"

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
			})
		})

		Context("concurrent installation scenarios", func() {
			It("handles multiple packages installed in sequence", func() {
				packages := []string{
					"requests",
					"numpy",
					"pandas",
				}

				for _, packageName := range packages {
					err := installer.Install(packageName, mockRepo)
					Expect(err).NotTo(HaveOccurred())
				}

				// Verify all packages were added to repository
				for _, packageName := range packages {
					Expect(mockRepo.AddedApps).To(ContainElement(packageName))
				}

				// Verify all install commands were executed
				for _, packageName := range packages {
					Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
				}
			})

			It("handles mixed installation states correctly", func() {
				// Mark some packages as already installed
				mockExec.InstallationState["requests"] = true
				mockExec.InstallationState["urllib3"] = true

				packages := []string{
					"requests", // Already installed
					"numpy",    // Not installed
					"urllib3",  // Already installed
					"pandas",   // Not installed
				}

				for _, packageName := range packages {
					err := installer.Install(packageName, mockRepo)
					Expect(err).NotTo(HaveOccurred())
				}

				// Only new packages should be added to repository
				Expect(mockRepo.AddedApps).To(ContainElement("numpy"))
				Expect(mockRepo.AddedApps).To(ContainElement("pandas"))
				Expect(mockRepo.AddedApps).ToNot(ContainElement("requests"))
				Expect(mockRepo.AddedApps).ToNot(ContainElement("urllib3"))

				// Only new packages should have install commands executed
				Expect(mockExec.Commands).To(ContainElement("pip install numpy"))
				Expect(mockExec.Commands).To(ContainElement("pip install pandas"))
				Expect(mockExec.Commands).ToNot(ContainElement("pip install requests"))
				Expect(mockExec.Commands).ToNot(ContainElement("pip install urllib3"))
			})
		})

		Context("with realistic package names and scenarios", func() {
			It("installs popular Python packages", func() {
				popularPackages := []string{
					"flask",
					"django",
					"fastapi",
					"sqlalchemy",
					"pytest",
					"black",
					"flake8",
					"mypy",
				}

				for _, packageName := range popularPackages {
					err := installer.Install(packageName, mockRepo)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockRepo.AddedApps).To(ContainElement(packageName))
				}
			})

			It("handles data science package ecosystem", func() {
				dataPackages := []string{
					"numpy",
					"pandas",
					"matplotlib",
					"seaborn",
					"scikit-learn",
					"jupyter",
					"ipython",
				}

				for _, packageName := range dataPackages {
					err := installer.Install(packageName, mockRepo)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockRepo.AddedApps).To(ContainElement(packageName))
				}
			})

			It("handles web development packages", func() {
				webPackages := []string{
					"django",
					"flask",
					"fastapi",
					"gunicorn",
					"celery",
					"redis",
					"psycopg2-binary",
				}

				for _, packageName := range webPackages {
					err := installer.Install(packageName, mockRepo)
					Expect(err).NotTo(HaveOccurred())
					Expect(mockRepo.AddedApps).To(ContainElement(packageName))
				}
			})
		})

		Context("error message validation", func() {
			It("provides helpful error messages for installation failures", func() {
				packageName := "failing-package"
				installCommand := "pip install " + packageName
				mockExec.FailingCommands[installCommand] = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install package via pip"))
				Expect(err.Error()).To(ContainSubstring(packageName))
			})

			It("provides helpful error messages for repository failures", func() {
				packageName := "successful-install"
				mockRepo.ShouldFailAddApp = true

				err := installer.Install(packageName, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to add pip package"))
				Expect(err.Error()).To(ContainSubstring(packageName))
				Expect(err.Error()).To(ContainSubstring("to repository"))
			})

			It("handles installation check failures gracefully", func() {
				packageName := "check-failing-package"
				mockExec.FailingCommands["pip show "+packageName] = true

				err := installer.Install(packageName, mockRepo)

				// Should succeed because pip show failing just means package not installed
				// and the installation command succeeds
				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(packageName))

				// Verify that pip show was called for installation check
				Expect(mockExec.Commands).To(ContainElement("pip show " + packageName))
				// Verify that pip install was called since check indicated not installed
				Expect(mockExec.Commands).To(ContainElement("pip install " + packageName))
			})
		})
	})
})

// MockRepository for testing
type MockRepository struct {
	AddedApps        []string
	ShouldFailAddApp bool
	Apps             []types.AppConfig
}

func (m *MockRepository) AddApp(name string) error {
	if m.ShouldFailAddApp {
		return errors.New("mock repository error")
	}
	m.AddedApps = append(m.AddedApps, name)
	return nil
}

func (m *MockRepository) DeleteApp(name string) error {
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
	return nil
}

func (m *MockRepository) Get(key string) (string, error) {
	return "", nil
}
