package config_test

import (
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Config", func() {
	BeforeEach(func() {
		// Initialize the logger to discard output during tests
		log.InitDefaultLogger(io.Discard)
	})

	Context("LoadConfigs", func() {
		It("loads configurations without error", func() {
			homeDir := "testdata"
			files := []string{"config.yaml"}

			v, err := config.LoadConfigs(homeDir, files) // Handle both return values
			Expect(err).ToNot(HaveOccurred())
			Expect(v).ToNot(BeNil()) // Ensure the returned viper instance is not nil
		})
	})

	Context("ValidateApp", func() {
		It("validates a valid app configuration", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "TestApp",
				},
				InstallMethod: "apt",
			}
			err := config.ValidateApp(app)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error for invalid app configuration", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "",
				},
				InstallMethod: "",
			}
			err := config.ValidateApp(app)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("LoadSettings", func() {
		It("loads settings successfully", func() {
			settings, err := config.LoadSettings("testdata/settings.yaml")
			Expect(err).ToNot(HaveOccurred())
			Expect(settings).ToNot(BeNil())
		})
	})

	Context("Configuration Validation", func() {
		It("validates applications config with proper structure", func() {
			// Create a valid applications config map
			configMap := map[string]interface{}{
				"applications": map[interface{}]interface{}{
					"development":  []interface{}{},
					"databases":    []interface{}{},
					"system_tools": []interface{}{},
					"optional":     []interface{}{},
				},
			}

			err := config.ValidateApplicationsConfig(configMap)
			Expect(err).ToNot(HaveOccurred())
		})

		It("fails validation when applications section is missing", func() {
			configMap := map[string]interface{}{
				"other": map[interface{}]interface{}{},
			}

			err := config.ValidateApplicationsConfig(configMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing required section: applications"))
		})

		It("fails validation when required subsection is missing", func() {
			configMap := map[string]interface{}{
				"applications": map[interface{}]interface{}{
					"development": []interface{}{},
					// Missing databases, system_tools, optional
				},
			}

			err := config.ValidateApplicationsConfig(configMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("missing required section: applications.databases"))
		})

		It("fails validation when applications is not a map", func() {
			configMap := map[string]interface{}{
				"applications": "invalid_structure",
			}

			err := config.ValidateApplicationsConfig(configMap)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("applications section must be a map"))
		})
	})

	Context("Utility Functions", func() {
		Context("ToStringSlice", func() {
			It("converts an array of interfaces to a string slice", func() {
				input := []any{"a", "b", "c"}
				result := config.ToStringSlice(input)
				Expect(result).To(Equal([]string{"a", "b", "c"}))
			})

			It("returns nil for nil input", func() {
				result := config.ToStringSlice(nil)
				Expect(result).To(BeNil())
			})
		})
	})

	Context("Platform Dependencies Resolution", func() {
		Context("ResolvePlatformDependencies", func() {
			It("returns legacy dependencies when present", func() {
				candidate := map[string]any{
					"dependencies": []any{"legacy-dep1", "legacy-dep2"},
				}

				result := config.ResolvePlatformDependencies(candidate)
				Expect(result).To(Equal([]string{"legacy-dep1", "legacy-dep2"}))
			})

			It("returns platform-specific dependencies for matching OS", func() {
				candidate := map[string]any{
					"platform_requirements": []any{
						map[string]any{
							"os":           "linux",
							"dependencies": []any{"linux-dep1", "linux-dep2"},
						},
						map[string]any{
							"os":           "darwin",
							"dependencies": []any{"macos-dep1", "macos-dep2"},
						},
					},
				}

				result := config.ResolvePlatformDependencies(candidate)
				Expect(result).To(BeElementOf([][]string{
					{"linux-dep1", "linux-dep2"},
					{"macos-dep1", "macos-dep2"},
					nil, // Windows or other OS without requirements
				}))
			})

			It("returns platform-specific dependencies for matching distribution", func() {
				candidate := map[string]any{
					"platform_requirements": []any{
						map[string]any{
							"os":           "ubuntu", // Distribution-specific requirement
							"dependencies": []any{"ubuntu-dep1", "ubuntu-dep2"},
						},
					},
				}

				result := config.ResolvePlatformDependencies(candidate)
				// Result depends on the actual platform, but should be valid
				if result != nil {
					Expect(result).To(Equal([]string{"ubuntu-dep1", "ubuntu-dep2"}))
				}
			})

			It("returns nil when no platform requirements exist", func() {
				candidate := map[string]any{
					"name": "test-app",
				}

				result := config.ResolvePlatformDependencies(candidate)
				Expect(result).To(BeNil())
			})

			It("returns nil when platform requirements is not a slice", func() {
				candidate := map[string]any{
					"platform_requirements": "invalid",
				}

				result := config.ResolvePlatformDependencies(candidate)
				Expect(result).To(BeNil())
			})

			It("skips invalid platform requirement entries", func() {
				candidate := map[string]any{
					"platform_requirements": []any{
						"invalid-entry",
						map[string]any{
							"os":           "linux",
							"dependencies": []any{"valid-dep"},
						},
					},
				}

				result := config.ResolvePlatformDependencies(candidate)
				// Should either be nil or contain valid-dep depending on platform
				if result != nil {
					Expect(result).To(Equal([]string{"valid-dep"}))
				}
			})

			It("handles missing OS field in platform requirements", func() {
				candidate := map[string]any{
					"platform_requirements": []any{
						map[string]any{
							"dependencies": []any{"no-os-dep"},
						},
					},
				}

				result := config.ResolvePlatformDependencies(candidate)
				Expect(result).To(BeNil())
			})

			It("handles non-string OS field in platform requirements", func() {
				candidate := map[string]any{
					"platform_requirements": []any{
						map[string]any{
							"os":           123, // Invalid type
							"dependencies": []any{"invalid-os-dep"},
						},
					},
				}

				result := config.ResolvePlatformDependencies(candidate)
				Expect(result).To(BeNil())
			})

			It("prioritizes legacy dependencies over platform requirements", func() {
				candidate := map[string]any{
					"dependencies": []any{"legacy-dep"},
					"platform_requirements": []any{
						map[string]any{
							"os":           "linux",
							"dependencies": []any{"platform-dep"},
						},
					},
				}

				result := config.ResolvePlatformDependencies(candidate)
				Expect(result).To(Equal([]string{"legacy-dep"}))
			})
		})

		Context("MatchesPlatform", func() {
			It("returns false when platform is nil", func() {
				result := config.MatchesPlatform("linux", nil)
				Expect(result).To(BeFalse())
			})

			It("matches direct OS match", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "linux",
					Distribution: "ubuntu",
				}

				result := config.MatchesPlatform("linux", testPlatform)
				Expect(result).To(BeTrue())
			})

			It("matches distribution for Linux platform", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "linux",
					Distribution: "ubuntu",
				}

				result := config.MatchesPlatform("ubuntu", testPlatform)
				Expect(result).To(BeTrue())
			})

			It("does not match distribution for non-Linux platform", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "darwin",
					Distribution: "macos",
				}

				result := config.MatchesPlatform("ubuntu", testPlatform)
				Expect(result).To(BeFalse())
			})

			It("returns false for non-matching OS", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "darwin",
					Distribution: "macos",
				}

				result := config.MatchesPlatform("windows", testPlatform)
				Expect(result).To(BeFalse())
			})

			It("returns false for non-matching distribution", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "linux",
					Distribution: "fedora",
				}

				result := config.MatchesPlatform("ubuntu", testPlatform)
				Expect(result).To(BeFalse())
			})

			It("handles empty OS requirement", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "linux",
					Distribution: "ubuntu",
				}

				result := config.MatchesPlatform("", testPlatform)
				Expect(result).To(BeFalse())
			})

			It("handles empty platform OS", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "",
					Distribution: "ubuntu",
				}

				result := config.MatchesPlatform("linux", testPlatform)
				Expect(result).To(BeFalse())
			})

			It("handles case sensitivity correctly", func() {
				testPlatform := &platform.DetectionResult{
					OS:           "Linux",  // Capital L
					Distribution: "Ubuntu", // Capital U
				}

				// Should not match due to case sensitivity
				Expect(config.MatchesPlatform("linux", testPlatform)).To(BeFalse())
				Expect(config.MatchesPlatform("ubuntu", testPlatform)).To(BeFalse())

				// Should match exact case
				Expect(config.MatchesPlatform("Linux", testPlatform)).To(BeTrue())
			})
		})
	})

	Context("Integration Tests", func() {
		Context("Platform-specific dependency resolution workflow", func() {
			It("resolves complete dependency chains with platform awareness", func() {
				// Create a complex scenario with multiple platform requirements
				candidate := map[string]any{
					"name": "multi-platform-app",
					"platform_requirements": []any{
						map[string]any{
							"os":           "linux",
							"dependencies": []any{"build-essential", "cmake"},
						},
						map[string]any{
							"os":           "darwin",
							"dependencies": []any{"xcode-command-line-tools"},
						},
						map[string]any{
							"os":           "ubuntu",
							"dependencies": []any{"ubuntu-specific-dep", "gnupg"},
						},
					},
				}

				// Verify that the resolution works consistently
				result1 := config.ResolvePlatformDependencies(candidate)
				result2 := config.ResolvePlatformDependencies(candidate)

				// Results should be consistent (testing caching)
				Expect(result1).To(Equal(result2))

				// Result should be one of the expected platform-specific dependency sets
				if result1 != nil {
					Expect(result1).To(BeElementOf([][]string{
						{"build-essential", "cmake"},     // Linux
						{"xcode-command-line-tools"},     // macOS
						{"ubuntu-specific-dep", "gnupg"}, // Ubuntu
					}))
				}
			})

			It("handles mixed legacy and platform-specific configurations", func() {
				// Test the prioritization correctly
				candidates := []map[string]any{
					{
						"name":         "legacy-only",
						"dependencies": []any{"legacy-dep1", "legacy-dep2"},
					},
					{
						"name": "platform-only",
						"platform_requirements": []any{
							map[string]any{
								"os":           "linux",
								"dependencies": []any{"platform-dep1"},
							},
						},
					},
					{
						"name":         "mixed-config",
						"dependencies": []any{"legacy-wins"},
						"platform_requirements": []any{
							map[string]any{
								"os":           "linux",
								"dependencies": []any{"should-not-use"},
							},
						},
					},
				}

				for _, candidate := range candidates {
					result := config.ResolvePlatformDependencies(candidate)

					switch candidate["name"] {
					case "legacy-only":
						Expect(result).To(Equal([]string{"legacy-dep1", "legacy-dep2"}))
					case "platform-only":
						// Result depends on actual platform
						if result != nil {
							Expect(result).To(Equal([]string{"platform-dep1"}))
						}
					case "mixed-config":
						// Legacy should always win
						Expect(result).To(Equal([]string{"legacy-wins"}))
					}
				}
			})

			It("validates platform matching logic across different scenarios", func() {
				testCases := []struct {
					name        string
					reqOS       string
					platform    platform.DetectionResult
					shouldMatch bool
				}{
					{
						name:        "exact OS match",
						reqOS:       "linux",
						platform:    platform.DetectionResult{OS: "linux", Distribution: "ubuntu"},
						shouldMatch: true,
					},
					{
						name:        "distribution match on Linux",
						reqOS:       "ubuntu",
						platform:    platform.DetectionResult{OS: "linux", Distribution: "ubuntu"},
						shouldMatch: true,
					},
					{
						name:        "distribution no match on macOS",
						reqOS:       "ubuntu",
						platform:    platform.DetectionResult{OS: "darwin", Distribution: "macos"},
						shouldMatch: false,
					},
					{
						name:        "case sensitive OS",
						reqOS:       "Linux",
						platform:    platform.DetectionResult{OS: "linux", Distribution: "ubuntu"},
						shouldMatch: false,
					},
					{
						name:        "Windows direct match",
						reqOS:       "windows",
						platform:    platform.DetectionResult{OS: "windows", Distribution: ""},
						shouldMatch: true,
					},
				}

				for _, tc := range testCases {
					By(tc.name)
					result := config.MatchesPlatform(tc.reqOS, &tc.platform)
					if tc.shouldMatch {
						Expect(result).To(BeTrue(), "Expected %s to match platform %+v", tc.reqOS, tc.platform)
					} else {
						Expect(result).To(BeFalse(), "Expected %s to NOT match platform %+v", tc.reqOS, tc.platform)
					}
				}
			})

			It("ensures thread safety of platform caching", func() {
				// Test concurrent access to platform caching
				results := make(chan []string, 10)
				candidate := map[string]any{
					"platform_requirements": []any{
						map[string]any{
							"os":           "linux",
							"dependencies": []any{"concurrent-dep"},
						},
					},
				}

				// Run multiple goroutines concurrently
				for i := 0; i < 10; i++ {
					go func() {
						result := config.ResolvePlatformDependencies(candidate)
						results <- result
					}()
				}

				// Collect results
				var allResults [][]string
				for i := 0; i < 10; i++ {
					allResults = append(allResults, <-results)
				}

				// All results should be consistent
				firstResult := allResults[0]
				for i, result := range allResults {
					Expect(result).To(Equal(firstResult), "Result %d should match first result", i)
				}
			})
		})
	})
})
