package docker

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Docker Installer Caching", func() {
	var (
		installer *DockerInstaller
		mockExec  *mocks.MockCommandExecutor
		mockRepo  *MockRepository
	)

	BeforeEach(func() {
		installer = NewWithCacheTimeout(30*time.Second, 5*time.Minute)
		mockExec = mocks.NewMockCommandExecutor()
		mockRepo = &MockRepository{}
		utils.CommandExec = mockExec
	})

	AfterEach(func() {
		mockExec.Commands = []string{}
		mockExec.FailingCommand = ""
		mockExec.FailingCommands = make(map[string]bool)
		mockExec.InstallationState = make(map[string]bool)
	})

	Describe("Container Status Caching", func() {
		Context("with cache hits", func() {
			It("returns cached status for repeated checks", func() {
				containerName := "test-container"

				// First call should miss cache and check container status
				isRunning1, err1 := installer.checkContainerStatus(containerName)
				Expect(err1).NotTo(HaveOccurred())
				Expect(isRunning1).To(BeFalse()) // Default mock state is not running

				// Record number of commands executed in first call
				firstCallCommandCount := len(mockExec.Commands)
				Expect(firstCallCommandCount).To(BeNumerically(">", 0))

				// Second call should hit cache and not execute any new Docker commands
				isRunning2, err2 := installer.checkContainerStatus(containerName)
				Expect(err2).NotTo(HaveOccurred())
				Expect(isRunning2).To(Equal(isRunning1))

				// Verify no additional commands were executed (cache hit)
				Expect(len(mockExec.Commands)).To(Equal(firstCallCommandCount))
			})

			It("returns cached status in IsInstalled method", func() {
				command := "docker run --name cached-container -d nginx:latest"

				// First call should miss cache
				isInstalled1, err1 := installer.IsInstalled(command)
				Expect(err1).NotTo(HaveOccurred())

				firstCallCommandCount := len(mockExec.Commands)

				// Second call should hit cache
				isInstalled2, err2 := installer.IsInstalled(command)
				Expect(err2).NotTo(HaveOccurred())
				Expect(isInstalled2).To(Equal(isInstalled1))

				// No new commands should be executed
				Expect(len(mockExec.Commands)).To(Equal(firstCallCommandCount))
			})
		})

		Context("with different containers", func() {
			It("caches status per container independently", func() {
				container1 := "container-1"
				container2 := "container-2"

				// Check both containers initially
				status1, err1 := installer.checkContainerStatus(container1)
				Expect(err1).NotTo(HaveOccurred())

				status2, err2 := installer.checkContainerStatus(container2)
				Expect(err2).NotTo(HaveOccurred())

				// Record command count after initial checks
				initialCommandCount := len(mockExec.Commands)

				// Check both containers again - should hit cache for both
				cachedStatus1, err3 := installer.checkContainerStatus(container1)
				Expect(err3).NotTo(HaveOccurred())
				Expect(cachedStatus1).To(Equal(status1))

				cachedStatus2, err4 := installer.checkContainerStatus(container2)
				Expect(err4).NotTo(HaveOccurred())
				Expect(cachedStatus2).To(Equal(status2))

				// No new commands should be executed
				Expect(len(mockExec.Commands)).To(Equal(initialCommandCount))
			})
		})

		Context("with cache expiration", func() {
			It("refreshes cache after timeout", func() {
				// Create installer with very short cache timeout
				shortCacheInstaller := NewWithCacheTimeout(30*time.Second, 10*time.Millisecond)
				containerName := "short-cache-container"

				// First call
				_, err1 := shortCacheInstaller.checkContainerStatus(containerName)
				Expect(err1).NotTo(HaveOccurred())

				firstCallCommandCount := len(mockExec.Commands)

				// Wait for cache to expire
				time.Sleep(15 * time.Millisecond)

				// Second call should miss cache due to expiration
				_, err2 := shortCacheInstaller.checkContainerStatus(containerName)
				Expect(err2).NotTo(HaveOccurred())
				// Status should be same but freshly fetched from Docker API

				// New commands should be executed due to cache miss
				Expect(len(mockExec.Commands)).To(BeNumerically(">", firstCallCommandCount))
			})
		})
	})

	Describe("Cache Management", func() {
		Context("cache clearing", func() {
			It("clears cache after successful installation", func() {
				command := "docker run --name install-test -d nginx:latest"
				containerName := "install-test"

				// Pre-populate cache with "not running" status
				_, err1 := installer.checkContainerStatus(containerName)
				Expect(err1).NotTo(HaveOccurred())
				// Verify cache was populated

				// Install container (this should clear cache)
				err := installer.Install(command, mockRepo)
				Expect(err).NotTo(HaveOccurred())

				// Verify cache was cleared by checking if next call executes new commands
				commandCountBeforeCheck := len(mockExec.Commands)

				// This should be a cache miss since cache was cleared after installation
				_, err2 := installer.checkContainerStatus(containerName)
				Expect(err2).NotTo(HaveOccurred())

				// Verify new commands were executed (cache was cleared)
				Expect(len(mockExec.Commands)).To(BeNumerically(">", commandCountBeforeCheck))
			})

			It("clears cache after successful uninstallation", func() {
				command := "docker run --name uninstall-test -d nginx:latest"
				containerName := "uninstall-test"

				// Set up mock to indicate container is running
				mockExec.InstallationState[containerName] = true

				// Pre-populate cache
				_, err1 := installer.checkContainerStatus(containerName)
				Expect(err1).NotTo(HaveOccurred())

				// Uninstall container (this should clear cache)
				err := installer.Uninstall(command, mockRepo)
				Expect(err).NotTo(HaveOccurred())

				// Verify cache was cleared
				commandCountBeforeCheck := len(mockExec.Commands)

				_, err2 := installer.checkContainerStatus(containerName)
				Expect(err2).NotTo(HaveOccurred())

				// Verify new commands were executed (cache was cleared)
				Expect(len(mockExec.Commands)).To(BeNumerically(">", commandCountBeforeCheck))
			})
		})

		Context("expired cache cleanup", func() {
			It("removes expired entries during cleanup", func() {
				// Create installer with very short cache timeout
				shortCacheInstaller := NewWithCacheTimeout(30*time.Second, 10*time.Millisecond)

				// Add some cache entries
				containerName := "expire-test"
				_, err := shortCacheInstaller.checkContainerStatus(containerName)
				Expect(err).NotTo(HaveOccurred())

				// Verify cache entry exists
				_, valid := shortCacheInstaller.getCachedStatus(containerName)
				Expect(valid).To(BeTrue())

				// Wait for cache to expire
				time.Sleep(15 * time.Millisecond)

				// Cleanup expired entries
				shortCacheInstaller.clearExpiredCache()

				// Verify cache entry was removed
				_, valid = shortCacheInstaller.getCachedStatus(containerName)
				Expect(valid).To(BeFalse())
			})
		})
	})

	Describe("Cache Configuration", func() {
		Context("constructor options", func() {
			It("creates installer with default cache timeout", func() {
				defaultInstaller := New()
				Expect(defaultInstaller).NotTo(BeNil())
				Expect(defaultInstaller.cacheTimeout).To(Equal(ContainerCacheTimeout))
			})

			It("creates installer with custom service timeout", func() {
				customTimeout := 45 * time.Second
				customInstaller := NewWithTimeout(customTimeout)
				Expect(customInstaller).NotTo(BeNil())
				Expect(customInstaller.ServiceTimeout).To(Equal(customTimeout))
				Expect(customInstaller.cacheTimeout).To(Equal(ContainerCacheTimeout))
			})

			It("creates installer with custom cache timeout", func() {
				serviceTimeout := 30 * time.Second
				cacheTimeout := 2 * time.Minute
				customInstaller := NewWithCacheTimeout(serviceTimeout, cacheTimeout)
				Expect(customInstaller).NotTo(BeNil())
				Expect(customInstaller.ServiceTimeout).To(Equal(serviceTimeout))
				Expect(customInstaller.cacheTimeout).To(Equal(cacheTimeout))
			})
		})
	})

	Describe("Thread Safety", func() {
		Context("concurrent cache access", func() {
			It("handles concurrent reads safely", func() {
				containerName := "concurrent-test"

				// Pre-populate cache
				_, err := installer.checkContainerStatus(containerName)
				Expect(err).NotTo(HaveOccurred())

				// Simulate concurrent reads
				done := make(chan bool, 10)
				for i := 0; i < 10; i++ {
					go func() {
						defer GinkgoRecover()
						cached, valid := installer.getCachedStatus(containerName)
						Expect(valid).To(BeTrue())
						Expect(cached).To(BeFalse()) // Default mock state
						done <- true
					}()
				}

				// Wait for all goroutines to complete
				for i := 0; i < 10; i++ {
					<-done
				}
			})

			It("handles concurrent cache operations safely", func() {
				containerNames := []string{"concurrent-1", "concurrent-2", "concurrent-3"}

				// Simulate concurrent cache operations
				done := make(chan bool, len(containerNames)*2)

				// Concurrent cache sets
				for _, name := range containerNames {
					go func(containerName string) {
						defer GinkgoRecover()
						installer.setCachedStatus(containerName, true)
						done <- true
					}(name)
				}

				// Concurrent cache gets
				for _, name := range containerNames {
					go func(containerName string) {
						defer GinkgoRecover()
						// Give sets time to complete
						time.Sleep(1 * time.Millisecond)
						cached, valid := installer.getCachedStatus(containerName)
						if valid {
							Expect(cached).To(BeTrue())
						}
						done <- true
					}(name)
				}

				// Wait for all operations to complete
				for i := 0; i < len(containerNames)*2; i++ {
					<-done
				}
			})
		})
	})
})
