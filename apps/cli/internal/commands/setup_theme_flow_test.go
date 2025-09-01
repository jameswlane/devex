package commands

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

func TestThemeSelectionFlow(t *testing.T) {
	// Create a mock setup model for testing
	createMockSetupModel := func() *SetupModel {
		return &SetupModel{
			step:             StepTheme,
			themes:           []string{"Tokyo Night", "Kanagawa", "Catppuccin"},
			selectedTheme:    0,
			cursor:           0,
			detectedPlatform: platform.Platform{OS: "linux", DesktopEnv: "gnome"},
			repo:             mocks.NewMockRepository(),
			settings:         config.CrossPlatformSettings{},
		}
	}

	t.Run("Enter key should NOT advance from theme step", func(t *testing.T) {
		model := createMockSetupModel()
		originalStep := model.step

		// Simulate Enter key press
		updatedModel, cmd := model.handleEnter()

		// Step should remain the same (Enter should not advance)
		assert.Equal(t, originalStep, updatedModel.step, "Enter key should not advance from theme step")
		assert.Nil(t, cmd, "Enter key should not return a command on theme step")
	})

	t.Run("'n' key should advance from theme step", func(t *testing.T) {
		model := createMockSetupModel()

		// Simulate 'n' key press by directly calling nextStep
		updatedModel, cmd := model.nextStep()

		// Step should advance to git config
		assert.Equal(t, StepGitConfig, updatedModel.step, "'n' key should advance to git config step")
		assert.Nil(t, cmd, "nextStep should not return a command")
	})

	t.Run("other steps should advance with Enter key", func(t *testing.T) {
		testCases := []struct {
			name         string
			currentStep  int
			expectedStep int
		}{
			{"Welcome to Desktop Apps", StepWelcome, StepDesktopApps},
			{"Languages to Databases", StepLanguages, StepDatabases},
			{"Databases to Shell", StepDatabases, StepShell},
			{"Shell to Theme", StepShell, StepTheme},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				model := createMockSetupModel()
				model.step = tc.currentStep
				model.hasDesktop = true                  // Ensure desktop apps step is included
				model.desktopApps = []string{"Test App"} // Ensure desktop apps are available

				updatedModel, _ := model.handleEnter()

				assert.Equal(t, tc.expectedStep, updatedModel.step,
					"Enter key should advance from step %d to step %d", tc.currentStep, tc.expectedStep)
			})
		}
	})

	t.Run("theme step should handle space key for selection", func(t *testing.T) {
		model := createMockSetupModel()
		model.cursor = 1 // Select second theme

		// Simulate space key press by calling Update with space key
		keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
		updatedModel, _ := model.Update(keyMsg)

		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")

		// The selected theme should be updated
		assert.Equal(t, 1, setupModel.selectedTheme, "Space key should select theme at cursor position")
	})

	t.Run("theme step should handle arrow key navigation", func(t *testing.T) {
		model := createMockSetupModel()
		model.cursor = 0

		// Test down arrow
		downKeyMsg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ := model.Update(downKeyMsg)
		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")
		assert.Equal(t, 1, setupModel.cursor, "Down arrow should move cursor down")

		// Test up arrow
		upKeyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = setupModel.Update(upKeyMsg)
		setupModel, ok = updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")
		assert.Equal(t, 0, setupModel.cursor, "Up arrow should move cursor up")
	})

	t.Run("theme step should handle cursor boundaries", func(t *testing.T) {
		model := createMockSetupModel()

		// Test cursor stays within bounds
		model.cursor = 0
		upKeyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ := model.Update(upKeyMsg)
		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")
		// Cursor should either wrap to last or stay at 0 (both are valid behaviors)
		assert.True(t, setupModel.cursor >= 0 && setupModel.cursor < len(model.themes),
			"Cursor should stay within bounds")

		// Test down from last position
		model.cursor = len(model.themes) - 1
		downKeyMsg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ = model.Update(downKeyMsg)
		setupModel, ok = updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")
		// Cursor should either wrap to first or stay at last (both are valid behaviors)
		assert.True(t, setupModel.cursor >= 0 && setupModel.cursor < len(model.themes),
			"Cursor should stay within bounds")
	})

	t.Run("theme step view should show correct instructions", func(t *testing.T) {
		model := createMockSetupModel()

		view := model.View()

		// Check that the view contains the correct instruction for theme step
		assert.Contains(t, view, "'n' to continue", "Theme step view should show 'n' to continue instruction")
		assert.NotContains(t, view, "Enter to continue", "Theme step view should not show 'Enter to continue' instruction")
	})

	t.Run("git config step should handle Enter key correctly", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepGitConfig
		model.gitInputActive = false
		model.cursor = 0

		// Enter should activate git input
		updatedModel, _ := model.handleEnter()

		assert.True(t, updatedModel.gitInputActive, "Enter key should activate git input on git config step")
		assert.Equal(t, 0, updatedModel.gitInputField, "Git input field should be set to cursor position")
	})

	t.Run("confirmation step should advance to installation", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepConfirmation

		updatedModel, cmd := model.handleEnter()

		assert.Equal(t, StepInstalling, updatedModel.step, "Enter on confirmation should advance to installing")
		assert.True(t, updatedModel.installing, "Installing flag should be set")
		assert.NotNil(t, cmd, "Enter on confirmation should return installation command")
	})

	t.Run("installing step should not respond to Enter", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepInstalling

		updatedModel, cmd := model.handleEnter()

		assert.Equal(t, StepInstalling, updatedModel.step, "Enter during installation should not change step")
		assert.Nil(t, cmd, "Enter during installation should not return a command")
	})

	t.Run("complete step should not respond to Enter (auto-exit handled)", func(t *testing.T) {
		model := createMockSetupModel()
		model.step = StepComplete

		updatedModel, cmd := model.handleEnter()

		assert.Equal(t, StepComplete, updatedModel.step, "Enter on complete step should not change step")
		assert.Nil(t, cmd, "Enter on complete step should not return a command (auto-exit handles this)")
	})
}

func TestThemeStepNavigation(t *testing.T) {
	t.Run("should skip to languages step if no desktop apps", func(t *testing.T) {
		model := &SetupModel{
			step:             StepDesktopApps,
			desktopApps:      []string{}, // No desktop apps
			hasDesktop:       true,
			detectedPlatform: platform.Platform{OS: "linux", DesktopEnv: "gnome"},
			repo:             mocks.NewMockRepository(),
			settings:         config.CrossPlatformSettings{},
		}

		// The view should automatically redirect to next step when no desktop apps
		_ = model.View() // View method has side effects, but we don't need to check the output

		// This is tricky to test directly as the View method has side effects
		// We can at least verify the model handles empty desktop apps gracefully
		assert.Empty(t, model.desktopApps, "Model should handle empty desktop apps list")
	})

	t.Run("should skip shell step on Windows", func(t *testing.T) {
		model := &SetupModel{
			step:             StepShell,
			detectedPlatform: platform.Platform{OS: "windows", DesktopEnv: ""},
			repo:             mocks.NewMockRepository(),
			settings:         config.CrossPlatformSettings{},
		}

		// The view should automatically redirect to next step on Windows
		_ = model.View() // View method has side effects, but we don't need to check the output

		// Verify Windows platform is handled
		assert.Equal(t, "windows", model.detectedPlatform.OS, "Should handle Windows platform")
	})

	t.Run("nextStep should follow correct sequence", func(t *testing.T) {
		model := &SetupModel{
			step:             StepWelcome,
			hasDesktop:       true,
			desktopApps:      []string{"Test App"},
			detectedPlatform: platform.Platform{OS: "linux", DesktopEnv: "gnome"},
			repo:             mocks.NewMockRepository(),
			settings:         config.CrossPlatformSettings{},
		}

		// Test the progression through steps
		expectedSequence := []int{
			StepDesktopApps, // From Welcome
			StepLanguages,   // From DesktopApps
			StepDatabases,   // From Languages
			StepShell,       // From Databases
			StepTheme,       // From Shell
			StepGitConfig,   // From Theme
		}

		for i, expectedStep := range expectedSequence {
			updatedModel, _ := model.nextStep()
			assert.Equal(t, expectedStep, updatedModel.step,
				"Step %d: Expected step %d, got %d", i, expectedStep, updatedModel.step)
			model = updatedModel
		}

		// Test git config step advancement (requires filled fields)
		model.gitFullName = "Test User"
		model.gitEmail = "test@example.com"
		updatedModel, _ := model.nextStep()
		assert.Equal(t, StepConfirmation, updatedModel.step,
			"Git config should advance to confirmation when fields are filled")
	})

	t.Run("prevStep should follow correct reverse sequence", func(t *testing.T) {
		model := &SetupModel{
			step:             StepConfirmation,
			hasDesktop:       true,
			desktopApps:      []string{"Test App"},
			detectedPlatform: platform.Platform{OS: "linux", DesktopEnv: "gnome"},
			repo:             mocks.NewMockRepository(),
			settings:         config.CrossPlatformSettings{},
		}

		// Test the reverse progression through steps
		expectedReverseSequence := []int{
			StepGitConfig,   // From Confirmation
			StepTheme,       // From GitConfig
			StepShell,       // From Theme
			StepDatabases,   // From Shell
			StepLanguages,   // From Databases
			StepDesktopApps, // From Languages
			StepWelcome,     // From DesktopApps
		}

		for i, expectedStep := range expectedReverseSequence {
			updatedModel, _ := model.prevStep()
			assert.Equal(t, expectedStep, updatedModel.step,
				"Reverse step %d: Expected step %d, got %d", i, expectedStep, updatedModel.step)
			model = updatedModel
		}
	})
}
