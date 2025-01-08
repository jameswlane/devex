package themes_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/themes"
)

var _ = Describe("LoadThemes", func() {
	It("loads themes from a valid YAML file", func() {
		themeYAML := `
themes:
  - name: Solarized Dark
    theme_color: "#073642"
    theme_background: "#002b36"
    neovim_colorscheme: "solarized-dark"
  - name: Gruvbox
    theme_color: "#282828"
    theme_background: "#1d2021"
    neovim_colorscheme: "gruvbox"
`
		filePath := "/tmp/themes.yaml"
		err := os.WriteFile(filePath, []byte(themeYAML), 0o644)
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(filePath)

		themes, err := themes.LoadThemes(filePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(themes).To(HaveLen(2))
		Expect(themes[0].Name).To(Equal("Solarized Dark"))
		Expect(themes[1].Name).To(Equal("Gruvbox"))
	})

	It("returns an error for an invalid YAML file", func() {
		filePath := "/tmp/invalid-themes.yaml"
		err := os.WriteFile(filePath, []byte("invalid yaml content"), 0o644)
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(filePath)

		_, err = themes.LoadThemes(filePath)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to parse theme YAML"))
	})

	It("returns an error if the file does not exist", func() {
		filePath := "/tmp/nonexistent.yaml"

		_, err := themes.LoadThemes(filePath)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to read theme file"))
	})

	It("returns an empty list for an empty theme file", func() {
		filePath := "/tmp/empty-themes.yaml"
		err := os.WriteFile(filePath, []byte("themes: []"), 0o644)
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(filePath)

		themes, err := themes.LoadThemes(filePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(themes).To(HaveLen(0))
	})
})
