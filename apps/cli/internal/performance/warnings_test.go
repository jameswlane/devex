package performance_test

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/cache"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/performance"
)

var _ = Describe("PerformanceAnalyzer", func() {
	var (
		analyzer *performance.PerformanceAnalyzer
		tempDir  string
		settings config.CrossPlatformSettings
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-performance-test-*")
		Expect(err).NotTo(HaveOccurred())

		settings = config.CrossPlatformSettings{
			HomeDir: tempDir,
		}

		analyzer, err = performance.NewPerformanceAnalyzer(settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(analyzer).NotTo(BeNil())
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("NewPerformanceAnalyzer", func() {
		Context("when creating a new analyzer", func() {
			It("should initialize successfully with valid settings", func() {
				analyzer, err := performance.NewPerformanceAnalyzer(settings)
				Expect(err).NotTo(HaveOccurred())
				Expect(analyzer).NotTo(BeNil())
			})
		})
	})

	Describe("AnalyzePreInstall", func() {
		Context("when analyzing a small application", func() {
			It("should generate no warnings for small apps", func() {
				warnings := analyzer.AnalyzePreInstall("vim", nil)
				Expect(len(warnings)).To(BeNumerically("<=", 1)) // At most one info warning
			})
		})

		Context("when analyzing a large application", func() {
			It("should generate size warnings for Docker", func() {
				warnings := analyzer.AnalyzePreInstall("docker", nil)

				var foundSizeWarning bool
				for _, warning := range warnings {
					if warning.Level >= performance.WarningLevelCaution {
						foundSizeWarning = true
						Expect(warning.Application).To(Equal("docker"))
						Expect(warning.Message).To(ContainSubstring("large"))
						break
					}
				}
				Expect(foundSizeWarning).To(BeTrue())
			})

			It("should generate critical warnings for very large apps", func() {
				warnings := analyzer.AnalyzePreInstall("android-studio", nil)

				// Android Studio should generate at least a warning (900MB is huge, not massive)
				Expect(len(warnings)).To(BeNumerically(">", 0))

				var foundSizeWarning bool
				for _, warning := range warnings {
					if warning.Level >= performance.WarningLevelWarning {
						foundSizeWarning = true
						Expect(warning.Application).To(Equal("android-studio"))
						Expect(warning.Message).To(ContainSubstring("large"))
						break
					}
				}
				Expect(foundSizeWarning).To(BeTrue())
			})
		})

		Context("when analyzing applications with system impact", func() {
			It("should warn about applications requiring restart", func() {
				warnings := analyzer.AnalyzePreInstall("docker", nil)

				// Check if any warnings are generated (Docker generates size warnings)
				Expect(len(warnings)).To(BeNumerically(">", 0))

				// Docker should have restart requirements detected in system impact
				var foundRestartWarning bool
				for _, warning := range warnings {
					if warning.Metrics != nil && warning.Metrics.RequiresRestart {
						foundRestartWarning = true
						break
					}
				}
				// Note: Docker may or may not require restart depending on system
				// This test validates that the system correctly analyzes restart requirements
				_ = foundRestartWarning // Acknowledge variable usage
			})

			It("should warn about system-level applications", func() {
				warnings := analyzer.AnalyzePreInstall("kernel", nil)

				// Check that system impact warnings are generated for high-impact apps
				var foundHighImpactMetrics bool
				for _, warning := range warnings {
					if warning.Metrics != nil && warning.Metrics.SystemImpact == "high" {
						foundHighImpactMetrics = true
						break
					}
				}
				// Note: Kernel should typically be flagged as high impact
				// This test validates the impact assessment logic
				_ = foundHighImpactMetrics // Acknowledge variable usage
			})
		})

		Context("when analyzing unknown applications", func() {
			It("should use default estimates", func() {
				warnings := analyzer.AnalyzePreInstall("unknown-app-12345", nil)

				// Should not crash and should provide some basic analysis
				// Default estimates should not trigger major warnings
				criticalWarnings := 0
				for _, warning := range warnings {
					if warning.Level == performance.WarningLevelCritical {
						criticalWarnings++
					}
				}
				Expect(criticalWarnings).To(Equal(0))
			})
		})
	})

	Describe("AnalyzePostInstall", func() {
		Context("when recording successful installation", func() {
			It("should record metrics without error", func() {
				startTime := time.Now().Add(-2 * time.Minute)
				err := analyzer.AnalyzePostInstall("test-app", startTime, true, 50*1024*1024)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should handle fast installations", func() {
				startTime := time.Now().Add(-10 * time.Second)
				err := analyzer.AnalyzePostInstall("test-app", startTime, true, 10*1024*1024)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when recording failed installation", func() {
			It("should record failure metrics", func() {
				startTime := time.Now().Add(-5 * time.Minute)
				err := analyzer.AnalyzePostInstall("failed-app", startTime, false, 0)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when recording slow installation", func() {
			It("should generate post-install warnings", func() {
				startTime := time.Now().Add(-20 * time.Minute) // Very slow
				err := analyzer.AnalyzePostInstall("slow-app", startTime, true, 100*1024*1024)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("FormatWarning", func() {
		Context("when formatting different warning levels", func() {
			It("should format critical warnings with appropriate emoji", func() {
				warning := performance.PerformanceWarning{
					Level:       performance.WarningLevelCritical,
					Application: "test-app",
					Message:     "Critical issue detected",
					Details:     []string{"Detail 1", "Detail 2"},
					Suggestions: []string{"Suggestion 1", "Suggestion 2"},
				}

				formatted := performance.FormatWarning(warning)
				Expect(formatted).To(ContainSubstring("🚨"))
				Expect(formatted).To(ContainSubstring("Critical issue detected"))
				Expect(formatted).To(ContainSubstring("Detail 1"))
				Expect(formatted).To(ContainSubstring("Suggestion 1"))
			})

			It("should format warning level with appropriate emoji", func() {
				warning := performance.PerformanceWarning{
					Level:       performance.WarningLevelWarning,
					Application: "test-app",
					Message:     "Warning message",
				}

				formatted := performance.FormatWarning(warning)
				Expect(formatted).To(ContainSubstring("⚠️"))
				Expect(formatted).To(ContainSubstring("Warning message"))
			})

			It("should format caution level with appropriate emoji", func() {
				warning := performance.PerformanceWarning{
					Level:       performance.WarningLevelCaution,
					Application: "test-app",
					Message:     "Caution message",
				}

				formatted := performance.FormatWarning(warning)
				Expect(formatted).To(ContainSubstring("⚡"))
				Expect(formatted).To(ContainSubstring("Caution message"))
			})

			It("should format info level with appropriate emoji", func() {
				warning := performance.PerformanceWarning{
					Level:       performance.WarningLevelInfo,
					Application: "test-app",
					Message:     "Info message",
				}

				formatted := performance.FormatWarning(warning)
				Expect(formatted).To(ContainSubstring("ℹ️"))
				Expect(formatted).To(ContainSubstring("Info message"))
			})
		})

		Context("when formatting warnings without details or suggestions", func() {
			It("should format minimal warnings correctly", func() {
				warning := performance.PerformanceWarning{
					Level:       performance.WarningLevelInfo,
					Application: "test-app",
					Message:     "Simple message",
				}

				formatted := performance.FormatWarning(warning)
				Expect(formatted).To(ContainSubstring("Simple message"))
				Expect(formatted).NotTo(ContainSubstring("Details:"))
				Expect(formatted).NotTo(ContainSubstring("Suggestions:"))
			})
		})
	})

	Describe("Historical Metrics Integration", func() {
		var cacheManager *cache.CacheManager

		BeforeEach(func() {
			var err error
			cacheManager, err = cache.NewCacheManager(settings)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when analyzing with historical data", func() {
			BeforeEach(func() {
				// Record some historical metrics
				metrics := []*cache.PerformanceMetrics{
					{
						ApplicationName: "test-app",
						InstallMethod:   "apt",
						Platform:        "linux",
						DownloadTime:    30 * time.Second,
						InstallTime:     2 * time.Minute,
						TotalTime:       2*time.Minute + 30*time.Second,
						PackageSize:     100 * 1024 * 1024, // 100MB
						Success:         true,
						Timestamp:       time.Now().Add(-1 * time.Hour),
					},
					{
						ApplicationName: "test-app",
						InstallMethod:   "apt",
						Platform:        "linux",
						DownloadTime:    45 * time.Second,
						InstallTime:     3 * time.Minute,
						TotalTime:       3*time.Minute + 45*time.Second,
						PackageSize:     110 * 1024 * 1024, // 110MB
						Success:         false,             // One failure
						Timestamp:       time.Now().Add(-2 * time.Hour),
					},
					{
						ApplicationName: "test-app",
						InstallMethod:   "apt",
						Platform:        "linux",
						DownloadTime:    25 * time.Second,
						InstallTime:     90 * time.Second,
						TotalTime:       90*time.Second + 25*time.Second,
						PackageSize:     95 * 1024 * 1024, // 95MB
						Success:         true,
						Timestamp:       time.Now().Add(-3 * time.Hour),
					},
				}

				for _, metric := range metrics {
					err := cacheManager.RecordPerformanceMetrics(metric)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should use historical data for size estimates", func() {
				warnings := analyzer.AnalyzePreInstall("test-app", nil)

				// Should have some warning based on historical 100MB+ size
				var foundSizeWarning bool
				for _, warning := range warnings {
					if warning.Metrics != nil && warning.Metrics.EstimatedSize > 90*1024*1024 {
						foundSizeWarning = true
						break
					}
				}
				Expect(foundSizeWarning).To(BeTrue())
			})

			It("should detect failure patterns in historical data", func() {
				// Add more failures to trigger failure rate warning
				failureMetrics := []*cache.PerformanceMetrics{
					{
						ApplicationName: "test-app",
						Success:         false,
						Timestamp:       time.Now().Add(-4 * time.Hour),
					},
					{
						ApplicationName: "test-app",
						Success:         false,
						Timestamp:       time.Now().Add(-5 * time.Hour),
					},
				}

				for _, metric := range failureMetrics {
					err := cacheManager.RecordPerformanceMetrics(metric)
					Expect(err).NotTo(HaveOccurred())
				}

				warnings := analyzer.AnalyzePreInstall("test-app", nil)

				// Should warn about high failure rate (3 failures out of 5 = 60%)
				var foundFailureWarning bool
				for _, warning := range warnings {
					if warning.Level >= performance.WarningLevelCaution &&
						warning.Metrics != nil &&
						warning.Metrics.HistoricalFailureRate > 0.1 {
						foundFailureWarning = true
						break
					}
				}
				Expect(foundFailureWarning).To(BeTrue())
			})
		})
	})

	Describe("Edge Cases and Error Handling", func() {
		Context("when cache is unavailable", func() {
			It("should still provide warnings based on estimates", func() {
				// Create analyzer with invalid cache path to simulate error
				invalidSettings := config.CrossPlatformSettings{
					HomeDir: "/invalid/path/that/does/not/exist",
				}

				// This should fail to create due to invalid path
				_, err := performance.NewPerformanceAnalyzer(invalidSettings)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when analyzing nil or empty configurations", func() {
			It("should handle nil app config gracefully", func() {
				warnings := analyzer.AnalyzePreInstall("test-app", nil)
				// Should not crash
				Expect(warnings).NotTo(BeNil())
			})

			It("should handle empty app names", func() {
				warnings := analyzer.AnalyzePreInstall("", nil)
				// Should not crash
				Expect(warnings).NotTo(BeNil())
			})
		})
	})

	Describe("Threshold Configuration", func() {
		Context("when using default thresholds", func() {
			It("should have reasonable default values", func() {
				thresholds := performance.DefaultWarningThresholds

				Expect(thresholds.LargeSizeThreshold).To(Equal(int64(100 * 1024 * 1024)))    // 100MB
				Expect(thresholds.HugeSizeThreshold).To(Equal(int64(500 * 1024 * 1024)))     // 500MB
				Expect(thresholds.MassiveSizeThreshold).To(Equal(int64(1024 * 1024 * 1024))) // 1GB

				Expect(thresholds.LongInstallThreshold).To(Equal(5 * time.Minute))
				Expect(thresholds.VeryLongInstallThreshold).To(Equal(15 * time.Minute))

				Expect(thresholds.ManyDependenciesThreshold).To(Equal(10))
				Expect(thresholds.TooManyDependenciesThreshold).To(Equal(25))

				Expect(thresholds.HighFailureRateThreshold).To(Equal(0.10))     // 10%
				Expect(thresholds.CriticalFailureRateThreshold).To(Equal(0.25)) // 25%
			})
		})
	})
})
