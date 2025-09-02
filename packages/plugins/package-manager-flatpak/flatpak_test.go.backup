package flatpak

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Flatpak Installer", func() {
	var (
		installer *FlatpakInstaller
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
		It("creates a new Flatpak installer instance", func() {
			installer := New()
			Expect(installer).ToNot(BeNil())
			Expect(installer).To(BeAssignableToTypeOf(&FlatpakInstaller{}))
		})
	})

	Describe("Version Detection", func() {
		Context("when detecting Flatpak version", func() {
			It("handles version detection failure gracefully", func() {
				mockExec.FailingCommands["flatpak --version"] = true

				// Install should continue even with version detection failure
				_ = installer.Install("org.mozilla.firefox", mockRepo)

				// Will fail due to validation suite, but should have attempted version detection
				Expect(mockExec.Commands).To(ContainElement("flatpak --version"))
			})
		})
	})

	Describe("Install", func() {
		Context("with valid Flatpak application", func() {
			It("installs Flatpak application successfully", func() {
				appID := "org.mozilla.firefox"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify Flatpak installation check was called
				Expect(mockExec.Commands).To(ContainElement("flatpak list --columns=application"))
				// Verify Flatpak install command was called
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})

			It("handles remote:app format correctly", func() {
				appID := "flathub:org.mozilla.firefox"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify correct install command format
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y flathub org.mozilla.firefox"))
			})

			It("installs application with complex ID", func() {
				appID := "org.videolan.VLC"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify commands were executed
				Expect(mockExec.Commands).To(ContainElement("flatpak list --columns=application"))
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})
		})

		Context("when application is already installed", func() {
			BeforeEach(func() {
				// Mark the application as already installed
				mockExec.InstallationState["org.mozilla.firefox"] = true
			})

			It("skips installation if application already installed", func() {
				appID := "org.mozilla.firefox"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				// Repository should not be called for already installed applications
				Expect(mockRepo.AddedApps).To(BeEmpty())

				// Verify that Flatpak installation check was called
				Expect(mockExec.Commands).To(ContainElement("flatpak list --columns=application"))
				// The flatpak install command should NOT be executed
				Expect(mockExec.Commands).ToNot(ContainElement("flatpak install -y " + appID))
			})
		})

		Context("with validation errors", func() {
			It("returns error when app is not available", func() {
				appID := "org.nonexistent.app"
				// Mock both search methods failing
				mockExec.FailingCommands["flatpak search org.nonexistent.app"] = true
				mockExec.FailingCommands["flatpak remote-ls flathub | grep -i org.nonexistent.app"] = true

				err := installer.Install(appID, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("app validation failed"))
			})
		})

		Context("when Flatpak install command fails", func() {
			BeforeEach(func() {
				// Mock Flatpak install command failing
				appID := "org.failing.app"
				installCommand := "flatpak install -y " + appID
				mockExec.FailingCommands[installCommand] = true
			})

			It("returns Flatpak install command execution error", func() {
				appID := "org.failing.app"

				err := installer.Install(appID, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install Flatpak app"))
				Expect(err.Error()).To(ContainSubstring(appID))
				// Repository should not be modified when installation fails
				Expect(mockRepo.AddedApps).To(BeEmpty())
			})
		})

		Context("when repository operations fail", func() {
			BeforeEach(func() {
				mockRepo.ShouldFailAddApp = true
			})

			It("returns repository error after successful installation", func() {
				appID := "org.mozilla.firefox"

				err := installer.Install(appID, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to add Flatpak app"))
				Expect(err.Error()).To(ContainSubstring("to repository"))
				Expect(err.Error()).To(ContainSubstring(appID))

				// Verify that installation was attempted
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})
		})

		Context("with edge cases and input validation", func() {
			It("handles application ID with special characters", func() {
				appID := "org.example.app-with-dashes_and_underscores"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify install command was called with special characters
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})

			It("handles very long application IDs", func() {
				appID := "org.very.long.application.id.with.many.segments.that.should.still.work.properly"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))
			})
		})
	})

	Describe("Uninstall", func() {
		Context("when app is installed", func() {
			BeforeEach(func() {
				mockExec.InstallationState["org.mozilla.firefox"] = true
			})

			It("uninstalls app successfully", func() {
				appID := "org.mozilla.firefox"

				err := installer.Uninstall(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("flatpak uninstall -y org.mozilla.firefox"))
			})

			It("handles remote:app format in uninstall", func() {
				appID := "flathub:org.mozilla.firefox"
				mockExec.InstallationState["org.mozilla.firefox"] = true

				err := installer.Uninstall(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				// Should extract just the app ID for uninstall
				Expect(mockExec.Commands).To(ContainElement("flatpak uninstall -y org.mozilla.firefox"))
			})
		})

		Context("when app is not installed", func() {
			It("skips uninstall gracefully", func() {
				appID := "org.mozilla.firefox"

				err := installer.Uninstall(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				// Should not attempt to uninstall
				Expect(mockExec.Commands).ToNot(ContainElement("flatpak uninstall -y org.mozilla.firefox"))
			})
		})
	})

	Describe("IsInstalled", func() {
		It("checks if app is installed", func() {
			appID := "org.mozilla.firefox"
			mockExec.InstallationState["org.mozilla.firefox"] = true

			installed, err := installer.IsInstalled(appID)

			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
		})

		It("handles remote:app format", func() {
			appID := "flathub:org.mozilla.firefox"
			mockExec.InstallationState["org.mozilla.firefox"] = true

			installed, err := installer.IsInstalled(appID)

			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
		})

		It("returns false when app is not installed", func() {
			appID := "org.notinstalled.app"

			installed, err := installer.IsInstalled(appID)

			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
		})
	})

	Describe("PackageManager Interface", func() {
		Describe("InstallPackages", func() {
			It("installs multiple packages", func() {
				packages := []string{"org.mozilla.firefox", "org.videolan.VLC"}

				err := installer.InstallPackages(ctx, packages, false)

				Expect(err).NotTo(HaveOccurred())
				// Should update metadata first
				Expect(mockExec.Commands).To(ContainElement("flatpak update --appstream"))
				// Should install each package
				for _, pkg := range packages {
					expectedCmd := fmt.Sprintf("flatpak install -y %s", pkg)
					Expect(mockExec.Commands).To(ContainElement(expectedCmd))
				}
			})

			It("handles dry run mode", func() {
				packages := []string{"org.mozilla.firefox"}

				err := installer.InstallPackages(ctx, packages, true)

				Expect(err).NotTo(HaveOccurred())
				// Should not execute any install commands in dry run
				Expect(mockExec.Commands).To(BeEmpty())
			})

			It("continues on individual package failures", func() {
				packages := []string{"org.mozilla.firefox", "org.failing.app", "org.videolan.VLC"}
				mockExec.FailingCommands["flatpak install -y org.failing.app"] = true

				err := installer.InstallPackages(ctx, packages, false)

				// Should not return error for individual failures
				Expect(err).NotTo(HaveOccurred())
				// Should still try to install other packages
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y org.mozilla.firefox"))
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y org.videolan.VLC"))
			})
		})

		Describe("IsAvailable", func() {
			It("returns true when Flatpak is available and initialized", func() {
				// Mock will return success for these commands by default

				available := installer.IsAvailable(ctx)

				Expect(available).To(BeTrue())
				Expect(mockExec.Commands).To(ContainElement("which flatpak"))
				Expect(mockExec.Commands).To(ContainElement("flatpak remotes"))
			})

			It("returns false when Flatpak is not installed", func() {
				mockExec.FailingCommands["which flatpak"] = true

				available := installer.IsAvailable(ctx)

				Expect(available).To(BeFalse())
			})

			It("returns false when Flatpak is not initialized", func() {
				mockExec.FailingCommands["flatpak remotes"] = true

				available := installer.IsAvailable(ctx)

				Expect(available).To(BeFalse())
			})
		})

		Describe("GetName", func() {
			It("returns correct package manager name", func() {
				name := installer.GetName()
				Expect(name).To(Equal("flatpak"))
			})
		})
	})

	Describe("Helper Functions", func() {
		Describe("parseFlatpakCommand", func() {
			It("parses remote:app format", func() {
				remote, appID := parseFlatpakCommand("flathub:org.mozilla.firefox")
				Expect(remote).To(Equal("flathub"))
				Expect(appID).To(Equal("org.mozilla.firefox"))
			})

			It("parses full app ID without remote", func() {
				remote, appID := parseFlatpakCommand("org.mozilla.firefox")
				Expect(remote).To(Equal(""))
				Expect(appID).To(Equal("org.mozilla.firefox"))
			})

			It("handles simple app names", func() {
				remote, appID := parseFlatpakCommand("firefox")
				Expect(remote).To(Equal(""))
				Expect(appID).To(Equal("firefox"))
			})
		})

		Describe("buildFlatpakInstallCommand", func() {
			It("builds command with remote", func() {
				cmd := buildFlatpakInstallCommand("flathub", "org.mozilla.firefox")
				Expect(cmd).To(Equal("flatpak install -y flathub org.mozilla.firefox"))
			})

			It("builds command without remote for full app ID", func() {
				cmd := buildFlatpakInstallCommand("", "org.mozilla.firefox")
				Expect(cmd).To(Equal("flatpak install -y org.mozilla.firefox"))
			})

			It("adds flathub for simple names", func() {
				cmd := buildFlatpakInstallCommand("", "firefox")
				Expect(cmd).To(Equal("flatpak install -y flathub firefox"))
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
