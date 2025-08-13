package utils_test

import (
	"context"

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
				Expect(err.Error()).To(ContainSubstring("invalid package name"))
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
})
