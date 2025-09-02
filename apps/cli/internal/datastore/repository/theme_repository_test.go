package repository

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Theme Repository", func() {
	Describe("GetGlobalTheme", func() {
		Context("when global theme exists", func() {
			It("should return the global theme", func() {
				mockRepo := mocks.NewMockSystemRepository()
				mockRepo.Set("global_theme", "Tokyo Night")

				themeRepo := NewThemeRepository(mockRepo)
				theme, err := themeRepo.GetGlobalTheme()

				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal("Tokyo Night"))
			})
		})

		Context("when no theme is set", func() {
			It("should return empty string", func() {
				mockRepo := mocks.NewMockSystemRepository()

				themeRepo := NewThemeRepository(mockRepo)
				theme, err := themeRepo.GetGlobalTheme()

				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal(""))
			})
		})
	})

	Describe("SetGlobalTheme", func() {
		It("should set global theme successfully", func() {
			mockRepo := mocks.NewMockSystemRepository()

			themeRepo := NewThemeRepository(mockRepo)
			err := themeRepo.SetGlobalTheme("Kanagawa")

			Expect(err).ToNot(HaveOccurred())

			// Verify it was stored
			storedTheme, err := mockRepo.Get("global_theme")
			Expect(err).ToNot(HaveOccurred())
			Expect(storedTheme).To(Equal("Kanagawa"))
		})
	})

	Describe("GetAppTheme", func() {
		Context("when app-specific theme exists", func() {
			It("should return the app-specific theme", func() {
				mockRepo := mocks.NewMockSystemRepository()
				mockRepo.Set("app_theme_neovim", "Tokyo Night")

				themeRepo := NewThemeRepository(mockRepo)
				theme, err := themeRepo.GetAppTheme("neovim")

				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal("Tokyo Night"))
			})
		})

		Context("when app theme not found", func() {
			It("should fallback to global theme", func() {
				mockRepo := mocks.NewMockSystemRepository()
				mockRepo.Set("global_theme", "Kanagawa")

				themeRepo := NewThemeRepository(mockRepo)
				theme, err := themeRepo.GetAppTheme("typora")

				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal("Kanagawa"))
			})
		})

		Context("when neither app nor global theme exists", func() {
			It("should return empty string", func() {
				mockRepo := mocks.NewMockSystemRepository()

				themeRepo := NewThemeRepository(mockRepo)
				theme, err := themeRepo.GetAppTheme("unknown")

				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal(""))
			})
		})
	})

	Describe("SetAppTheme", func() {
		It("should set app theme successfully", func() {
			mockRepo := mocks.NewMockSystemRepository()

			themeRepo := NewThemeRepository(mockRepo)
			err := themeRepo.SetAppTheme("neovim", "Tokyo Night")

			Expect(err).ToNot(HaveOccurred())

			// Verify it was stored
			storedTheme, err := mockRepo.Get("app_theme_neovim")
			Expect(err).ToNot(HaveOccurred())
			Expect(storedTheme).To(Equal("Tokyo Night"))
		})
	})

	Describe("GetAllThemePreferences", func() {
		Context("when preferences exist", func() {
			It("should return all preferences successfully", func() {
				mockRepo := mocks.NewMockSystemRepository()
				mockRepo.Set("global_theme", "Tokyo Night")
				mockRepo.Set("app_theme_neovim", "Kanagawa")
				mockRepo.Set("app_theme_typora", "Dark Theme")
				mockRepo.Set("other_key", "other_value") // Should be ignored

				themeRepo := NewThemeRepository(mockRepo)
				prefs, err := themeRepo.GetAllThemePreferences()

				Expect(err).ToNot(HaveOccurred())
				Expect(prefs).ToNot(BeNil())
				Expect(prefs.GlobalTheme).To(Equal("Tokyo Night"))
				Expect(len(prefs.AppThemes)).To(Equal(2))
				Expect(prefs.AppThemes["neovim"]).To(Equal("Kanagawa"))
				Expect(prefs.AppThemes["typora"]).To(Equal("Dark Theme"))
			})
		})

		Context("when no preferences exist", func() {
			It("should handle empty preferences", func() {
				mockRepo := mocks.NewMockSystemRepository()

				themeRepo := NewThemeRepository(mockRepo)
				prefs, err := themeRepo.GetAllThemePreferences()

				Expect(err).ToNot(HaveOccurred())
				Expect(prefs).ToNot(BeNil())
				Expect(prefs.GlobalTheme).To(Equal(""))
				Expect(len(prefs.AppThemes)).To(Equal(0))
			})
		})
	})

	Describe("Integration Tests", func() {
		It("should handle complete workflow", func() {
			mockRepo := mocks.NewMockSystemRepository()
			themeRepo := NewThemeRepository(mockRepo)

			// Set global theme
			err := themeRepo.SetGlobalTheme("Tokyo Night")
			Expect(err).ToNot(HaveOccurred())

			// Set app theme
			err = themeRepo.SetAppTheme("neovim", "Kanagawa")
			Expect(err).ToNot(HaveOccurred())

			// Get global theme
			globalTheme, err := themeRepo.GetGlobalTheme()
			Expect(err).ToNot(HaveOccurred())
			Expect(globalTheme).To(Equal("Tokyo Night"))

			// Get app theme
			appTheme, err := themeRepo.GetAppTheme("neovim")
			Expect(err).ToNot(HaveOccurred())
			Expect(appTheme).To(Equal("Kanagawa"))

			// Get theme for app without specific theme (should fallback to global)
			fallbackTheme, err := themeRepo.GetAppTheme("typora")
			Expect(err).ToNot(HaveOccurred())
			Expect(fallbackTheme).To(Equal("Tokyo Night"))

			// Get all preferences
			prefs, err := themeRepo.GetAllThemePreferences()
			Expect(err).ToNot(HaveOccurred())
			Expect(prefs.GlobalTheme).To(Equal("Tokyo Night"))
			Expect(len(prefs.AppThemes)).To(Equal(1))
			Expect(prefs.AppThemes["neovim"]).To(Equal("Kanagawa"))
		})
	})

	Describe("Edge Cases", func() {
		Context("empty theme names", func() {
			It("should handle empty theme names", func() {
				mockRepo := mocks.NewMockSystemRepository()
				themeRepo := NewThemeRepository(mockRepo)

				// Set empty global theme
				err := themeRepo.SetGlobalTheme("")
				Expect(err).ToNot(HaveOccurred())

				theme, err := themeRepo.GetGlobalTheme()
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal(""))
			})
		})

		Context("empty app names", func() {
			It("should handle empty app names", func() {
				mockRepo := mocks.NewMockSystemRepository()
				themeRepo := NewThemeRepository(mockRepo)

				// Set theme for empty app name
				err := themeRepo.SetAppTheme("", "Tokyo Night")
				Expect(err).ToNot(HaveOccurred())

				theme, err := themeRepo.GetAppTheme("")
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal("Tokyo Night"))
			})
		})

		Context("special characters in theme names", func() {
			It("should handle special characters", func() {
				mockRepo := mocks.NewMockSystemRepository()
				themeRepo := NewThemeRepository(mockRepo)

				specialTheme := "Theme-With_Special.Characters (v1.0)"
				err := themeRepo.SetGlobalTheme(specialTheme)
				Expect(err).ToNot(HaveOccurred())

				theme, err := themeRepo.GetGlobalTheme()
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal(specialTheme))
			})
		})
	})

	Describe("Database Error Handling", func() {
		var dbErrorMock *DatabaseErrorMock

		Context("when database errors occur", func() {
			BeforeEach(func() {
				dbErrorMock = &DatabaseErrorMock{
					shouldFailWithDbError: true,
					errorMessage:          "database connection failed",
				}
			})

			It("should propagate actual database errors for GetGlobalTheme", func() {
				themeRepo := NewThemeRepository(dbErrorMock)

				theme, err := themeRepo.GetGlobalTheme()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to retrieve global theme preference"))
				Expect(err.Error()).To(ContainSubstring("database connection failed"))
				Expect(theme).To(Equal(""))
			})

			It("should propagate actual database errors for GetAppTheme", func() {
				themeRepo := NewThemeRepository(dbErrorMock)

				theme, err := themeRepo.GetAppTheme("test-app")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to retrieve app theme preference"))
				Expect(err.Error()).To(ContainSubstring("database connection failed"))
				Expect(theme).To(Equal(""))
			})
		})

		Context("when keys are not found", func() {
			BeforeEach(func() {
				dbErrorMock = &DatabaseErrorMock{
					shouldFailWithDbError: false, // Will return "not found" errors
				}
			})

			It("should distinguish between not found and database errors for GetGlobalTheme", func() {
				themeRepo := NewThemeRepository(dbErrorMock)

				theme, err := themeRepo.GetGlobalTheme()
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal(""))
			})

			It("should distinguish between not found and database errors for GetAppTheme", func() {
				themeRepo := NewThemeRepository(dbErrorMock)

				theme, err := themeRepo.GetAppTheme("test-app")
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal(""))
			})
		})

		Context("app theme fallback behavior", func() {
			It("should handle fallback correctly", func() {
				mockRepo := mocks.NewMockSystemRepository()
				themeRepo := NewThemeRepository(mockRepo)

				// Test normal "not found" behavior (should fallback to global theme)
				theme, err := themeRepo.GetAppTheme("nonexistent-app")
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal("")) // Empty because global theme is also not set

				// Set a global theme and test fallback
				err = themeRepo.SetGlobalTheme("Global Theme")
				Expect(err).ToNot(HaveOccurred())

				theme, err = themeRepo.GetAppTheme("nonexistent-app")
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal("Global Theme")) // Should fallback to global

				// Set an app-specific theme
				err = themeRepo.SetAppTheme("test-app", "App Theme")
				Expect(err).ToNot(HaveOccurred())

				theme, err = themeRepo.GetAppTheme("test-app")
				Expect(err).ToNot(HaveOccurred())
				Expect(theme).To(Equal("App Theme")) // Should return app-specific theme
			})
		})
	})

	Describe("isNotFoundError helper", func() {
		It("should identify not found errors correctly", func() {
			testCases := []struct {
				err      error
				expected bool
			}{
				{fmt.Errorf("key 'test' not found"), true},
				{fmt.Errorf("something not found"), true},
				{fmt.Errorf("database connection failed"), false},
				{fmt.Errorf("permission denied"), false},
				{fmt.Errorf("timeout error"), false},
				{nil, false},
			}

			for _, tc := range testCases {
				result := isNotFoundError(tc.err)
				if tc.err != nil {
					Expect(result).To(Equal(tc.expected), "Error: %v", tc.err.Error())
				} else {
					Expect(result).To(Equal(tc.expected), "Error: nil")
				}
			}
		})
	})
})

// DatabaseErrorMock implements SystemRepository but returns database errors
type DatabaseErrorMock struct {
	shouldFailWithDbError bool
	errorMessage          string
}

func (m *DatabaseErrorMock) Get(key string) (string, error) {
	if m.shouldFailWithDbError {
		return "", fmt.Errorf("%s", m.errorMessage)
	}
	return "", fmt.Errorf("key '%s' not found", key) // Normal "not found" error
}

func (m *DatabaseErrorMock) Set(key, value string) error {
	if m.shouldFailWithDbError {
		return fmt.Errorf("%s", m.errorMessage)
	}
	return nil
}

func (m *DatabaseErrorMock) GetAll() (map[string]string, error) {
	if m.shouldFailWithDbError {
		return nil, fmt.Errorf("%s", m.errorMessage)
	}
	return make(map[string]string), nil
}
