package repository

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/pkg/mocks"
)

func TestThemeRepository_GetGlobalTheme(t *testing.T) {
	t.Run("should return global theme when exists", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		mockRepo.Set("global_theme", "Tokyo Night")

		themeRepo := NewThemeRepository(mockRepo)
		theme, err := themeRepo.GetGlobalTheme()

		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", theme)
	})

	t.Run("should return empty string when no theme set", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()

		themeRepo := NewThemeRepository(mockRepo)
		theme, err := themeRepo.GetGlobalTheme()

		assert.NoError(t, err)
		assert.Equal(t, "", theme)
	})
}

func TestThemeRepository_SetGlobalTheme(t *testing.T) {
	t.Run("should set global theme successfully", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()

		themeRepo := NewThemeRepository(mockRepo)
		err := themeRepo.SetGlobalTheme("Kanagawa")

		assert.NoError(t, err)

		// Verify it was stored
		storedTheme, err := mockRepo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, "Kanagawa", storedTheme)
	})
}

func TestThemeRepository_GetAppTheme(t *testing.T) {
	t.Run("should return app-specific theme when exists", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		mockRepo.Set("app_theme_neovim", "Tokyo Night")

		themeRepo := NewThemeRepository(mockRepo)
		theme, err := themeRepo.GetAppTheme("neovim")

		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", theme)
	})

	t.Run("should fallback to global theme when app theme not found", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		mockRepo.Set("global_theme", "Kanagawa")

		themeRepo := NewThemeRepository(mockRepo)
		theme, err := themeRepo.GetAppTheme("typora")

		assert.NoError(t, err)
		assert.Equal(t, "Kanagawa", theme)
	})

	t.Run("should return empty when neither app nor global theme exists", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()

		themeRepo := NewThemeRepository(mockRepo)
		theme, err := themeRepo.GetAppTheme("unknown")

		assert.NoError(t, err)
		assert.Equal(t, "", theme)
	})
}

func TestThemeRepository_SetAppTheme(t *testing.T) {
	t.Run("should set app theme successfully", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()

		themeRepo := NewThemeRepository(mockRepo)
		err := themeRepo.SetAppTheme("neovim", "Tokyo Night")

		assert.NoError(t, err)

		// Verify it was stored
		storedTheme, err := mockRepo.Get("app_theme_neovim")
		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", storedTheme)
	})
}

func TestThemeRepository_GetAllThemePreferences(t *testing.T) {
	t.Run("should return all preferences successfully", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		mockRepo.Set("global_theme", "Tokyo Night")
		mockRepo.Set("app_theme_neovim", "Kanagawa")
		mockRepo.Set("app_theme_typora", "Dark Theme")
		mockRepo.Set("other_key", "other_value") // Should be ignored

		themeRepo := NewThemeRepository(mockRepo)
		prefs, err := themeRepo.GetAllThemePreferences()

		assert.NoError(t, err)
		assert.NotNil(t, prefs)
		assert.Equal(t, "Tokyo Night", prefs.GlobalTheme)
		assert.Equal(t, 2, len(prefs.AppThemes))
		assert.Equal(t, "Kanagawa", prefs.AppThemes["neovim"])
		assert.Equal(t, "Dark Theme", prefs.AppThemes["typora"])
	})

	t.Run("should handle empty preferences", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()

		themeRepo := NewThemeRepository(mockRepo)
		prefs, err := themeRepo.GetAllThemePreferences()

		assert.NoError(t, err)
		assert.NotNil(t, prefs)
		assert.Equal(t, "", prefs.GlobalTheme)
		assert.Equal(t, 0, len(prefs.AppThemes))
	})
}

func TestThemeRepository_Integration(t *testing.T) {
	t.Run("should handle complete workflow", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		themeRepo := NewThemeRepository(mockRepo)

		// Set global theme
		err := themeRepo.SetGlobalTheme("Tokyo Night")
		assert.NoError(t, err)

		// Set app theme
		err = themeRepo.SetAppTheme("neovim", "Kanagawa")
		assert.NoError(t, err)

		// Get global theme
		globalTheme, err := themeRepo.GetGlobalTheme()
		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", globalTheme)

		// Get app theme
		appTheme, err := themeRepo.GetAppTheme("neovim")
		assert.NoError(t, err)
		assert.Equal(t, "Kanagawa", appTheme)

		// Get theme for app without specific theme (should fallback to global)
		fallbackTheme, err := themeRepo.GetAppTheme("typora")
		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", fallbackTheme)

		// Get all preferences
		prefs, err := themeRepo.GetAllThemePreferences()
		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", prefs.GlobalTheme)
		assert.Equal(t, 1, len(prefs.AppThemes))
		assert.Equal(t, "Kanagawa", prefs.AppThemes["neovim"])
	})
}

func TestThemeRepository_EdgeCases(t *testing.T) {
	t.Run("should handle empty theme names", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		themeRepo := NewThemeRepository(mockRepo)

		// Set empty global theme
		err := themeRepo.SetGlobalTheme("")
		assert.NoError(t, err)

		theme, err := themeRepo.GetGlobalTheme()
		assert.NoError(t, err)
		assert.Equal(t, "", theme)
	})

	t.Run("should handle empty app names", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		themeRepo := NewThemeRepository(mockRepo)

		// Set theme for empty app name
		err := themeRepo.SetAppTheme("", "Tokyo Night")
		assert.NoError(t, err)

		theme, err := themeRepo.GetAppTheme("")
		assert.NoError(t, err)
		assert.Equal(t, "Tokyo Night", theme)
	})

	t.Run("should handle special characters in theme names", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		themeRepo := NewThemeRepository(mockRepo)

		specialTheme := "Theme-With_Special.Characters (v1.0)"
		err := themeRepo.SetGlobalTheme(specialTheme)
		assert.NoError(t, err)

		theme, err := themeRepo.GetGlobalTheme()
		assert.NoError(t, err)
		assert.Equal(t, specialTheme, theme)
	})
}

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

func TestThemeRepository_DatabaseErrorHandling(t *testing.T) {
	t.Run("should propagate actual database errors", func(t *testing.T) {
		// Test with mock that returns database errors
		dbErrorMock := &DatabaseErrorMock{
			shouldFailWithDbError: true,
			errorMessage:          "database connection failed",
		}
		themeRepo := NewThemeRepository(dbErrorMock)

		// This should propagate the database error, not return empty string
		theme, err := themeRepo.GetGlobalTheme()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve global theme preference")
		assert.Contains(t, err.Error(), "database connection failed")
		assert.Equal(t, "", theme)

		// Test app theme database error propagation
		theme, err = themeRepo.GetAppTheme("test-app")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to retrieve app theme preference")
		assert.Contains(t, err.Error(), "database connection failed")
		assert.Equal(t, "", theme)
	})

	t.Run("should distinguish between not found and database errors", func(t *testing.T) {
		// Test with mock that returns "not found" errors (normal behavior)
		dbErrorMock := &DatabaseErrorMock{
			shouldFailWithDbError: false, // Will return "not found" errors
		}
		themeRepo := NewThemeRepository(dbErrorMock)

		// This should return empty string (not found is normal)
		theme, err := themeRepo.GetGlobalTheme()
		assert.NoError(t, err)
		assert.Equal(t, "", theme)

		// App theme should fallback to global theme (which is also not found)
		theme, err = themeRepo.GetAppTheme("test-app")
		assert.NoError(t, err)
		assert.Equal(t, "", theme)
	})

	t.Run("should handle app theme fallback correctly", func(t *testing.T) {
		mockRepo := mocks.NewMockSystemRepository()
		themeRepo := NewThemeRepository(mockRepo)

		// Test normal "not found" behavior (should fallback to global theme)
		theme, err := themeRepo.GetAppTheme("nonexistent-app")
		assert.NoError(t, err)
		assert.Equal(t, "", theme) // Empty because global theme is also not set

		// Set a global theme and test fallback
		err = themeRepo.SetGlobalTheme("Global Theme")
		assert.NoError(t, err)

		theme, err = themeRepo.GetAppTheme("nonexistent-app")
		assert.NoError(t, err)
		assert.Equal(t, "Global Theme", theme) // Should fallback to global

		// Set an app-specific theme
		err = themeRepo.SetAppTheme("test-app", "App Theme")
		assert.NoError(t, err)

		theme, err = themeRepo.GetAppTheme("test-app")
		assert.NoError(t, err)
		assert.Equal(t, "App Theme", theme) // Should return app-specific theme
	})
}

func TestIsNotFoundError(t *testing.T) {
	t.Run("should identify not found errors correctly", func(t *testing.T) {
		// Test various error message patterns
		testCases := []struct {
			error    error
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
			result := isNotFoundError(tc.error)
			if tc.error != nil {
				assert.Equal(t, tc.expected, result, "Error: %v", tc.error.Error())
			} else {
				assert.Equal(t, tc.expected, result, "Error: nil")
			}
		}
	})
}
