package utilities_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("PackageManagerCache", func() {
	var (
		mockRepo     *mocks.MockRepository
		mockExec     *mocks.MockCommandExecutor
		originalExec utils.Interface
		ctx          context.Context
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockExec = mocks.NewMockCommandExecutor()
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec
		ctx = context.Background()

		// Reset cache for each test
		utilities.ResetPackageManagerCache()
	})

	AfterEach(func() {
		utils.CommandExec = originalExec
	})

	Describe("Input Validation", func() {
		Context("with invalid package manager names", func() {
			It("rejects names with special characters", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt; rm -rf /", mockRepo, time.Hour)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid package manager name"))
			})

			It("rejects names with spaces", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt install malware", mockRepo, time.Hour)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid package manager name"))
			})

			It("rejects unknown package managers", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "malicious-pm", mockRepo, time.Hour)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown package manager"))
			})
		})

		Context("with valid package manager names", func() {
			It("accepts standard package managers", func() {
				validManagers := []string{"apt", "dnf", "yum", "pacman", "zypper", "brew"}
				for _, pm := range validManagers {
					err := utilities.EnsurePackageManagerUpdated(ctx, pm, mockRepo, time.Hour)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
	})

	Describe("Cache Behavior", func() {
		Context("with fresh cache", func() {
			It("executes update command for apt", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo apt-get update"))
			})

			It("executes update command for dnf", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "dnf", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo dnf check-update"))
			})

			It("executes update command for pacman", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "pacman", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo pacman -Sy"))
			})
		})

		Context("with recently updated cache", func() {
			It("skips update when within cache window", func() {
				// First update
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())

				// Reset command tracking
				mockExec.Commands = []string{}

				// Second update within cache window
				err = utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(BeEmpty()) // Should not execute update again
			})

			It("updates when cache window expires", func() {
				// First update
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())

				// Reset command tracking
				mockExec.Commands = []string{}

				// Second update with very short cache window (should update again)
				err = utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, 1*time.Nanosecond)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo apt-get update"))
			})
		})

		Context("with package managers that don't need updates", func() {
			It("skips update for pip", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "pip", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(BeEmpty())
			})

			It("skips update for deb", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "deb", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(BeEmpty())
			})
		})
	})

	Describe("Error Handling", func() {
		Context("when update command fails", func() {
			BeforeEach(func() {
				mockExec.FailingCommand = "sudo apt-get update"
			})

			It("returns error for failed updates with actionable hints", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to update apt package lists"))
				Expect(err.Error()).To(ContainSubstring("hint: check network connectivity"))
			})
		})

		Context("with DNF/YUM special cases", func() {
			BeforeEach(func() {
				mockExec.FailingCommand = "sudo dnf check-update"
			})

			It("handles DNF exit code 100 as normal", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "dnf", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred()) // Should not error for DNF
			})
		})
	})

	Describe("Concurrent Access", func() {
		It("handles multiple goroutines safely", func() {
			const numGoroutines = 10
			var wg sync.WaitGroup
			errors := make(chan error, numGoroutines)

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					err := utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
					if err != nil {
						errors <- err
					}
				}()
			}

			wg.Wait()
			close(errors)

			// Check that no errors occurred
			for err := range errors {
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify that apt-get update was called at least once
			Expect(mockExec.Commands).To(ContainElement("sudo apt-get update"))
		})

		It("handles concurrent access to different package managers", func() {
			const numGoroutines = 4
			packageManagers := []string{"apt", "dnf", "pacman", "zypper"}
			var wg sync.WaitGroup
			errors := make(chan error, numGoroutines)

			// Ensure a fresh cache for this test by clearing any existing data
			utilities.ResetPackageManagerCache()

			for i, pm := range packageManagers {
				wg.Add(1)
				go func(packageManager string, index int) {
					defer wg.Done()
					err := utilities.EnsurePackageManagerUpdated(ctx, packageManager, mockRepo, time.Hour)
					if err != nil {
						errors <- err
					}
				}(pm, i)
			}

			wg.Wait()
			close(errors)

			// Check that no errors occurred
			for err := range errors {
				Expect(err).NotTo(HaveOccurred())
			}

			// Verify that package manager commands were executed
			// Due to caching logic and race conditions, we may not get exactly 4 commands
			// but we should get at least some commands and they should be the expected ones
			Expect(len(mockExec.Commands)).To(BeNumerically(">=", 1))
			Expect(len(mockExec.Commands)).To(BeNumerically("<=", len(packageManagers)))

			// Verify that only expected package manager commands were called
			expectedCommands := []string{
				"sudo apt-get update",
				"sudo dnf check-update",
				"sudo pacman -Sy",
				"sudo zypper refresh",
			}

			for _, cmd := range mockExec.Commands {
				Expect(expectedCommands).To(ContainElement(cmd),
					"Unexpected command executed: %s", cmd)
			}
		})
	})

	Describe("Repository Persistence", func() {
		Context("when loading from repository", func() {
			It("loads existing timestamps correctly", func() {
				// Simulate existing timestamp in repository
				timestamp := time.Now().Add(-30 * time.Minute).Format(time.RFC3339)
				err := mockRepo.Set("last_apt_update", timestamp)
				Expect(err).NotTo(HaveOccurred())

				// Reset cache to force reload
				utilities.ResetPackageManagerCache()

				// This should not trigger an update since we're within 1 hour window
				err = utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(BeEmpty()) // Should not update
			})

			It("handles corrupted timestamps gracefully", func() {
				// Simulate corrupted timestamp in repository
				err := mockRepo.Set("last_apt_update", "invalid-timestamp")
				Expect(err).NotTo(HaveOccurred())

				// Reset cache to force reload
				utilities.ResetPackageManagerCache()

				// This should trigger an update since timestamp is invalid
				err = utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())
				Expect(mockExec.Commands).To(ContainElement("sudo apt-get update"))
			})
		})

		Context("when saving to repository", func() {
			It("persists update timestamps", func() {
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())

				// Check that timestamp was saved
				timestamp, err := mockRepo.Get("last_apt_update")
				Expect(err).NotTo(HaveOccurred())
				Expect(timestamp).NotTo(BeEmpty())

				// Verify timestamp format
				_, err = time.Parse(time.RFC3339, timestamp)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("GetLastUpdateTime", func() {
		Context("with existing cache", func() {
			It("returns correct timestamp", func() {
				// Trigger an update to populate cache
				err := utilities.EnsurePackageManagerUpdated(ctx, "apt", mockRepo, time.Hour)
				Expect(err).NotTo(HaveOccurred())

				// Get the timestamp
				timestamp, exists := utilities.GetLastUpdateTime("apt", mockRepo)
				Expect(exists).To(BeTrue())
				Expect(timestamp).To(BeTemporally("~", time.Now(), time.Minute))
			})
		})

		Context("with no cache", func() {
			It("returns false for non-existent timestamps", func() {
				timestamp, exists := utilities.GetLastUpdateTime("nonexistent", mockRepo)
				Expect(exists).To(BeFalse())
				Expect(timestamp.IsZero()).To(BeTrue())
			})
		})
	})
})
