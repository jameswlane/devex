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
		defer func(name string) {
			err := os.Remove(name)
			if err != nil {
				panic(err)
			}
		}(filePath)

		loadedThemes, err := themes.LoadThemes(filePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(loadedThemes).To(HaveLen(2))
		Expect(loadedThemes[0].Name).To(Equal("Solarized Dark"))
		Expect(loadedThemes[1].Name).To(Equal("Gruvbox"))
	})

	It("returns an error for an invalid YAML file", func() {
		filePath := "/tmp/invalid-themes.yaml"
		err := os.WriteFile(filePath, []byte("invalid yaml content"), 0o644)
		Expect(err).ToNot(HaveOccurred())
		defer func(name string) {
			err := os.Remove(name)
			if err != nil {
				panic(err)
			}
		}(filePath)

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
		defer func(name string) {
			err := os.Remove(name)
			if err != nil {
				panic(err)
			}
		}(filePath)

		loadedThemes, err := themes.LoadThemes(filePath)
		Expect(err).ToNot(HaveOccurred())
		Expect(loadedThemes).To(HaveLen(0))
	})
})

var _ = Describe("GetAvailableThemes", func() {
	It("extracts themes from apps with theme support", func() {
		apps := []interface{}{
			map[string]interface{}{
				"name": "neovim",
				"themes": []interface{}{
					map[string]interface{}{
						"name":             "Tokyo Night",
						"theme_color":      "#1A1B26",
						"theme_background": "dark",
					},
					map[string]interface{}{
						"name":             "Kanagawa",
						"theme_color":      "#16161D",
						"theme_background": "dark",
					},
				},
			},
			map[string]interface{}{
				"name": "typora",
				"themes": []interface{}{
					map[string]interface{}{
						"name":             "Standard Theme",
						"theme_color":      "",
						"theme_background": "light",
					},
				},
			},
		}

		themesList := themes.GetAvailableThemes(apps)

		Expect(themesList).To(HaveLen(3))

		themeNames := make([]string, len(themesList))
		for i, theme := range themesList {
			themeNames[i] = theme.Name
		}

		Expect(themeNames).To(ContainElement("Tokyo Night"))
		Expect(themeNames).To(ContainElement("Kanagawa"))
		Expect(themeNames).To(ContainElement("Standard Theme"))
	})

	It("handles apps without themes", func() {
		apps := []interface{}{
			map[string]interface{}{
				"name": "git",
				// No themes field
			},
			map[string]interface{}{
				"name":   "docker",
				"themes": []interface{}{}, // Empty themes
			},
		}

		themesList := themes.GetAvailableThemes(apps)
		Expect(themesList).To(HaveLen(0))
	})

	It("handles empty apps list", func() {
		apps := []interface{}{}
		themesList := themes.GetAvailableThemes(apps)
		Expect(themesList).To(HaveLen(0))
	})

	It("deduplicates themes with same name", func() {
		apps := []interface{}{
			map[string]interface{}{
				"name": "app1",
				"themes": []interface{}{
					map[string]interface{}{
						"name":             "Dark Theme",
						"theme_color":      "#000000",
						"theme_background": "dark",
					},
				},
			},
			map[string]interface{}{
				"name": "app2",
				"themes": []interface{}{
					map[string]interface{}{
						"name":             "Dark Theme", // Duplicate name
						"theme_color":      "#111111",    // Different color
						"theme_background": "dark",
					},
				},
			},
		}

		themesList := themes.GetAvailableThemes(apps)
		Expect(themesList).To(HaveLen(1))
		Expect(themesList[0].Name).To(Equal("Dark Theme"))
	})

	It("handles malformed theme data gracefully", func() {
		apps := []interface{}{
			map[string]interface{}{
				"name": "malformed-app",
				"themes": []interface{}{
					map[string]interface{}{
						// Missing name field
						"theme_color":      "#000000",
						"theme_background": "dark",
					},
					map[string]interface{}{
						"name":             "Valid Theme",
						"theme_color":      "#FFFFFF",
						"theme_background": "light",
					},
					"invalid-theme-data", // Not a map
				},
			},
		}

		themesList := themes.GetAvailableThemes(apps)
		Expect(themesList).To(HaveLen(1))
		Expect(themesList[0].Name).To(Equal("Valid Theme"))
	})

	It("handles mixed valid and invalid apps", func() {
		apps := []interface{}{
			"invalid-app-data", // Not a map
			map[string]interface{}{
				"name": "valid-app",
				"themes": []interface{}{
					map[string]interface{}{
						"name":             "Valid Theme",
						"theme_color":      "#FFFFFF",
						"theme_background": "light",
					},
				},
			},
			nil, // Nil app
		}

		themesList := themes.GetAvailableThemes(apps)
		Expect(themesList).To(HaveLen(1))
		Expect(themesList[0].Name).To(Equal("Valid Theme"))
	})

	It("handles themes with missing optional fields", func() {
		apps := []interface{}{
			map[string]interface{}{
				"name": "minimal-app",
				"themes": []interface{}{
					map[string]interface{}{
						"name": "Minimal Theme",
						// Missing theme_color and theme_background
					},
				},
			},
		}

		themesList := themes.GetAvailableThemes(apps)
		Expect(themesList).To(HaveLen(1))
		Expect(themesList[0].Name).To(Equal("Minimal Theme"))
		Expect(themesList[0].ThemeColor).To(Equal(""))
		Expect(themesList[0].ThemeBackground).To(Equal(""))
	})

	It("handles nil themes array", func() {
		apps := []interface{}{
			map[string]interface{}{
				"name":   "app-with-nil-themes",
				"themes": nil,
			},
		}

		themesList := themes.GetAvailableThemes(apps)
		Expect(themesList).To(HaveLen(0))
	})
})
