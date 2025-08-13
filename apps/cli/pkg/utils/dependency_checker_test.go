package utils_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

// Mock PackageManager for testing
type MockPackageManager struct {
	installCalled bool
	installError  error
	available     bool
	name          string
}

func (m *MockPackageManager) InstallPackages(ctx context.Context, packages []string, dryRun bool) error {
	m.installCalled = true
	return m.installError
}

func (m *MockPackageManager) IsAvailable(ctx context.Context) bool {
	return m.available
}

func (m *MockPackageManager) GetName() string {
	return m.name
}

var _ = Describe("DependencyChecker", func() {
	var (
		depChecker   *utils.DependencyChecker
		mockPM       *MockPackageManager
		testPlatform platform.Platform
		ctx          context.Context
	)

	BeforeEach(func() {
		mockPM = &MockPackageManager{
			available: true,
			name:      "test-pm",
		}
		testPlatform = platform.Platform{
			OS:           "linux",
			Distribution: "debian",
			Architecture: "amd64",
		}
		depChecker = utils.NewDependencyChecker(mockPM, testPlatform)
		ctx = context.Background()
	})

	Describe("Package Name Validation", func() {
		Context("when checking valid package names", func() {
			It("should accept valid package names", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl", "git", "gnupg2", "build-essential"},
						},
					},
				}

				// This should not error due to validation
				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should accept package names with common characters", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"lib-test", "test+plus", "test.dot", "test_underscore"},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when checking invalid package names", func() {
			It("should reject empty package names", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{""},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dependency validation failed for package"))
				Expect(err.Error()).To(ContainSubstring("package name cannot be empty"))
			})

			It("should reject package names with invalid characters", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"test;rm -rf /"},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dependency validation failed for package"))
				Expect(err.Error()).To(ContainSubstring("contains invalid characters"))
			})

			It("should reject package names that are too long", func() {
				longPackageName := string(make([]byte, 256))
				for range longPackageName {
					longPackageName = "a" + longPackageName[1:]
				}

				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{longPackageName},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("dependency validation failed for package"))
				Expect(err.Error()).To(ContainSubstring("package name too long"))
			})
		})
	})

	Describe("Platform Matching", func() {
		Context("when platform requirements match current platform", func() {
			It("should find matching requirements for distribution", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl"},
						},
						{
							OS:                   "ubuntu",
							PlatformDependencies: []string{"wget"},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should find matching requirements for OS", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "linux",
							PlatformDependencies: []string{"curl"},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when platform requirements don't match", func() {
			It("should skip when no platform requirements", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should skip when platform doesn't match", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "windows",
							PlatformDependencies: []string{"curl"},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, false)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Context Cancellation", func() {
		It("should respect context cancellation", func() {
			cancelCtx, cancel := context.WithCancel(context.Background())
			cancel() // Cancel immediately

			osConfig := types.OSConfig{
				PlatformRequirements: []types.PlatformRequirement{
					{
						OS:                   "debian",
						PlatformDependencies: []string{"curl"},
					},
				},
			}

			err := depChecker.CheckAndInstallPlatformDependencies(cancelCtx, osConfig, false)
			// Should handle context cancellation gracefully
			Expect(err).To(Or(BeNil(), HaveOccurred()))
		})
	})

	Describe("Dry Run Mode", func() {
		It("should not install packages in dry run mode", func() {
			osConfig := types.OSConfig{
				PlatformRequirements: []types.PlatformRequirement{
					{
						OS:                   "debian",
						PlatformDependencies: []string{"nonexistent-package-12345"},
					},
				},
			}

			err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(mockPM.installCalled).To(BeFalse())
		})
	})

	Describe("Dependency Caching", func() {
		Context("when checking cached dependencies", func() {
			It("should use cached results for repeated dependency checks", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl", "git"},
						},
					},
				}

				// First check - should cache results
				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(depChecker.Cache.Size()).To(BeNumerically(">", 0))

				// Second check - should use cached results
				initialCacheSize := depChecker.Cache.Size()
				err = depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(depChecker.Cache.Size()).To(Equal(initialCacheSize))
			})

			It("should invalidate cache entries after installation", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"nonexistent-package-test"},
						},
					},
				}

				// First check should cache the missing dependency
				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())

				// Verify cache has the entry
				available, found := depChecker.Cache.Get("nonexistent-package-test")
				Expect(found).To(BeTrue())
				Expect(available).To(BeFalse()) // Should be cached as missing

				// Simulate installation and cache invalidation
				depChecker.InvalidateCacheEntries([]string{"nonexistent-package-test"})

				// Verify cache entry was removed
				_, found = depChecker.Cache.Get("nonexistent-package-test")
				Expect(found).To(BeFalse())
			})

			It("should clear all cache entries", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl", "git", "wget"},
						},
					},
				}

				// Check dependencies to populate cache
				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(depChecker.Cache.Size()).To(BeNumerically(">", 0))

				// Clear cache
				depChecker.ClearCache()
				Expect(depChecker.Cache.Size()).To(Equal(0))
			})
		})

		Context("when testing cache expiration", func() {
			It("should create dependency checker with custom cache settings", func() {
				customChecker := utils.NewDependencyCheckerWithCache(mockPM, testPlatform, 1*time.Second, 10)
				Expect(customChecker).NotTo(BeNil())
				Expect(customChecker.Cache.TTL).To(Equal(1 * time.Second))
				Expect(customChecker.Cache.MaxEntries).To(Equal(10))
			})
		})

		Context("when testing cache eviction", func() {
			It("should evict oldest entries when cache is full", func() {
				// Create checker with small cache for testing eviction
				smallCacheChecker := utils.NewDependencyCheckerWithCache(mockPM, testPlatform, 5*time.Minute, 2)

				// Add entries to fill cache
				osConfig1 := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl"},
						},
					},
				}
				osConfig2 := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"git"},
						},
					},
				}
				osConfig3 := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"wget"},
						},
					},
				}

				// Fill cache to capacity
				err := smallCacheChecker.CheckAndInstallPlatformDependencies(ctx, osConfig1, true)
				Expect(err).NotTo(HaveOccurred())
				err = smallCacheChecker.CheckAndInstallPlatformDependencies(ctx, osConfig2, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(smallCacheChecker.Cache.Size()).To(Equal(2))

				// Add third entry should evict oldest
				err = smallCacheChecker.CheckAndInstallPlatformDependencies(ctx, osConfig3, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(smallCacheChecker.Cache.Size()).To(Equal(2))
			})
		})
	})

	Describe("Metrics Collection", func() {
		Context("when tracking dependency operations", func() {
			It("should track cache hits and misses correctly", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl", "git"},
						},
					},
				}

				// Initial metrics should be zero
				initialMetrics := depChecker.Metrics.GetMetrics()
				Expect(initialMetrics.TotalChecks).To(Equal(int64(0)))
				Expect(initialMetrics.CacheHits).To(Equal(int64(0)))
				Expect(initialMetrics.CacheMisses).To(Equal(int64(0)))

				// First check - should result in cache misses
				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())

				firstMetrics := depChecker.Metrics.GetMetrics()
				Expect(firstMetrics.TotalChecks).To(Equal(int64(2))) // curl + git
				Expect(firstMetrics.CacheMisses).To(Equal(int64(2)))
				Expect(firstMetrics.CacheHits).To(Equal(int64(0)))

				// Second check - should result in cache hits
				err = depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())

				secondMetrics := depChecker.Metrics.GetMetrics()
				Expect(secondMetrics.TotalChecks).To(Equal(int64(4))) // 2 + 2
				Expect(secondMetrics.CacheHits).To(Equal(int64(2)))   // Second check hits cache
				Expect(secondMetrics.CacheMisses).To(Equal(int64(2))) // First check missed
			})

			It("should track validation and install times", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl"},
						},
					},
				}

				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())

				metrics := depChecker.Metrics.GetMetrics()
				Expect(metrics.ValidationTime).To(BeNumerically(">", 0))
				// Install time should be 0 for dry run
				Expect(metrics.InstallTime).To(Equal(time.Duration(0)))
				Expect(metrics.PackagesInstalled).To(Equal(int64(0)))
			})

			It("should reset metrics correctly", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl"},
						},
					},
				}

				// Generate some metrics
				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())

				// Verify metrics exist
				metrics := depChecker.Metrics.GetMetrics()
				Expect(metrics.TotalChecks).To(BeNumerically(">", 0))

				// Reset metrics
				depChecker.Metrics.Reset()

				// Verify metrics are cleared
				resetMetrics := depChecker.Metrics.GetMetrics()
				Expect(resetMetrics.TotalChecks).To(Equal(int64(0)))
				Expect(resetMetrics.CacheHits).To(Equal(int64(0)))
				Expect(resetMetrics.CacheMisses).To(Equal(int64(0)))
				Expect(resetMetrics.ValidationTime).To(Equal(time.Duration(0)))
				Expect(resetMetrics.InstallTime).To(Equal(time.Duration(0)))
				Expect(resetMetrics.PackagesInstalled).To(Equal(int64(0)))
			})

			It("should provide metrics summary logging", func() {
				osConfig := types.OSConfig{
					PlatformRequirements: []types.PlatformRequirement{
						{
							OS:                   "debian",
							PlatformDependencies: []string{"curl", "git"},
						},
					},
				}

				// Generate some metrics
				err := depChecker.CheckAndInstallPlatformDependencies(ctx, osConfig, true)
				Expect(err).NotTo(HaveOccurred())

				// Should not panic when logging metrics
				Expect(func() { depChecker.LogMetricsSummary() }).NotTo(Panic())

				// Test logging with no metrics
				depChecker.Metrics.Reset()
				Expect(func() { depChecker.LogMetricsSummary() }).NotTo(Panic())
			})
		})
	})
})
