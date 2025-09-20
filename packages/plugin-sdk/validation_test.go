package sdk_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	
	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("sdk.TimeoutConfig", func() {
	Describe("DefaultTimeouts", func() {
		It("should return sensible default values", func() {
			defaults := sdk.DefaultTimeouts()
			
			Expect(defaults.Default).To(Equal(5 * time.Minute))
			Expect(defaults.Install).To(Equal(10 * time.Minute))
			Expect(defaults.Update).To(Equal(2 * time.Minute))
			Expect(defaults.Upgrade).To(Equal(15 * time.Minute))
			Expect(defaults.Search).To(Equal(30 * time.Second))
			Expect(defaults.Network).To(Equal(1 * time.Minute))
			Expect(defaults.Build).To(Equal(30 * time.Minute))
			Expect(defaults.Shell).To(Equal(5 * time.Minute))
		})
	})

	Describe("GetTimeout", func() {
		var config sdk.TimeoutConfig

		BeforeEach(func() {
			config = sdk.TimeoutConfig{
				Default: 1 * time.Minute,
				Install: 5 * time.Minute,
				Update:  30 * time.Second,
				Upgrade: 10 * time.Minute,
				Search:  10 * time.Second,
				Network: 45 * time.Second,
				Build:   20 * time.Minute,
				Shell:   2 * time.Minute,
			}
		})

		It("should return specific timeout for known operations", func() {
			Expect(config.GetTimeout("install")).To(Equal(5 * time.Minute))
			Expect(config.GetTimeout("update")).To(Equal(30 * time.Second))
			Expect(config.GetTimeout("upgrade")).To(Equal(10 * time.Minute))
			Expect(config.GetTimeout("search")).To(Equal(10 * time.Second))
			Expect(config.GetTimeout("network")).To(Equal(45 * time.Second))
			Expect(config.GetTimeout("build")).To(Equal(20 * time.Minute))
			Expect(config.GetTimeout("shell")).To(Equal(2 * time.Minute))
		})

		It("should be case insensitive", func() {
			Expect(config.GetTimeout("INSTALL")).To(Equal(5 * time.Minute))
			Expect(config.GetTimeout("Install")).To(Equal(5 * time.Minute))
			Expect(config.GetTimeout("iNsTaLl")).To(Equal(5 * time.Minute))
		})

		It("should return default timeout for unknown operations", func() {
			Expect(config.GetTimeout("unknown")).To(Equal(1 * time.Minute))
			Expect(config.GetTimeout("")).To(Equal(1 * time.Minute))
		})

		It("should fallback to system defaults when specific timeout is zero", func() {
			config.Install = 0
			Expect(config.GetTimeout("install")).To(Equal(1 * time.Minute)) // falls back to config default
			
			config.Default = 0
			Expect(config.GetTimeout("install")).To(Equal(sdk.DefaultTimeouts().Default))
		})
	})

	Describe("sdk.ValidateTimeoutConfig", func() {
		It("should accept valid timeout configurations", func() {
			config := sdk.TimeoutConfig{
				Default: 5 * time.Minute,
				Install: 10 * time.Minute,
				Update:  2 * time.Minute,
				Upgrade: 15 * time.Minute,
				Search:  30 * time.Second,
				Network: 1 * time.Minute,
				Build:   30 * time.Minute,
				Shell:   5 * time.Minute,
			}
			
			Expect(sdk.ValidateTimeoutConfig(config)).To(Succeed())
		})

		It("should accept zero timeouts", func() {
			config := sdk.TimeoutConfig{} // All zero values
			Expect(sdk.ValidateTimeoutConfig(config)).To(Succeed())
		})

		It("should reject negative timeouts", func() {
			testCases := []struct {
				name   string
				config sdk.TimeoutConfig
				error  string
			}{
				{
					name:   "negative default",
					config: sdk.TimeoutConfig{Default: -1 * time.Second},
					error:  "default timeout cannot be negative",
				},
				{
					name:   "negative install",
					config: sdk.TimeoutConfig{Install: -1 * time.Second},
					error:  "install timeout cannot be negative",
				},
				{
					name:   "negative update",
					config: sdk.TimeoutConfig{Update: -1 * time.Second},
					error:  "update timeout cannot be negative",
				},
				{
					name:   "negative upgrade",
					config: sdk.TimeoutConfig{Upgrade: -1 * time.Second},
					error:  "upgrade timeout cannot be negative",
				},
				{
					name:   "negative search",
					config: sdk.TimeoutConfig{Search: -1 * time.Second},
					error:  "search timeout cannot be negative",
				},
				{
					name:   "negative network",
					config: sdk.TimeoutConfig{Network: -1 * time.Second},
					error:  "network timeout cannot be negative",
				},
				{
					name:   "negative build",
					config: sdk.TimeoutConfig{Build: -1 * time.Second},
					error:  "build timeout cannot be negative",
				},
				{
					name:   "negative shell",
					config: sdk.TimeoutConfig{Shell: -1 * time.Second},
					error:  "shell timeout cannot be negative",
				},
			}

			for _, tc := range testCases {
				err := sdk.ValidateTimeoutConfig(tc.config)
				Expect(err).To(HaveOccurred(), "test case: %s", tc.name)
				Expect(err.Error()).To(Equal(tc.error), "test case: %s", tc.name)
			}
		})
	})

	Describe("sdk.ValidateTimeout", func() {
		It("should accept positive timeouts", func() {
			Expect(sdk.ValidateTimeout(5*time.Second, "test")).To(Succeed())
			Expect(sdk.ValidateTimeout(1*time.Minute, "test")).To(Succeed())
			Expect(sdk.ValidateTimeout(1*time.Hour, "test")).To(Succeed())
		})

		It("should accept zero timeout", func() {
			Expect(sdk.ValidateTimeout(0, "test")).To(Succeed())
		})

		It("should reject negative timeouts", func() {
			err := sdk.ValidateTimeout(-1*time.Second, "test")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("test timeout cannot be negative"))
		})
	})

	Describe("sdk.IsTimeoutError", func() {
		It("should correctly identify timeout errors", func() {
			timeoutErr := &sdk.TimeoutError{
				Command: "test",
				Args:    []string{"arg1"},
				Timeout: 5 * time.Second,
			}
			
			Expect(sdk.IsTimeoutError(timeoutErr)).To(BeTrue())
		})

		It("should return false for non-timeout errors", func() {
			regularErr := fmt.Errorf("regular error")
			Expect(sdk.IsTimeoutError(regularErr)).To(BeFalse())
		})

		It("should return false for nil error", func() {
			Expect(sdk.IsTimeoutError(nil)).To(BeFalse())
		})
	})

	Describe("sdk.GetTimeoutError", func() {
		It("should extract timeout error", func() {
			original := &sdk.TimeoutError{
				Command: "test",
				Args:    []string{"arg1"},
				Timeout: 5 * time.Second,
			}
			
			extracted := sdk.GetTimeoutError(original)
			Expect(extracted).To(Equal(original))
		})

		It("should return nil for non-timeout errors", func() {
			regularErr := fmt.Errorf("regular error")
			Expect(sdk.GetTimeoutError(regularErr)).To(BeNil())
		})

		It("should return nil for nil error", func() {
			Expect(sdk.GetTimeoutError(nil)).To(BeNil())
		})
	})

	Describe("sdk.NormalizeTimeout", func() {
		It("should use defaults for zero or negative timeouts", func() {
			result := sdk.NormalizeTimeout(0, "install")
			Expect(result).To(Equal(sdk.DefaultTimeouts().Install))

			result = sdk.NormalizeTimeout(-1*time.Second, "install")
			Expect(result).To(Equal(sdk.DefaultTimeouts().Install))
		})

		It("should enforce minimum timeout", func() {
			result := sdk.NormalizeTimeout(1*time.Second, "install")
			Expect(result).To(Equal(5 * time.Second)) // minimum timeout
		})

		It("should enforce maximum timeout", func() {
			result := sdk.NormalizeTimeout(5*time.Hour, "install")
			Expect(result).To(Equal(2 * time.Hour)) // maximum timeout
		})

		It("should pass through valid timeouts unchanged", func() {
			timeout := 30 * time.Second
			result := sdk.NormalizeTimeout(timeout, "install")
			Expect(result).To(Equal(timeout))
		})
	})

	Describe("sdk.TimeoutConfigFromDefaults", func() {
		It("should create config with default values", func() {
			config := sdk.TimeoutConfigFromDefaults(map[string]time.Duration{})
			defaults := sdk.DefaultTimeouts()
			Expect(config).To(Equal(defaults))
		})

		It("should override specific values", func() {
			overrides := map[string]time.Duration{
				"install": 20 * time.Minute,
				"search":  15 * time.Second,
			}
			
			config := sdk.TimeoutConfigFromDefaults(overrides)
			
			Expect(config.Install).To(Equal(20 * time.Minute))
			Expect(config.Search).To(Equal(15 * time.Second))
			// Other values should remain default
			Expect(config.Update).To(Equal(sdk.DefaultTimeouts().Update))
			Expect(config.Upgrade).To(Equal(sdk.DefaultTimeouts().Upgrade))
		})

		It("should handle unknown operation types gracefully", func() {
			overrides := map[string]time.Duration{
				"unknown": 1 * time.Minute,
				"install": 20 * time.Minute,
			}
			
			config := sdk.TimeoutConfigFromDefaults(overrides)
			
			// Known operation should be overridden
			Expect(config.Install).To(Equal(20 * time.Minute))
			// Unknown operation should be ignored, defaults preserved
			Expect(config.Default).To(Equal(sdk.DefaultTimeouts().Default))
		})
	})
})

var _ = Describe("sdk.TimeoutError", func() {
	Describe("Error", func() {
		It("should format error message without operation", func() {
			err := &sdk.TimeoutError{
				Command: "apt",
				Args:    []string{"install", "package"},
				Timeout: 5 * time.Minute,
			}
			
			expected := "command 'apt install package' timed out after 5m0s"
			Expect(err.Error()).To(Equal(expected))
		})

		It("should format error message with operation", func() {
			err := &sdk.TimeoutError{
				Command:   "apt",
				Args:      []string{"install", "package"},
				Timeout:   5 * time.Minute,
				Operation: "install",
			}
			
			expected := "command 'apt install package' timed out after 5m0s during install operation"
			Expect(err.Error()).To(Equal(expected))
		})

		It("should handle empty args", func() {
			err := &sdk.TimeoutError{
				Command: "apt",
				Args:    []string{},
				Timeout: 5 * time.Minute,
			}
			
			expected := "command 'apt ' timed out after 5m0s"
			Expect(err.Error()).To(Equal(expected))
		})
	})
})
