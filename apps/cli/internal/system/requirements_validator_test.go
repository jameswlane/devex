package system_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/system"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("RequirementsValidator", func() {
	var validator *system.RequirementsValidator

	BeforeEach(func() {
		validator = system.NewRequirementsValidator()
	})

	Describe("ValidateRequirements", func() {
		Context("when validating comprehensive system requirements", func() {
			It("should validate memory requirements", func() {
				requirements := types.SystemRequirements{
					MinMemoryMB: 2048,
				}

				results, err := validator.ValidateRequirements("TestApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).ToNot(BeEmpty())

				// Find memory requirement result
				var memoryResult *system.ValidationResult
				for _, result := range results {
					if result.Requirement == "Memory >= 2048 MB" {
						memoryResult = &result
						break
					}
				}
				Expect(memoryResult).ToNot(BeNil())
				Expect(memoryResult.Status).To(BeElementOf([]system.ValidationStatus{
					system.ValidationPassed,
					system.ValidationFailed,
					system.ValidationWarning,
				}))
			})

			It("should validate disk space requirements", func() {
				requirements := types.SystemRequirements{
					MinDiskSpaceMB: 1024,
				}

				results, err := validator.ValidateRequirements("TestApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).ToNot(BeEmpty())

				// Find disk space requirement result
				var diskResult *system.ValidationResult
				for _, result := range results {
					if result.Requirement == "Disk space >= 1024 MB" {
						diskResult = &result
						break
					}
				}
				Expect(diskResult).ToNot(BeNil())
				Expect(diskResult.Status).To(BeElementOf([]system.ValidationStatus{
					system.ValidationPassed,
					system.ValidationFailed,
					system.ValidationWarning,
				}))
			})

			It("should validate version requirements", func() {
				requirements := types.SystemRequirements{
					GitVersion:    "2.0+",
					DockerVersion: "20.10+",
				}

				results, err := validator.ValidateRequirements("TestApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).ToNot(BeEmpty())

				// Check that version requirements are included
				gitFound := false
				dockerFound := false
				for _, result := range results {
					if result.Requirement == "Git 2.0+" {
						gitFound = true
					}
					if result.Requirement == "Docker 20.10+" {
						dockerFound = true
					}
				}
				Expect(gitFound).To(BeTrue())
				Expect(dockerFound).To(BeTrue())
			})

			It("should validate required commands", func() {
				requirements := types.SystemRequirements{
					RequiredCommands: []string{"curl", "wget", "unzip"},
				}

				results, err := validator.ValidateRequirements("TestApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(3))

				for _, result := range results {
					Expect(result.Requirement).To(MatchRegexp("Command '.+' available"))
					Expect(result.Status).To(BeElementOf([]system.ValidationStatus{
						system.ValidationPassed,
						system.ValidationFailed,
					}))
				}
			})

			It("should validate required services", func() {
				requirements := types.SystemRequirements{
					RequiredServices: []string{"docker", "ssh"},
				}

				results, err := validator.ValidateRequirements("TestApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(2))

				for _, result := range results {
					Expect(result.Requirement).To(MatchRegexp("Service '.+' running"))
					Expect(result.Status).To(BeElementOf([]system.ValidationStatus{
						system.ValidationPassed,
						system.ValidationFailed,
					}))
				}
			})

			It("should validate required ports", func() {
				requirements := types.SystemRequirements{
					RequiredPorts: []int{8080, 9000, 3000},
				}

				results, err := validator.ValidateRequirements("TestApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(3))

				for _, result := range results {
					Expect(result.Requirement).To(MatchRegexp("Port \\d+ available"))
					Expect(result.Status).To(BeElementOf([]system.ValidationStatus{
						system.ValidationPassed,
						system.ValidationFailed,
					}))
				}
			})

			It("should validate required environment variables", func() {
				requirements := types.SystemRequirements{
					RequiredEnvVars: []string{"HOME", "USER", "CUSTOM_VAR"},
				}

				results, err := validator.ValidateRequirements("TestApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(3))

				for _, result := range results {
					Expect(result.Requirement).To(MatchRegexp("Environment variable '.+' set"))
					Expect(result.Status).To(BeElementOf([]system.ValidationStatus{
						system.ValidationPassed,
						system.ValidationFailed,
					}))
				}
			})

			It("should handle complex requirements with multiple validations", func() {
				requirements := types.SystemRequirements{
					MinMemoryMB:      2048,
					MinDiskSpaceMB:   1024,
					DockerVersion:    "20.10+",
					GitVersion:       "2.0+",
					GoVersion:        "1.19+",
					RequiredCommands: []string{"curl", "git"},
					RequiredServices: []string{"docker"},
					RequiredPorts:    []int{8080},
					RequiredEnvVars:  []string{"HOME"},
				}

				results, err := validator.ValidateRequirements("ComplexApp", requirements)
				Expect(err).ToNot(HaveOccurred())

				// Should have at least 9 validation results
				Expect(len(results)).To(BeNumerically(">=", 9))

				// Verify no unknown statuses
				for _, result := range results {
					Expect(result.Status).To(BeElementOf([]system.ValidationStatus{
						system.ValidationPassed,
						system.ValidationFailed,
						system.ValidationWarning,
						system.ValidationSkipped,
					}))
				}
			})

			It("should return empty results when no requirements are specified", func() {
				requirements := types.SystemRequirements{}

				results, err := validator.ValidateRequirements("EmptyApp", requirements)
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(BeEmpty())
			})
		})
	})

	Describe("HasFailures", func() {
		It("should correctly identify when there are failures", func() {
			results := []system.ValidationResult{
				{Status: system.ValidationPassed},
				{Status: system.ValidationFailed},
				{Status: system.ValidationWarning},
			}

			hasFailures := validator.HasFailures(results)
			Expect(hasFailures).To(BeTrue())
		})

		It("should correctly identify when there are no failures", func() {
			results := []system.ValidationResult{
				{Status: system.ValidationPassed},
				{Status: system.ValidationWarning},
				{Status: system.ValidationSkipped},
			}

			hasFailures := validator.HasFailures(results)
			Expect(hasFailures).To(BeFalse())
		})

		It("should handle empty results", func() {
			results := []system.ValidationResult{}

			hasFailures := validator.HasFailures(results)
			Expect(hasFailures).To(BeFalse())
		})
	})

	Describe("GetFailures", func() {
		It("should return only failed validation results", func() {
			results := []system.ValidationResult{
				{Status: system.ValidationPassed, Requirement: "Test1"},
				{Status: system.ValidationFailed, Requirement: "Test2"},
				{Status: system.ValidationWarning, Requirement: "Test3"},
				{Status: system.ValidationFailed, Requirement: "Test4"},
			}

			failures := validator.GetFailures(results)
			Expect(failures).To(HaveLen(2))
			Expect(failures[0].Requirement).To(Equal("Test2"))
			Expect(failures[1].Requirement).To(Equal("Test4"))
		})

		It("should return empty slice when no failures", func() {
			results := []system.ValidationResult{
				{Status: system.ValidationPassed},
				{Status: system.ValidationWarning},
			}

			failures := validator.GetFailures(results)
			Expect(failures).To(BeEmpty())
		})
	})

	Describe("GetWarnings", func() {
		It("should return only warning validation results", func() {
			results := []system.ValidationResult{
				{Status: system.ValidationPassed, Requirement: "Test1"},
				{Status: system.ValidationFailed, Requirement: "Test2"},
				{Status: system.ValidationWarning, Requirement: "Test3"},
				{Status: system.ValidationWarning, Requirement: "Test4"},
			}

			warnings := validator.GetWarnings(results)
			Expect(warnings).To(HaveLen(2))
			Expect(warnings[0].Requirement).To(Equal("Test3"))
			Expect(warnings[1].Requirement).To(Equal("Test4"))
		})

		It("should return empty slice when no warnings", func() {
			results := []system.ValidationResult{
				{Status: system.ValidationPassed},
				{Status: system.ValidationFailed},
			}

			warnings := validator.GetWarnings(results)
			Expect(warnings).To(BeEmpty())
		})
	})

	Describe("ValidationStatus String method", func() {
		It("should return correct string representations", func() {
			Expect(system.ValidationPassed.String()).To(Equal("PASSED"))
			Expect(system.ValidationFailed.String()).To(Equal("FAILED"))
			Expect(system.ValidationWarning.String()).To(Equal("WARNING"))
			Expect(system.ValidationSkipped.String()).To(Equal("SKIPPED"))
		})

		It("should handle unknown status", func() {
			unknownStatus := system.ValidationStatus(999)
			Expect(unknownStatus.String()).To(Equal("UNKNOWN"))
		})
	})
})
