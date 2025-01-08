package fonts_test

import (
	"archive/zip"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/jameswlane/devex/pkg/fonts"
	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/mocks"
)

func mockDownloadFile(url string) (string, error) {
	zipFile := "/mock/test-font.zip"

	file, err := fs.AppFs.Create(zipFile)
	if err != nil {
		return "", fmt.Errorf("failed to create mock zip file: %w", err)
	}

	writer := zip.NewWriter(file)

	// Add a test font file to the zip
	w, err := writer.Create("test-font.ttf")
	if err != nil {
		return "", fmt.Errorf("failed to add file to mock zip: %w", err)
	}
	_, err = w.Write([]byte("mock font data"))
	if err != nil {
		return "", fmt.Errorf("failed to write mock font data: %w", err)
	}

	// Close the writer to finalize the zip file
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize mock zip: %w", err)
	}

	// Close the file to flush its content to the in-memory filesystem
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("failed to close mock zip file: %w", err)
	}

	return zipFile, nil
}

var _ = Describe("Fonts", func() {
	BeforeEach(func() {
		fs.SetFs(afero.NewMemMapFs()) // Reset to in-memory filesystem
	})

	It("loads fonts from a valid YAML file", func() {
		fontsYAML := `
- name: Test Font
  method: url
  url: https://example.com/test-font.zip
  destination: /mock/fonts
`
		filename := "/mock/test-fonts.yaml"
		err := fs.WriteFile(filename, []byte(fontsYAML), 0o644)
		Expect(err).ToNot(HaveOccurred())

		fontsList, err := fonts.LoadFonts(filename)
		Expect(err).ToNot(HaveOccurred())
		Expect(fontsList).To(HaveLen(1))
		Expect(fontsList[0].Name).To(Equal("Test Font"))
	})

	It("handles an invalid YAML file gracefully", func() {
		filename := "/mock/invalid-fonts.yaml"
		err := fs.WriteFile(filename, []byte("{invalid_yaml}"), 0o644)
		Expect(err).ToNot(HaveOccurred())

		_, err = fonts.LoadFonts(filename)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to unmarshal fonts YAML"))
	})

	It("installs a font from a URL using mock utils", func() {
		mockUtils := mocks.NewMockUtils()
		fonts.SetUtils(mockUtils)
		fonts.SetDownloadFile(mockDownloadFile)

		font := fonts.Font{
			Name:        "Mock Font",
			Method:      "url",
			URL:         "https://mockurl.com/font.zip",
			Destination: "/mock/fonts",
		}

		err := fonts.InstallFont(font)
		Expect(err).ToNot(HaveOccurred())

		exists, err := fs.Exists("/mock/fonts/test-font.ttf")
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("extracts fonts from a zip archive", func() {
		zipFile := "/mock/test-font.zip"
		destDir := "/mock/fonts"

		// Create a zip file in memory
		file, err := fs.AppFs.Create(zipFile)
		Expect(err).ToNot(HaveOccurred())

		writer := zip.NewWriter(file)

		w, err := writer.Create("test-font.ttf")
		Expect(err).ToNot(HaveOccurred())
		_, err = w.Write([]byte("mock font data"))
		Expect(err).ToNot(HaveOccurred())

		// Close the writer and file
		Expect(writer.Close()).To(Succeed())
		Expect(file.Close()).To(Succeed())

		// Test the UnzipAndMove function
		err = fonts.UnzipAndMove(zipFile, "", destDir)
		Expect(err).ToNot(HaveOccurred())

		exists, err := fs.Exists("/mock/fonts/test-font.ttf")
		Expect(err).ToNot(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("handles installation via unsupported method gracefully", func() {
		font := fonts.Font{
			Name:   "Invalid Font",
			Method: "unsupported",
		}

		err := fonts.InstallFont(font)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported install method"))
	})
})
