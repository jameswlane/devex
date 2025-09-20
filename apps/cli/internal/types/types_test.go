package types_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("AppConfig", func() {
	Context("Validate", func() {
		It("returns an error if the name is empty", func() {
			app := types.AppConfig{
				InstallMethod: "apt",
			}
			err := app.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("app name is required"))
		})

		It("returns an error if the install method is empty", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "TestApp",
				},
			}
			err := app.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("install method is required"))
		})

		It("returns an error if APT sources are invalid", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "TestApp",
				},
				InstallMethod: "apt",
				AptSources: []types.AptSource{
					{SourceRepo: "", SourceName: ""},
				},
			}
			err := app.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("APT source at index 0 must have repo and list_file defined"))
		})

		It("validates successfully with valid fields", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "TestApp",
				},
				InstallMethod: "curlpipe",
				DownloadURL:   "https://example.com",
			}
			err := app.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("DockerOptions", func() {
	Context("Validate", func() {
		It("returns an error if ContainerName is empty", func() {
			options := types.DockerOptions{}
			err := options.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("container name is required"))
		})

		It("validates successfully with a valid ContainerName", func() {
			options := types.DockerOptions{
				ContainerName: "test-container",
			}
			err := options.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("Font", func() {
	Context("Validate", func() {
		It("returns an error if Name is empty", func() {
			font := types.Font{
				Method: "url",
			}
			err := font.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("font name is required"))
		})

		It("returns an error if Method is empty", func() {
			font := types.Font{
				Name: "TestFont",
			}
			err := font.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("install method is required"))
		})

		It("validates successfully with valid fields", func() {
			font := types.Font{
				Name:   "TestFont",
				Method: "url",
			}
			err := font.Validate()
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

var _ = Describe("AptSource", func() {
	It("allows creation with all fields", func() {
		source := types.AptSource{
			KeySource:      "https://example.com/key",
			KeyName:        "example-key",
			SourceRepo:     "http://example.com/repo",
			SourceName:     "example-repo",
			RequireDearmor: true,
		}

		Expect(source.KeySource).To(Equal("https://example.com/key"))
		Expect(source.KeyName).To(Equal("example-key"))
		Expect(source.SourceRepo).To(Equal("http://example.com/repo"))
		Expect(source.SourceName).To(Equal("example-repo"))
		Expect(source.RequireDearmor).To(BeTrue())
	})
})
