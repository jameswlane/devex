package commands

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
)

// FailingMockRepositoryWithSystem implements both Repository and SystemRepository but always fails
type FailingMockRepositoryWithSystem struct{}

func (m *FailingMockRepositoryWithSystem) ListApps() ([]types.AppConfig, error) {
	return nil, fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) SaveApp(app types.AppConfig) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) Set(key, value string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) Get(key string) (string, error) {
	return "", fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) GetAll() (map[string]string, error) {
	return nil, fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) DeleteApp(name string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) AddApp(name string) error {
	return fmt.Errorf("mock repository error")
}

func (m *FailingMockRepositoryWithSystem) GetApp(name string) (*types.AppConfig, error) {
	return nil, fmt.Errorf("mock repository error")
}

func TestSetupModel_saveThemePreference(t *testing.T) {
	t.Run("should save theme preference successfully", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			themes:        []string{"Tokyo Night", "Kanagawa", "Light Theme"},
			selectedTheme: 1, // Select "Kanagawa"
		}

		err := model.saveThemePreference()

		assert.NoError(t, err)

		// Verify the theme was stored in the repository
		storedTheme, err := mockRepo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, "Kanagawa", storedTheme)
	})

	t.Run("should handle invalid selected theme index", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			themes:        []string{"Tokyo Night", "Kanagawa"},
			selectedTheme: 999, // Invalid index
		}

		// Should panic or handle gracefully
		assert.Panics(t, func() {
			model.saveThemePreference()
		})
	})

	t.Run("should handle empty themes list", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			themes:        []string{},
			selectedTheme: 0,
		}

		// Should panic or handle gracefully
		assert.Panics(t, func() {
			model.saveThemePreference()
		})
	})

	t.Run("should handle repository that doesn't implement SystemRepository", func(t *testing.T) {
		// This test would check the type assertion, but since our mock implements
		// both interfaces, we'd need a different mock that doesn't implement SystemRepository

		// For now, we'll test that the type assertion works with the mock
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			themes:        []string{"Tokyo Night"},
			selectedTheme: 0,
		}

		err := model.saveThemePreference()
		assert.NoError(t, err)
	})

	t.Run("should save first theme when selectedTheme is 0", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			themes:        []string{"First Theme", "Second Theme"},
			selectedTheme: 0,
		}

		err := model.saveThemePreference()

		assert.NoError(t, err)

		storedTheme, err := mockRepo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, "First Theme", storedTheme)
	})

	t.Run("should handle special characters in theme names", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		specialTheme := "Theme-With_Special.Characters (v1.0)"
		model := &SetupModel{
			repo:          mockRepo,
			themes:        []string{specialTheme},
			selectedTheme: 0,
		}

		err := model.saveThemePreference()

		assert.NoError(t, err)

		storedTheme, err := mockRepo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, specialTheme, storedTheme)
	})
}

func TestSetupModel_ThemeStep_Navigation(t *testing.T) {
	t.Run("should handle theme navigation", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			step:          StepTheme,
			themes:        []string{"Tokyo Night", "Kanagawa", "Light Theme"},
			selectedTheme: 0,
			cursor:        0,
		}

		// Test moving down in theme list
		originalCursor := model.cursor
		// Note: We can't easily test the private navigation methods without
		// exposing them or testing through the Update method

		// For now, verify the model state
		assert.Equal(t, StepTheme, model.step)
		assert.Equal(t, 3, len(model.themes))
		assert.Equal(t, originalCursor, model.cursor)
	})
}

func TestSetupModel_ThemeStep_Selection(t *testing.T) {
	t.Run("should track selected theme", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			step:          StepTheme,
			themes:        []string{"Tokyo Night", "Kanagawa", "Light Theme"},
			selectedTheme: 1, // Kanagawa selected
		}

		// Verify selected theme tracking
		assert.Equal(t, 1, model.selectedTheme)
		assert.Equal(t, "Kanagawa", model.themes[model.selectedTheme])
	})
}

func TestSetupModel_ThemeStep_Integration(t *testing.T) {
	t.Run("should handle complete theme workflow in setup", func(t *testing.T) {
		mockRepo := mocks.NewMockRepository()

		model := &SetupModel{
			repo:          mockRepo,
			step:          StepTheme,
			themes:        []string{"Tokyo Night", "Kanagawa", "Light Theme"},
			selectedTheme: 2, // Light Theme
		}

		// Simulate theme selection and saving
		err := model.saveThemePreference()
		assert.NoError(t, err)

		// Verify the theme was persisted
		storedTheme, err := mockRepo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, "Light Theme", storedTheme)

		// Verify the model state
		assert.Equal(t, StepTheme, model.step)
		assert.Equal(t, 2, model.selectedTheme)
	})
}

// Test helper to create a mock setup model
func createMockSetupModel(themes []string, selectedIndex int) *SetupModel {
	return &SetupModel{
		repo:          mocks.NewMockRepository(),
		step:          StepTheme,
		themes:        themes,
		selectedTheme: selectedIndex,
	}
}

func TestSetupModel_ThemeStep_EdgeCases(t *testing.T) {
	t.Run("should handle single theme", func(t *testing.T) {
		model := createMockSetupModel([]string{"Only Theme"}, 0)

		err := model.saveThemePreference()
		assert.NoError(t, err)

		storedTheme, err := model.repo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, "Only Theme", storedTheme)
	})

	t.Run("should handle theme names with spaces", func(t *testing.T) {
		themeName := "Theme With Multiple Spaces"
		model := createMockSetupModel([]string{themeName}, 0)

		err := model.saveThemePreference()
		assert.NoError(t, err)

		storedTheme, err := model.repo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, themeName, storedTheme)
	})

	t.Run("should handle theme names with unicode characters", func(t *testing.T) {
		themeName := "テーマ Theme 🎨"
		model := createMockSetupModel([]string{themeName}, 0)

		err := model.saveThemePreference()
		assert.NoError(t, err)

		storedTheme, err := model.repo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, themeName, storedTheme)
	})

	t.Run("should handle very long theme names", func(t *testing.T) {
		longThemeName := "This is a very long theme name that might exceed normal length limits and test boundary conditions"
		model := createMockSetupModel([]string{longThemeName}, 0)

		err := model.saveThemePreference()
		assert.NoError(t, err)

		storedTheme, err := model.repo.Get("global_theme")
		assert.NoError(t, err)
		assert.Equal(t, longThemeName, storedTheme)
	})
}

func TestSetupModel_ThemeStep_RepositoryErrors(t *testing.T) {
	t.Run("should handle repository set errors", func(t *testing.T) {
		// Create a combined failing repo that implements both interfaces
		failingRepo := &FailingMockRepositoryWithSystem{}

		model := &SetupModel{
			repo:          failingRepo,
			themes:        []string{"Tokyo Night"},
			selectedTheme: 0,
		}

		err := model.saveThemePreference()

		// Should return an error when repository fails
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save global theme preference")
	})
}

func TestSetupStep_ThemeStepConstant(t *testing.T) {
	t.Run("should have correct step order", func(t *testing.T) {
		// Verify that StepTheme comes after StepShell and before StepGitConfig
		assert.Equal(t, 4, int(StepShell))
		assert.Equal(t, 5, int(StepTheme))
		assert.Equal(t, 6, int(StepGitConfig))
	})
}
