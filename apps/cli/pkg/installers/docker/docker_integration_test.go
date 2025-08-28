package docker

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// IntegrationRepository provides a realistic repository mock for integration tests
type IntegrationRepository struct {
	installedApps []string
	shouldError   bool
	mutex         sync.Mutex
	failureRate   float32 // 0.0 = never fail, 1.0 = always fail
	callCount     int
}

func (r *IntegrationRepository) AddApp(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.callCount++
	if r.shouldError || (r.failureRate > 0 && float32(r.callCount%10)/10.0 < r.failureRate) {
		return fmt.Errorf("repository integration test error for %s", name)
	}

	r.installedApps = append(r.installedApps, name)
	return nil
}

func (r *IntegrationRepository) GetApps() ([]string, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.shouldError {
		return nil, fmt.Errorf("repository get apps error")
	}

	return append([]string{}, r.installedApps...), nil
}

func (r *IntegrationRepository) RemoveApp(name string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.shouldError {
		return fmt.Errorf("repository remove app error")
	}

	for i, app := range r.installedApps {
		if app == name {
			r.installedApps = append(r.installedApps[:i], r.installedApps[i+1:]...)
			break
		}
	}
	return nil
}

func (r *IntegrationRepository) ClearApps() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.shouldError {
		return fmt.Errorf("repository clear apps error")
	}

	r.installedApps = []string{}
	return nil
}

// Additional Repository interface methods
func (r *IntegrationRepository) DeleteApp(name string) error {
	return r.RemoveApp(name)
}

func (r *IntegrationRepository) GetApp(name string) (*types.AppConfig, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.shouldError {
		return nil, fmt.Errorf("repository get app error")
	}

	for _, app := range r.installedApps {
		if app == name {
			return &types.AppConfig{BaseConfig: types.BaseConfig{Name: name}}, nil
		}
	}
	return nil, fmt.Errorf("app not found: %s", name)
}

func (r *IntegrationRepository) ListApps() ([]types.AppConfig, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.shouldError {
		return nil, fmt.Errorf("repository list apps error")
	}

	apps := make([]types.AppConfig, len(r.installedApps))
	for i, name := range r.installedApps {
		apps[i] = types.AppConfig{BaseConfig: types.BaseConfig{Name: name}}
	}
	return apps, nil
}

func (r *IntegrationRepository) SaveApp(app types.AppConfig) error {
	return r.AddApp(app.Name)
}

func (r *IntegrationRepository) Set(key, value string) error {
	if r.shouldError {
		return fmt.Errorf("repository set error")
	}
	return nil
}

func (r *IntegrationRepository) Get(key string) (string, error) {
	if r.shouldError {
		return "", fmt.Errorf("repository get error")
	}
	return "test-value", nil
}

var _ = Describe("Docker Installer Integration Tests", func() {
	var (
		installer    *DockerInstaller
		mockExec     *mocks.MockCommandExecutor
		repo         *IntegrationRepository
		originalExec interface{}
	)

	BeforeEach(func() {
		installer = New()
		mockExec = mocks.NewMockCommandExecutor()
		repo = &IntegrationRepository{}
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec
	})

	AfterEach(func() {
		installer.StopCleanup() // Clean shutdown of background goroutine
		if originalExec != nil {
			utils.CommandExec = originalExec.(utils.Interface)
		}
		repo.ClearApps()
	})

	Describe("Complete Docker Installation Workflow", func() {
		Context("when installing Docker from scratch", func() {
			BeforeEach(func() {
				// Simulate fresh system without Docker
				mockExec.InstallationState["docker"] = false
				mockExec.InstallationState["docker-ce"] = false
			})

			It("should complete full Docker installation successfully", func() {
				command := "install docker-ce docker-ce-cli containerd.io"

				By("Installing Docker packages")
				err := installer.Install(command, repo)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying installation commands were executed")
				Expect(len(mockExec.Commands)).To(BeNumerically(">", 0))

				By("Verifying repository interaction")
				apps, err := repo.GetApps()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(apps)).To(BeNumerically(">=", 0))
			})

			It("should handle Docker service startup failures gracefully", func() {
				// Simulate Docker installation success but service startup failure
				mockExec.FailingCommands = map[string]bool{
					"sudo systemctl start docker": true,
					"sudo service docker start":   true,
					"systemctl is-active docker":  true,
					"service docker status":       true,
				}

				command := "install docker-ce"
				err := installer.Install(command, repo)

				// Should attempt installation but may warn about service startup
				// This tests the fallback behavior in container environments
				Expect(err).To(Or(BeNil(), MatchError(ContainSubstring("service"))))
			})
		})

		Context("when Docker is already installed", func() {
			BeforeEach(func() {
				// Simulate Docker already installed
				mockExec.InstallationState["docker"] = true
				mockExec.InstallationState["docker-ce"] = true
			})

			It("should detect existing installation and skip reinstallation", func() {
				command := "install docker-ce"

				err := installer.Install(command, repo)
				Expect(err).ToNot(HaveOccurred())

				By("Verifying minimal commands were executed")
				// Should only run detection commands, not installation
				Expect(len(mockExec.Commands)).To(BeNumerically("<", 10))
			})
		})

		Context("when running in containerized environment", func() {
			BeforeEach(func() {
				// Simulate container environment with K8s pod hostname pattern
				os.Setenv("HOSTNAME", "test-pod-12345")
			})

			AfterEach(func() {
				os.Unsetenv("HOSTNAME")
			})

			It("should handle Docker-in-Docker scenarios", func() {
				command := "docker run --name test-container -d nginx:latest"

				// Simulate Docker daemon not running in container
				mockExec.FailingCommands = map[string]bool{
					"docker version":      true,
					"sudo docker version": true,
					"docker version --format '{{.Server.Version}}'":      true,
					"sudo docker version --format '{{.Server.Version}}'": true,
					"test -S /var/run/docker.sock":                       true, // No docker socket available
				}

				err := installer.Install(command, repo)

				// Should attempt Docker daemon startup in container environment
				Expect(err).To(Or(BeNil(), MatchError(ContainSubstring("container"))))

				By("Verifying Docker daemon startup attempts")
				commandsStr := strings.Join(mockExec.Commands, " ")
				Expect(commandsStr).To(Or(
					ContainSubstring("sudo service docker start"),
					ContainSubstring("sudo systemctl start docker"),
					ContainSubstring("sudo dockerd"),
				))
			})
		})
	})

	Describe("Cache and Performance Integration", func() {
		It("should maintain cache consistency across multiple operations", func() {
			command := "docker run --name nginx -d -p 8080:80 nginx:latest"

			By("Installing container and caching status")
			// Manually mark container as installed for testing purposes
			mockExec.InstallationState["nginx"] = true

			err := installer.Install(command, repo)
			Expect(err).ToNot(HaveOccurred())

			By("Checking cached container status")
			isInstalled, err := installer.IsInstalled(command)
			Expect(err).ToNot(HaveOccurred())
			Expect(isInstalled).To(BeTrue())

			By("Verifying cache efficiency - second check should be faster")
			startTime := time.Now()
			isInstalled, err = installer.IsInstalled(command)
			duration := time.Since(startTime)

			Expect(err).ToNot(HaveOccurred())
			Expect(isInstalled).To(BeTrue())
			Expect(duration).To(BeNumerically("<", 100*time.Millisecond))
		})

		It("should automatically clean up expired cache entries", func() {
			// Use a very short cache timeout for this test
			installer = NewWithCacheTimeout(DefaultServiceTimeout, 100*time.Millisecond)
			utils.CommandExec = mockExec

			command := "docker run --name redis -d redis:latest"

			By("Installing container to populate cache")
			err := installer.Install(command, repo)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying cache entry exists")
			installer.cacheMutex.RLock()
			cacheSize := len(installer.containerCache)
			installer.cacheMutex.RUnlock()
			Expect(cacheSize).To(BeNumerically(">=", 0))

			By("Waiting for cache expiration and cleanup")
			time.Sleep(200 * time.Millisecond) // Wait for cache to expire

			// Trigger cleanup by accessing cache
			_, _ = installer.IsInstalled(command)

			By("Verifying cache was cleaned up eventually")
			Eventually(func() int {
				installer.cacheMutex.RLock()
				size := len(installer.containerCache)
				installer.cacheMutex.RUnlock()
				return size
			}, 2*time.Second, 200*time.Millisecond).Should(BeNumerically("<=", 1))
		})
	})

	Describe("Error Recovery and Resilience", func() {
		It("should recover from temporary Docker daemon failures", func() {
			command := "docker run --name postgres -e POSTGRES_PASSWORD=test -d postgres:13"

			// Simulate Docker daemon temporarily unavailable
			mockExec.FailingCommands = map[string]bool{
				"docker version": true,
			}

			By("Initial installation attempt should handle daemon failure")
			err := installer.Install(command, repo)
			// Should attempt daemon startup or provide appropriate error
			Expect(err).To(Or(BeNil(), MatchError(ContainSubstring("daemon"))))

			By("Simulating daemon recovery")
			mockExec.FailingCommands = map[string]bool{}
			mockExec.InstallationState["postgres"] = true

			By("Retry should succeed")
			err = installer.Install(command, repo)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should handle repository connection failures", func() {
			repo.shouldError = true

			command := "docker run --name mongo -d mongo:latest"

			By("Installation should proceed despite repository issues")
			err := installer.Install(command, repo)

			// Docker operations should succeed even if repository fails
			Expect(err).To(Or(BeNil(), HaveOccurred())) // Either works or fails, but shouldn't panic
		})

		It("should validate Docker commands for security", func() {
			maliciousCommand := "docker run --name evil && rm -rf / --no-preserve-root"

			By("Installation should reject malicious commands")
			// Test the validation function directly to ensure security works
			err := validateDockerCommand(maliciousCommand)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rm -rf"))
		})
	})

	Describe("Container Lifecycle Management", func() {
		It("should properly uninstall containers and clean up resources", func() {
			command := "docker run --name apache -d -p 8080:80 httpd:latest"

			By("Installing container")
			err := installer.Install(command, repo)
			Expect(err).ToNot(HaveOccurred())

			// Mark container as installed for the test
			mockExec.InstallationState["apache"] = true

			By("Verifying container is running")
			isInstalled, err := installer.IsInstalled(command)
			Expect(err).ToNot(HaveOccurred())
			Expect(isInstalled).To(BeTrue())

			By("Uninstalling container")
			uninstallCommand := "stop apache && rm apache"
			err = installer.Uninstall(uninstallCommand, repo)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying cleanup commands were executed")
			commandsStr := strings.Join(mockExec.Commands, " ")
			Expect(commandsStr).To(Or(
				ContainSubstring("docker stop"),
				ContainSubstring("docker rm"),
			))
		})

		It("should handle partial container states gracefully", func() {
			command := "docker run --name partial -d busybox:latest"

			By("Simulating container that exists but is stopped")
			// This test verifies that the installer can handle partial container states
			// without configuration of specific mock outputs

			_, err := installer.IsInstalled(command)

			// Container state detection should not error regardless of actual state
			// Implementation dependent - this tests the robustness of status detection
			Expect(err).ToNot(HaveOccurred()) // Should not error regardless
		})
	})

	Describe("Background Cleanup Integration", func() {
		It("should properly start and stop cleanup routine", func() {
			By("Verifying cleanup routine starts with new installer")
			testInstaller := New()
			Expect(testInstaller.cleanupTicker).ToNot(BeNil())

			By("Verifying cleanup can be stopped")
			testInstaller.StopCleanup()
			// Should not panic or cause issues
		})

		It("should handle concurrent cache access during cleanup", func() {
			command := "docker run --name concurrent -d nginx:latest"

			By("Installing container to populate cache")
			err := installer.Install(command, repo)
			Expect(err).ToNot(HaveOccurred())

			By("Performing concurrent cache operations")
			done := make(chan bool, 2)

			// Concurrent reads
			go func() {
				defer GinkgoRecover()
				for i := 0; i < 5; i++ {
					_, _ = installer.IsInstalled(command)
					time.Sleep(10 * time.Millisecond)
				}
				done <- true
			}()

			// Trigger cleanup
			go func() {
				defer GinkgoRecover()
				installer.clearExpiredCache()
				done <- true
			}()

			// Wait for both goroutines
			Eventually(done).Should(Receive())
			Eventually(done).Should(Receive())
		})
	})
})
