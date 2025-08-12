package config_test

import (
	"io"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
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
})
