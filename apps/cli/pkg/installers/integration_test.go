package installers_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

// TestInstallerPipeline tests the complete installer pipeline from selection to post-install
func TestInstallerPipeline(t *testing.T) {
	// Store original values
	originalExec := utils.CommandExec
	defer func() {
		utils.CommandExec = originalExec
	}()

	// Set up mock executor
	mockExec := mocks.NewMockCommandExecutor()
	utils.CommandExec = mockExec
	mockRepo := mocks.NewMockRepository()

	// Test pipeline for different installers and packages
	testCases := []struct {
		installer   string
		packageName string
		description string
	}{
		{"apt", "nginx", "APT installer with web server package"},
		{"dnf", "docker", "DNF installer with Docker package"},
		{"pacman", "git", "Pacman installer with development tool"},
		{"snap", "code", "Snap installer with application"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Reset mock for each test case
			mockExec.Commands = []string{}
			mockExec.FailingCommands = make(map[string]bool)

			// Get installer instance
			installer := installers.GetInstaller(tc.installer)
			if installer == nil {
				t.Fatalf("Failed to get installer for %s", tc.installer)
			}

			// Test the complete pipeline:
			// 1. Check if package is installed (should be false initially)
			installed, err := installer.IsInstalled(tc.packageName)
			if err != nil {
				t.Logf("IsInstalled check returned error (expected for some installers): %v", err)
			} else if installed {
				t.Logf("Package %s already installed", tc.packageName)
			}

			// 2. Attempt installation (will fail in test env but should follow proper flow)
			err = installer.Install(tc.packageName, mockRepo)
			if err == nil {
				t.Error("Expected installation to fail in test environment")
			} else {
				t.Logf("Installation failed as expected in test env: %v", err)
			}

			// 3. Verify some commands were executed during the pipeline
			if len(mockExec.Commands) == 0 {
				t.Error("Expected some commands to be executed during installer pipeline")
			} else {
				t.Logf("Commands executed during pipeline: %d", len(mockExec.Commands))
			}

			// 4. Test post-install handler execution (if applicable)
			if utilities.DefaultRegistry.HasHandler(tc.packageName) {
				err = utilities.ExecutePostInstallHandler(tc.packageName)
				if err != nil {
					t.Logf("Post-install handler failed as expected in test env: %v", err)
				}
			}
		})
	}
}

// TestSystemPathsConfiguration tests the configurable paths system
func TestSystemPathsConfiguration(t *testing.T) {
	t.Run("returns default paths when no env vars set", func(t *testing.T) {
		paths := utilities.GetSystemPaths()

		if paths.YumReposDir != "/etc/yum.repos.d" {
			t.Errorf("Expected default YUM repos dir, got %s", paths.YumReposDir)
		}

		if paths.AptSourcesDir != "/etc/apt/sources.list.d" {
			t.Errorf("Expected default APT sources dir, got %s", paths.AptSourcesDir)
		}
	})

	t.Run("generates correct repository file paths", func(t *testing.T) {
		paths := utilities.GetSystemPaths()

		dnfPath := paths.GetRepositoryFilePath("dnf", "test-repo")
		expectedDnfPath := "/etc/yum.repos.d/test-repo.repo"
		if dnfPath != expectedDnfPath {
			t.Errorf("Expected %s, got %s", expectedDnfPath, dnfPath)
		}

		aptPath := paths.GetRepositoryFilePath("apt", "test-repo")
		expectedAptPath := "/etc/apt/sources.list.d/test-repo.list"
		if aptPath != expectedAptPath {
			t.Errorf("Expected %s, got %s", expectedAptPath, aptPath)
		}
	})
}

// TestPostInstallHandlerRegistry tests the handler registry system
func TestPostInstallHandlerRegistry(t *testing.T) {
	t.Run("default registry has expected handlers", func(t *testing.T) {
		expectedHandlers := []string{"docker", "docker-ce", "nginx", "httpd", "apache2"}

		for _, packageName := range expectedHandlers {
			if !utilities.DefaultRegistry.HasHandler(packageName) {
				t.Errorf("Expected handler for %s to be registered", packageName)
			}
		}
	})

	t.Run("package variations work correctly", func(t *testing.T) {
		// Test Docker variations
		dockerVariations := []string{"docker", "docker-ce", "docker.io"}
		for _, variation := range dockerVariations {
			err := utilities.ExecutePostInstallHandler(variation)
			if err != nil {
				t.Logf("Handler for %s failed in test environment: %v", variation, err)
			}
		}
	})
}

// TestCommonUtilities tests the extracted common utilities
func TestCommonUtilities(t *testing.T) {
	t.Run("GetCurrentUser returns consistent results", func(t *testing.T) {
		user1 := utilities.GetCurrentUser()
		user2 := utilities.GetCurrentUser()

		if user1 != user2 {
			t.Error("GetCurrentUser should return consistent results")
		}

		// Should return some non-empty result in most environments
		if user1 == "" {
			t.Log("GetCurrentUser returned empty string - may be acceptable in some test environments")
		}
	})
}
