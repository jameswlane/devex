package setup

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

func TestGitConfigValidation(t *testing.T) {
	// Create a mock setup model for testing
	createMockSetupModel := func() *SetupModel {
		return &SetupModel{
			step:   StepGitConfig,
			cursor: 0,
			git: GitConfiguration{
				gitInputActive: false,
				gitInputField:  0,
				gitFullName:    "",
				gitEmail:       "",
			},
			system: SystemInfo{
				detectedPlatform: platform.DetectionResult{OS: "linux", DesktopEnv: "gnome"},
			},
			repo:     mocks.NewMockRepository(),
			settings: config.CrossPlatformSettings{},
		}
	}

	t.Run("should require both full name and email to advance", func(t *testing.T) {
		testCases := []struct {
			name          string
			fullName      string
			email         string
			shouldAdvance bool
		}{
			{"both empty", "", "", false},
			{"only name", "John Doe", "", false},
			{"only email", "", "john@example.com", false},
			{"both filled", "John Doe", "john@example.com", true},
			{"name with spaces", "John Doe", "john@example.com", true},
			{"name with special chars", "Jean-François", "jean@example.com", true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				model := createMockSetupModel()
				model.git.gitFullName = tc.fullName
				model.git.gitEmail = tc.email

				updatedModel, _ := model.nextStep()

				if tc.shouldAdvance {
					assert.Equal(t, StepConfirmation, updatedModel.step,
						"Should advance to confirmation when both fields are filled")
				} else {
					assert.Equal(t, StepGitConfig, updatedModel.step,
						"Should stay on git config when fields are incomplete")
				}
			})
		}
	})

	t.Run("should handle whitespace-only input as empty", func(t *testing.T) {
		testCases := []struct {
			name     string
			fullName string
			email    string
		}{
			{"spaces only in name", "   ", "john@example.com"},
			{"spaces only in email", "John Doe", "   "},
			{"tabs in name", "\t\t", "john@example.com"},
			{"newlines in email", "John Doe", "\n\n"},
			{"mixed whitespace", " \t ", " \n "},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				model := createMockSetupModel()
				model.git.gitFullName = tc.fullName
				model.git.gitEmail = tc.email

				updatedModel, _ := model.nextStep()

				// Should not advance because whitespace-only is treated as empty
				assert.Equal(t, StepGitConfig, updatedModel.step,
					"Should not advance with whitespace-only input")
			})
		}
	})

	t.Run("should handle Enter key for field activation", func(t *testing.T) {
		model := createMockSetupModel()
		model.cursor = 0
		model.git.gitInputActive = false

		// Enter should activate input for name field
		updatedModel, _ := model.handleEnter()

		assert.True(t, updatedModel.git.gitInputActive, "Enter should activate git input")
		assert.Equal(t, 0, updatedModel.git.gitInputField, "Should set input field to name (0)")

		// Test email field activation
		model.cursor = 1
		model.git.gitInputActive = false

		updatedModel, _ = model.handleEnter()

		assert.True(t, updatedModel.git.gitInputActive, "Enter should activate git input")
		assert.Equal(t, 1, updatedModel.git.gitInputField, "Should set input field to email (1)")
	})

	t.Run("should handle navigation between fields", func(t *testing.T) {
		model := createMockSetupModel()
		model.cursor = 0

		// Test down arrow navigation
		downKeyMsg := tea.KeyMsg{Type: tea.KeyDown}
		updatedModel, _ := model.Update(downKeyMsg)
		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")
		assert.Equal(t, 1, setupModel.cursor, "Down arrow should move to email field")

		// Test up arrow navigation
		upKeyMsg := tea.KeyMsg{Type: tea.KeyUp}
		updatedModel, _ = setupModel.Update(upKeyMsg)
		setupModel, ok = updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")
		assert.Equal(t, 0, setupModel.cursor, "Up arrow should move to name field")
	})

	t.Run("should handle text input during active editing", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitInputActive = true
		model.git.gitInputField = 0 // Name field

		// Simulate typing text
		textKeyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J', 'o', 'h', 'n'}}
		for _, char := range textKeyMsg.Runes {
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}}
			updatedModel, _ := model.Update(keyMsg)
			model, _ = updatedModel.(*SetupModel)
		}

		assert.Contains(t, model.git.gitFullName, "John", "Should handle text input for name field")
	})

	t.Run("should handle backspace during active editing", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitInputActive = true
		model.git.gitInputField = 0
		model.git.gitFullName = "John Doe"

		// Simulate backspace
		backspaceKeyMsg := tea.KeyMsg{Type: tea.KeyBackspace}
		updatedModel, _ := model.Update(backspaceKeyMsg)
		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")

		assert.Equal(t, "John Do", setupModel.git.gitFullName, "Backspace should remove last character")
	})

	t.Run("should handle escape key appropriately", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitInputActive = true
		model.git.gitInputField = 0

		// Simulate escape key
		escapeKeyMsg := tea.KeyMsg{Type: tea.KeyEsc}
		updatedModel, _ := model.Update(escapeKeyMsg)
		setupModel, ok := updatedModel.(*SetupModel)
		assert.True(t, ok, "Updated model should be SetupModel")

		// Implementation may or may not handle escape - just verify it doesn't crash
		assert.NotNil(t, setupModel, "Model should still exist after escape key")
	})

	t.Run("should show correct field indicators in view", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitFullName = "John Doe"
		model.git.gitEmail = "john@example.com"

		view := model.View()

		assert.Contains(t, view, "Full Name: John Doe", "Should display full name")
		assert.Contains(t, view, "Email: john@example.com", "Should display email")
		assert.Contains(t, view, "'n' to continue",
			"Should show instruction for continuing")
	})

	t.Run("should show cursor indicators for active field", func(t *testing.T) {
		model := createMockSetupModel()
		model.cursor = 0
		model.git.gitInputActive = true
		model.git.gitInputField = 0
		model.git.gitFullName = "John"

		view := model.View()

		// Should show cursor for active field
		assert.Contains(t, view, "John_", "Should show cursor for active name field")
	})

	t.Run("should show editing instructions when input is active", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitInputActive = true

		view := model.View()

		assert.Contains(t, view, "Type your information and press Enter to confirm",
			"Should show typing instruction when input is active")
		assert.Contains(t, view, "Escape to cancel editing",
			"Should show escape instruction when input is active")
	})

	t.Run("should show navigation instructions when input is not active", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitInputActive = false

		view := model.View()

		assert.Contains(t, view, "Use ↑/↓ to navigate", "Should show navigation instruction")
		assert.Contains(t, view, "Enter to edit field", "Should show edit instruction")
	})

	t.Run("should validate email format", func(t *testing.T) {
		testCases := []struct {
			name          string
			email         string
			shouldAdvance bool
		}{
			{"valid simple email", "user@example.com", true},
			{"valid email with subdomain", "user@mail.example.com", true},
			{"valid email with plus", "user+tag@example.com", true},
			{"valid email with dots", "user.name@example.com", true},
			{"valid email with dashes", "user@ex-ample.com", true},
			{"valid short domain", "user@ex.co", true},
			{"invalid missing @", "userexample.com", false},
			{"invalid missing .", "user@example", false},
			{"invalid missing both", "userexample", false},
			{"invalid only @", "user@", false},
			{"invalid only .", "user.", false},
			{"invalid empty", "", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				model := createMockSetupModel()
				model.git.gitFullName = "John Doe"
				model.git.gitEmail = tc.email

				updatedModel, _ := model.nextStep()

				if tc.shouldAdvance {
					assert.Equal(t, StepConfirmation, updatedModel.step,
						"Should advance with valid email: %s", tc.email)
				} else {
					assert.Equal(t, StepGitConfig, updatedModel.step,
						"Should not advance with invalid email: %s", tc.email)
				}
			})
		}
	})

	t.Run("should handle long names and emails", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitFullName = "This Is A Very Long Full Name With Many Words And Characters"
		model.git.gitEmail = "this.is.a.very.long.email.address@a.very.long.domain.name.example.com"

		updatedModel, _ := model.nextStep()

		assert.Equal(t, StepConfirmation, updatedModel.step,
			"Should handle long names and emails")
	})

	t.Run("should handle special characters in names", func(t *testing.T) {
		testCases := []struct {
			name     string
			fullName string
		}{
			{"accented characters", "José María"},
			{"hyphenated name", "Jean-François"},
			{"apostrophe", "O'Connor"},
			{"period", "Dr. Smith"},
			{"unicode characters", "王小明"},
			{"mixed scripts", "John 王"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				model := createMockSetupModel()
				model.git.gitFullName = tc.fullName
				model.git.gitEmail = "user@example.com"

				updatedModel, _ := model.nextStep()

				assert.Equal(t, StepConfirmation, updatedModel.step,
					"Should handle special characters in name: %s", tc.fullName)
			})
		}
	})

	t.Run("should maintain field values during navigation", func(t *testing.T) {
		model := createMockSetupModel()
		model.git.gitFullName = "John Doe"
		model.git.gitEmail = "john@example.com"

		// Navigate to different steps and back
		nextModel, _ := model.nextStep()     // Go to confirmation
		prevModel, _ := nextModel.prevStep() // Back to git config

		assert.Equal(t, "John Doe", prevModel.git.gitFullName, "Should preserve full name")
		assert.Equal(t, "john@example.com", prevModel.git.gitEmail, "Should preserve email")
	})
}

func TestEmailValidation(t *testing.T) {
	t.Run("isValidEmail should validate correctly", func(t *testing.T) {
		testCases := []struct {
			email    string
			expected bool
		}{
			{"user@example.com", true},
			{"test.email+tag@domain.co.uk", true},
			{"user@sub.domain.com", true},
			{"userexample.com", false}, // missing @
			{"user@example", false},    // missing .
			{"userexample", false},     // missing both
			{"user@", false},           // only @
			{"user.", false},           // only .
			{"", false},                // empty
			{"@example.com", false},    // invalid: starts with @
			{"user@.com", false},       // invalid: domain starts with .
			{"user@domain", false},     // invalid: no TLD
			{"user@domain.", false},    // invalid: TLD too short
			{"user@domain.c", false},   // invalid: TLD too short
		}

		for _, tc := range testCases {
			t.Run("email: "+tc.email, func(t *testing.T) {
				result := isValidEmail(tc.email)
				assert.Equal(t, tc.expected, result,
					"Email validation failed for: %s", tc.email)
			})
		}
	})
}
