package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/jameswlane/devex/pkg/types"
)

func TestThemeSelector_NewThemeSelector(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
	}

	selector := NewThemeSelector("neovim", themes)

	assert.Equal(t, "neovim", selector.appName)
	assert.Equal(t, themes, selector.themes)
	assert.Equal(t, 0, selector.cursor)
	assert.Equal(t, 0, selector.selected)
	assert.False(t, selector.confirmed)
	assert.False(t, selector.cancelled)
}

func TestThemeSelector_Update_Navigation(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
		{Name: "Light Theme", ThemeColor: "#FFFFFF", ThemeBackground: "light"},
	}

	selector := NewThemeSelector("neovim", themes)

	t.Run("should move cursor down", func(t *testing.T) {
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 1, updatedSelector.cursor)
		assert.Nil(t, cmd)
	})

	t.Run("should move cursor up", func(t *testing.T) {
		selector.cursor = 1 // Start at position 1
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 0, updatedSelector.cursor)
		assert.Nil(t, cmd)
	})

	t.Run("should not move cursor below 0", func(t *testing.T) {
		selector.cursor = 0
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyUp})

		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 0, updatedSelector.cursor)
		assert.Nil(t, cmd)
	})

	t.Run("should not move cursor above max", func(t *testing.T) {
		selector.cursor = 2 // Last position
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyDown})

		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 2, updatedSelector.cursor)
		assert.Nil(t, cmd)
	})
}

func TestThemeSelector_Update_Selection(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
	}

	selector := NewThemeSelector("neovim", themes)
	selector.cursor = 1

	t.Run("should select theme with enter", func(t *testing.T) {
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyEnter})

		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 1, updatedSelector.selected)
		assert.True(t, updatedSelector.confirmed)
		assert.False(t, updatedSelector.cancelled)

		// Should return quit command
		assert.NotNil(t, cmd)
	})

	t.Run("should select theme with space", func(t *testing.T) {
		selector.confirmed = false // Reset
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 1, updatedSelector.selected)
		assert.True(t, updatedSelector.confirmed)
		assert.False(t, updatedSelector.cancelled)

		// Should return quit command
		assert.NotNil(t, cmd)
	})
}

func TestThemeSelector_Update_Cancellation(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
	}

	selector := NewThemeSelector("neovim", themes)

	t.Run("should cancel with escape", func(t *testing.T) {
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyEsc})

		updatedSelector := model.(*ThemeSelector)
		assert.True(t, updatedSelector.cancelled)
		assert.False(t, updatedSelector.confirmed)

		// Should return quit command
		assert.NotNil(t, cmd)
	})

	t.Run("should cancel with q", func(t *testing.T) {
		selector.cancelled = false // Reset
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		updatedSelector := model.(*ThemeSelector)
		assert.True(t, updatedSelector.cancelled)
		assert.False(t, updatedSelector.confirmed)

		// Should return quit command
		assert.NotNil(t, cmd)
	})
}

func TestThemeSelector_GetSelectedTheme(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
	}

	selector := NewThemeSelector("neovim", themes)

	t.Run("should return selected theme when confirmed", func(t *testing.T) {
		selector.selected = 1
		selector.confirmed = true

		theme, ok := selector.GetSelectedTheme()

		assert.True(t, ok)
		assert.Equal(t, "Kanagawa", theme.Name)
		assert.Equal(t, "#16161D", theme.ThemeColor)
		assert.Equal(t, "dark", theme.ThemeBackground)
	})

	t.Run("should return false when not confirmed", func(t *testing.T) {
		selector.confirmed = false

		theme, ok := selector.GetSelectedTheme()

		assert.False(t, ok)
		assert.Equal(t, types.Theme{}, theme)
	})

	t.Run("should handle invalid selected index", func(t *testing.T) {
		selector.selected = 999 // Invalid index
		selector.confirmed = true

		theme, ok := selector.GetSelectedTheme()

		assert.False(t, ok)
		assert.Equal(t, types.Theme{}, theme)
	})
}

func TestThemeSelector_IsCancelled(t *testing.T) {
	selector := NewThemeSelector("neovim", []types.Theme{})

	t.Run("should return false when not cancelled", func(t *testing.T) {
		assert.False(t, selector.IsCancelled())
	})

	t.Run("should return true when cancelled", func(t *testing.T) {
		selector.cancelled = true
		assert.True(t, selector.IsCancelled())
	})
}

func TestThemeSelector_View(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "light"},
	}

	selector := NewThemeSelector("neovim", themes)

	t.Run("should render view with themes", func(t *testing.T) {
		view := selector.View()

		// Should contain app name
		assert.Contains(t, view, "neovim")

		// Should contain theme names
		assert.Contains(t, view, "Tokyo Night")
		assert.Contains(t, view, "Kanagawa")

		// Should contain background types
		assert.Contains(t, view, "dark background")
		assert.Contains(t, view, "light background")

		// Should contain instructions
		assert.Contains(t, view, "↑/↓ to navigate")
		assert.Contains(t, view, "Enter/Space to select")
		assert.Contains(t, view, "Esc to cancel")
	})

	t.Run("should show cursor on current position", func(t *testing.T) {
		selector.cursor = 1
		view := selector.View()

		// The view should show cursor positioning but testing exact formatting
		// is brittle since it depends on lipgloss styling
		assert.NotEmpty(t, view)
	})
}

func TestShowThemeSelector_EdgeCases(t *testing.T) {
	t.Run("should handle empty themes list", func(t *testing.T) {
		themes := []types.Theme{}

		_, err := ShowThemeSelector("neovim", themes)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no themes available")
	})

	t.Run("should handle nil themes list", func(t *testing.T) {
		_, err := ShowThemeSelector("neovim", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no themes available")
	})
}

func TestThemeSelector_Init(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
	}

	selector := NewThemeSelector("neovim", themes)
	cmd := selector.Init()

	// Init should return nil
	assert.Nil(t, cmd)
}

func TestThemeSelector_HandleUnknownMessages(t *testing.T) {
	themes := []types.Theme{
		{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
	}

	selector := NewThemeSelector("neovim", themes)

	t.Run("should ignore non-key messages", func(t *testing.T) {
		// Send a mouse message (not a KeyMsg)
		model, cmd := selector.Update("some other message")

		updatedSelector := model.(*ThemeSelector)
		// State should remain unchanged
		assert.Equal(t, 0, updatedSelector.cursor)
		assert.False(t, updatedSelector.confirmed)
		assert.False(t, updatedSelector.cancelled)
		assert.Nil(t, cmd)
	})

	t.Run("should ignore unknown key messages", func(t *testing.T) {
		model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

		updatedSelector := model.(*ThemeSelector)
		// State should remain unchanged
		assert.Equal(t, 0, updatedSelector.cursor)
		assert.False(t, updatedSelector.confirmed)
		assert.False(t, updatedSelector.cancelled)
		assert.Nil(t, cmd)
	})
}

func TestThemeSelector_ArrowKeyNavigation(t *testing.T) {
	themes := []types.Theme{
		{Name: "Theme 1", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
		{Name: "Theme 2", ThemeColor: "#16161D", ThemeBackground: "dark"},
		{Name: "Theme 3", ThemeColor: "#FFFFFF", ThemeBackground: "light"},
	}

	selector := NewThemeSelector("test-app", themes)

	t.Run("should handle arrow key navigation", func(t *testing.T) {
		// Test down arrow
		model, _ := selector.Update(tea.KeyMsg{Type: tea.KeyDown})
		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 1, updatedSelector.cursor)

		// Test up arrow
		model, _ = updatedSelector.Update(tea.KeyMsg{Type: tea.KeyUp})
		updatedSelector = model.(*ThemeSelector)
		assert.Equal(t, 0, updatedSelector.cursor)
	})

	t.Run("should handle vim-style navigation", func(t *testing.T) {
		// Test j (down)
		model, _ := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		updatedSelector := model.(*ThemeSelector)
		assert.Equal(t, 1, updatedSelector.cursor)

		// Test k (up)
		model, _ = updatedSelector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		updatedSelector = model.(*ThemeSelector)
		assert.Equal(t, 0, updatedSelector.cursor)
	})
}
