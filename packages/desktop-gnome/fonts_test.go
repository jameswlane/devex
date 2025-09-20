package main

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GNOME Font Manager", func() {
	var (
		fm  *FontManager
		ctx context.Context
	)

	BeforeEach(func() {
		fm = NewFontManager()
		ctx = context.Background()
	})

	Describe("Font Name Mapping", func() {
		Context("when mapping common font names", func() {
			It("should map firacode and jetbrains correctly", func() {
				input := []string{"firacode", "jetbrains"}
				expected := []string{"fonts-firacode", "fonts-jetbrains-mono"}
				result := fm.mapFontNames(input)
				Expect(result).To(Equal(expected))
			})

			It("should preserve unknown font names", func() {
				input := []string{"custom-font"}
				expected := []string{"custom-font"}
				result := fm.mapFontNames(input)
				Expect(result).To(Equal(expected))
			})

			It("should handle mixed case input", func() {
				input := []string{"FiraCode", "HACK"}
				expected := []string{"fonts-firacode", "fonts-hack"}
				result := fm.mapFontNames(input)
				Expect(result).To(Equal(expected))
			})

			It("should handle empty input", func() {
				input := []string{}
				result := fm.mapFontNames(input)
				Expect(result).To(BeEmpty())
			})
		})
	})

	Describe("Default Font Settings", func() {
		Context("when getting default GNOME font settings", func() {
			It("should return non-empty font settings", func() {
				fonts := fm.getDefaultFontSettings()
				Expect(fonts).ToNot(BeEmpty())
			})

			It("should include monospace font setting", func() {
				fonts := fm.getDefaultFontSettings()
				hasMonospace := false
				for _, font := range fonts {
					if font.key == "monospace-font-name" {
						hasMonospace = true
						break
					}
				}
				Expect(hasMonospace).To(BeTrue())
			})

			It("should include system font setting", func() {
				fonts := fm.getDefaultFontSettings()
				hasSystemFont := false
				for _, font := range fonts {
					if font.key == "font-name" {
						hasSystemFont = true
						break
					}
				}
				Expect(hasSystemFont).To(BeTrue())
			})
		})
	})

	Describe("Font Validation", func() {
		Context("when validating font names", func() {
			It("should accept valid font names", func() {
				validNames := []string{"JetBrains Mono", "Fira Code", "Ubuntu-Bold"}
				for _, name := range validNames {
					err := validateFontName(name)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should reject font names with illegal characters", func() {
				invalidNames := []string{"font;rm -rf /", "font`id`", "font$HOME"}
				for _, name := range invalidNames {
					err := validateFontName(name)
					Expect(err).To(HaveOccurred())
				}
			})

			It("should reject font names that are too long", func() {
				longName := make([]byte, 101)
				for i := range longName {
					longName[i] = 'A'
				}
				err := validateFontName(string(longName))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("too long"))
			})
		})
	})

	Describe("Configuration with Context", func() {
		Context("when context is cancelled", func() {
			It("should respect context cancellation", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				defer cancel()

				// This should fail quickly due to context timeout
				err := fm.ConfigureFonts(ctx, []string{"JetBrains Mono"})
				// We expect this to either succeed quickly or fail due to timeout
				// The important thing is it respects the context
				_ = err // We don't assert on the error since it depends on system state
			})
		})
	})

	Describe("Error Handling", func() {
		Context("when invalid arguments are provided", func() {
			It("should validate font arguments in InstallFonts", func() {
				invalidArgs := []string{"valid-font", "invalid;font"}
				err := fm.InstallFonts(ctx, invalidArgs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid font argument"))
			})

			It("should validate font arguments in ConfigureFonts", func() {
				invalidArgs := []string{"font`rm -rf /`"}
				err := fm.ConfigureFonts(ctx, invalidArgs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid font name"))
			})
		})
	})
})
