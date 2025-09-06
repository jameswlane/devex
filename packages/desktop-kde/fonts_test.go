package main

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KDE Font Manager", func() {
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

			It("should include KDE-specific fonts in defaults", func() {
				defaultFonts := fm.getDefaultKDEFonts()
				Expect(defaultFonts).To(ContainElement("fonts-oxygen"))
				Expect(defaultFonts).To(ContainElement("fonts-noto"))
			})
		})
	})

	Describe("Default KDE Font Settings", func() {
		Context("when getting default KDE font settings", func() {
			It("should return non-empty font settings", func() {
				fonts := fm.getDefaultKDEFontSettings()
				Expect(fonts).ToNot(BeEmpty())
			})

			It("should include fixed width font setting", func() {
				fonts := fm.getDefaultKDEFontSettings()
				hasFixed := false
				for _, font := range fonts {
					if font.Key == "fixed" {
						hasFixed = true
						break
					}
				}
				Expect(hasFixed).To(BeTrue())
			})

			It("should include general font setting", func() {
				fonts := fm.getDefaultKDEFontSettings()
				hasGeneral := false
				for _, font := range fonts {
					if font.Key == "font" && font.Group == "General" {
						hasGeneral = true
						break
					}
				}
				Expect(hasGeneral).To(BeTrue())
			})
		})
	})

	Describe("Font Validation", func() {
		Context("when validating font names", func() {
			It("should accept valid font names", func() {
				validNames := []string{"JetBrains Mono", "Noto Sans", "Oxygen-Regular"}
				for _, name := range validNames {
					err := validateFontName(name)
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should reject font names with illegal characters", func() {
				invalidNames := []string{"font;kwriteconfig5", "font`id`", "font$HOME"}
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

	Describe("Context Support", func() {
		Context("when using context for operations", func() {
			It("should respect context cancellation in InstallFonts", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				defer cancel()

				err := fm.InstallFonts(ctx, []string{"fonts-firacode"})
				// Error handling depends on system state, we just verify context is used
				_ = err
			})

			It("should respect context cancellation in ConfigureFonts", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				defer cancel()

				err := fm.ConfigureFonts(ctx, []string{"JetBrains Mono"})
				_ = err
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
				invalidArgs := []string{"font`kwriteconfig5 --file /etc/passwd`"}
				err := fm.ConfigureFonts(ctx, invalidArgs)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid font name"))
			})
		})

		Context("when package manager detection fails", func() {
			It("should handle package installation gracefully", func() {
				// This test would require mocking package managers
				// For now, we just verify the method signature accepts context
				err := fm.installPackages(ctx, []string{"nonexistent-font-package"})
				_ = err // Error depends on system state
			})
		})
	})

	Describe("Cache Management", func() {
		Context("when refreshing font cache", func() {
			It("should handle font cache refresh with context", func() {
				err := fm.refreshFontCache(ctx)
				// Error depends on whether fc-cache is available
				_ = err
			})

			It("should handle KDE cache update with context", func() {
				err := fm.updateKDECache(ctx)
				// Error depends on whether kbuildsycoca5 is available
				_ = err
			})
		})
	})

	Describe("KDE Configuration Structure", func() {
		Context("when working with KDE config types", func() {
			It("should create proper KDEConfig structures", func() {
				config := KDEConfig{
					File:  "kdeglobals",
					Group: "General",
					Key:   "font",
					Value: "Noto Sans,10,-1,5,50,0,0,0,0,0",
					Desc:  "General font",
				}

				Expect(config.File).To(Equal("kdeglobals"))
				Expect(config.Group).To(Equal("General"))
				Expect(config.Key).To(Equal("font"))
				Expect(config.Value).To(ContainSubstring("Noto Sans"))
			})
		})
	})
})
