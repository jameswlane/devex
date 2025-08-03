package flatpak

import (
	"errors"

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
		It("creates a new Flatpak installer instance", func() {
			installer := New()
			Expect(installer).ToNot(BeNil())
			Expect(installer).To(BeAssignableToTypeOf(&FlatpakInstaller{}))
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

			It("installs Flatpak application with complex ID", func() {
				appID := "org.videolan.VLC"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify commands were executed
				Expect(mockExec.Commands).To(ContainElement("flatpak list --columns=application"))
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})

			It("installs application with flathub repository specified", func() {
				appID := "flathub org.gimp.GIMP"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify install command includes the full specification
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})
		})

		Context("when application is already installed", func() {
			BeforeEach(func() {
				// Mark the application as already installed in the mock executor
				// This will be returned by "flatpak list --columns=application"
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
				// The flatpak install command should NOT be executed since app already exists
				Expect(mockExec.Commands).ToNot(ContainElement("flatpak install -y " + appID))
			})

			It("handles partial application ID matches correctly", func() {
				// Test that partial matches don't cause false positives
				appID := "org.mozilla.firefox-beta"
				// Only mark the base firefox as installed, not the beta version
				mockExec.InstallationState["org.mozilla.firefox"] = true

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify install command was called for the beta version
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})
		})

		Context("when Flatpak list command fails", func() {
			BeforeEach(func() {
				// Mock Flatpak list command failing - this will make the installer think the app is not installed
				// and try to install it, so we also need to make the install command fail to simulate the scenario
				// where Flatpak itself is not available
				mockExec.FailingCommands["flatpak list --columns=application"] = true
				mockExec.FailingCommands["flatpak install -y org.mozilla.firefox"] = true
			})

			It("returns error when Flatpak is not available on system", func() {
				appID := "org.mozilla.firefox"

				err := installer.Install(appID, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install Flatpak app"))
				// Repository should not be modified when installation fails
				Expect(mockRepo.AddedApps).To(BeEmpty())
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

			It("handles network connection failures", func() {
				appID := "org.network.failing"
				installCommand := "flatpak install -y " + appID
				mockExec.FailingCommands[installCommand] = true

				err := installer.Install(appID, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install Flatpak app"))
			})

			It("handles repository access failures", func() {
				appID := "unknown.repository.app"
				installCommand := "flatpak install -y " + appID
				mockExec.FailingCommands[installCommand] = true

				err := installer.Install(appID, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install Flatpak app"))
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
			It("handles empty application ID", func() {
				appID := ""

				err := installer.Install(appID, mockRepo)

				// With empty app ID, IsAppInstalled returns true (no commands to check)
				// so installer thinks app is already installed and skips installation
				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(BeEmpty())
				// No commands should be executed since it thinks the app is already installed
				Expect(mockExec.Commands).To(BeEmpty())
			})

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

			It("handles application ID with version specification", func() {
				appID := "org.mozilla.firefox//stable"

				err := installer.Install(appID, mockRepo)

				Expect(err).NotTo(HaveOccurred())
				Expect(mockRepo.AddedApps).To(ContainElement(appID))

				// Verify install command includes version specification
				Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
			})
		})

		Context("when Flatpak is not available", func() {
			BeforeEach(func() {
				// Mock Flatpak not being available by making all commands fail
				mockExec.FailingCommands["flatpak list --columns=application"] = true
				mockExec.FailingCommands["flatpak install -y org.mozilla.firefox"] = true
			})

			It("handles Flatpak not being installed on system", func() {
				appID := "org.mozilla.firefox"

				err := installer.Install(appID, mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to install Flatpak app"))
			})
		})

		Context("concurrent installation scenarios", func() {
			It("handles multiple applications installed in sequence", func() {
				apps := []string{
					"org.mozilla.firefox",
					"org.videolan.VLC",
					"org.gimp.GIMP",
				}

				for _, appID := range apps {
					err := installer.Install(appID, mockRepo)
					Expect(err).NotTo(HaveOccurred())
				}

				// Verify all apps were added to repository
				for _, appID := range apps {
					Expect(mockRepo.AddedApps).To(ContainElement(appID))
				}

				// Verify all install commands were executed
				for _, appID := range apps {
					Expect(mockExec.Commands).To(ContainElement("flatpak install -y " + appID))
				}
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
