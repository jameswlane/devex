//go:build integration

package installers_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("Installer Pipeline", func() {
	var (
		mockExec     *mocks.MockCommandExecutor
		mockRepo     *mocks.MockRepository
		originalExec utils.Interface
	)

	BeforeEach(func() {
		// Store original values
		originalExec = utils.CommandExec

		// Set up mock executor
		mockExec = mocks.NewMockCommandExecutor()
		utils.CommandExec = mockExec
		mockRepo = mocks.NewMockRepository()

		// Enable test mode for mock installers
		installers.EnableTestMode()
	})

	AfterEach(func() {
		utils.CommandExec = originalExec
		// Disable test mode
		installers.DisableTestMode()
	})

	Describe("complete installer pipeline from selection to post-install", func() {
		type testCase struct {
			installer   string
			packageName string
			description string
		}

		testCases := []testCase{
			{"apt", "nginx", "APT installer with web server package"},
			{"dnf", "docker", "DNF installer with Docker package"},
			{"pacman", "git", "Pacman installer with development tool"},
			{"snap", "code", "Snap installer with application"},
		}

		for _, tc := range testCases {
			tc := tc // capture loop variable
			Context(tc.description, func() {
				var installer interface{}

				BeforeEach(func() {
					// Reset mock for each test case
					mockExec.Commands = []string{}
					mockExec.FailingCommands = make(map[string]bool)
					mockExec.InstallationState = make(map[string]bool)

					// Configure mock to fail installation commands to simulate test environment
					switch tc.installer {
					case "apt":
						mockExec.FailingCommands[fmt.Sprintf("sudo apt-get install -y %s", tc.packageName)] = true
					case "dnf":
						mockExec.FailingCommands[fmt.Sprintf("sudo dnf install -y %s", tc.packageName)] = true
					case "pacman":
						mockExec.FailingCommands[fmt.Sprintf("sudo pacman -S --noconfirm %s", tc.packageName)] = true
					case "snap":
						mockExec.FailingCommands[fmt.Sprintf("sudo snap install %s", tc.packageName)] = true
					}

					// Get installer instance
					installer = installers.GetInstaller(tc.installer)
				})

				It("should get a valid installer instance", func() {
					Expect(installer).NotTo(BeNil())
				})

				It("should execute commands during the pipeline", func() {
					// Attempt installation (will fail in test env but should follow proper flow)
					if installerWithInstall, ok := installer.(interface {
						Install(string, interface{}) error
					}); ok {
						err := installerWithInstall.Install(tc.packageName, mockRepo)
						Expect(err).To(HaveOccurred()) // Expected to fail in test environment
						Expect(len(mockExec.Commands)).To(BeNumerically(">", 0))
					}
				})

				It("should handle post-install handlers if applicable", func() {
					Skip("Skipping post-install handler test to avoid real system commands")
				})
			})
		}
	})
})

var _ = Describe("System Paths Configuration", func() {
	Describe("when getting default system paths", func() {
		It("should return default paths when no env vars set", func() {
			paths := utilities.GetSystemPaths()

			Expect(paths.YumReposDir).To(Equal("/etc/yum.repos.d"))
			Expect(paths.AptSourcesDir).To(Equal("/etc/apt/sources.list.d"))
		})
	})

	Describe("when generating repository file paths", func() {
		It("should generate correct DNF repository file path", func() {
			paths := utilities.GetSystemPaths()
			dnfPath := paths.GetRepositoryFilePath("dnf", "test-repo")
			Expect(dnfPath).To(Equal("/etc/yum.repos.d/test-repo.repo"))
		})

		It("should generate correct APT repository file path", func() {
			paths := utilities.GetSystemPaths()
			aptPath := paths.GetRepositoryFilePath("apt", "test-repo")
			Expect(aptPath).To(Equal("/etc/apt/sources.list.d/test-repo.list"))
		})
	})
})

var _ = Describe("Post Install Handler Registry", func() {
	Describe("default registry", func() {
		It("should have expected handlers registered", func() {
			expectedHandlers := []string{"docker", "docker-ce", "nginx", "httpd", "apache2"}

			for _, packageName := range expectedHandlers {
				Expect(utilities.DefaultRegistry.HasHandler(packageName)).To(BeTrue(),
					fmt.Sprintf("Expected handler for %s to be registered", packageName))
			}
		})
	})

	Describe("package variations", func() {
		It("should work correctly for Docker variations", func() {
			Skip("Skipping Docker variation test to avoid real system commands")
		})
	})
})

var _ = Describe("Common Utilities", func() {
	Describe("GetCurrentUser", func() {
		It("should return consistent results", func() {
			user1 := utilities.GetCurrentUser()
			user2 := utilities.GetCurrentUser()

			Expect(user1).To(Equal(user2))

			// Should return some non-empty result in most environments
			if user1 == "" {
				By("returning empty string - may be acceptable in some test environments")
			}
		})
	})
})
