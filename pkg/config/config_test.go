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
			app := types.AppConfig{Name: "TestApp", InstallMethod: "apt"} // Correct struct reference
			err := config.ValidateApp(app)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns an error for invalid app configuration", func() {
			app := types.AppConfig{Name: "", InstallMethod: ""} // Correct struct reference
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
