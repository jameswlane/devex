package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("ThemeSelector", func() {
	Describe("NewThemeSelector", func() {
		It("should create new theme selector with default values", func() {
			themes := []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
				{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
			}

			selector := NewThemeSelector("neovim", themes)

			Expect(selector.appName).To(Equal("neovim"))
			Expect(selector.themes).To(Equal(themes))
			Expect(selector.cursor).To(Equal(0))
			Expect(selector.selected).To(Equal(0))
			Expect(selector.confirmed).To(BeFalse())
			Expect(selector.cancelled).To(BeFalse())
		})
	})

	Describe("Update Navigation", func() {
		var themes []types.Theme
		var selector *ThemeSelector

		BeforeEach(func() {
			themes = []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
				{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
				{Name: "Light Theme", ThemeColor: "#FFFFFF", ThemeBackground: "light"},
			}
			selector = NewThemeSelector("neovim", themes)
		})

		It("should move cursor down with j key", func() {
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(1))
			Expect(cmd).To(BeNil())
		})

		It("should move cursor up with k key", func() {
			selector.cursor = 1 // Start at position 1
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(0))
			Expect(cmd).To(BeNil())
		})

		It("should not move cursor below 0", func() {
			selector.cursor = 0
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyUp})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(0))
			Expect(cmd).To(BeNil())
		})

		It("should not move cursor above max", func() {
			selector.cursor = 2 // Last position
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyDown})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(2))
			Expect(cmd).To(BeNil())
		})
	})

	Describe("Update Selection", func() {
		var themes []types.Theme
		var selector *ThemeSelector

		BeforeEach(func() {
			themes = []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
				{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
			}
			selector = NewThemeSelector("neovim", themes)
			selector.cursor = 1
		})

		It("should select theme with enter", func() {
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyEnter})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.selected).To(Equal(1))
			Expect(updatedSelector.confirmed).To(BeTrue())
			Expect(updatedSelector.cancelled).To(BeFalse())

			// Should return quit command
			Expect(cmd).ToNot(BeNil())
		})

		It("should select theme with space", func() {
			selector.confirmed = false // Reset
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.selected).To(Equal(1))
			Expect(updatedSelector.confirmed).To(BeTrue())
			Expect(updatedSelector.cancelled).To(BeFalse())

			// Should return quit command
			Expect(cmd).ToNot(BeNil())
		})
	})

	Describe("Update Cancellation", func() {
		var themes []types.Theme
		var selector *ThemeSelector

		BeforeEach(func() {
			themes = []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
			}
			selector = NewThemeSelector("neovim", themes)
		})

		It("should cancel with escape", func() {
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyEsc})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cancelled).To(BeTrue())
			Expect(updatedSelector.confirmed).To(BeFalse())

			// Should return quit command
			Expect(cmd).ToNot(BeNil())
		})

		It("should cancel with q", func() {
			selector.cancelled = false // Reset
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cancelled).To(BeTrue())
			Expect(updatedSelector.confirmed).To(BeFalse())

			// Should return quit command
			Expect(cmd).ToNot(BeNil())
		})
	})

	Describe("GetSelectedTheme", func() {
		var themes []types.Theme
		var selector *ThemeSelector

		BeforeEach(func() {
			themes = []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
				{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "dark"},
			}
			selector = NewThemeSelector("neovim", themes)
		})

		It("should return selected theme when confirmed", func() {
			selector.selected = 1
			selector.confirmed = true

			theme, ok := selector.GetSelectedTheme()

			Expect(ok).To(BeTrue())
			Expect(theme.Name).To(Equal("Kanagawa"))
			Expect(theme.ThemeColor).To(Equal("#16161D"))
			Expect(theme.ThemeBackground).To(Equal("dark"))
		})

		It("should return false when not confirmed", func() {
			selector.confirmed = false

			theme, ok := selector.GetSelectedTheme()

			Expect(ok).To(BeFalse())
			Expect(theme).To(Equal(types.Theme{}))
		})

		It("should handle invalid selected index", func() {
			selector.selected = 999 // Invalid index
			selector.confirmed = true

			theme, ok := selector.GetSelectedTheme()

			Expect(ok).To(BeFalse())
			Expect(theme).To(Equal(types.Theme{}))
		})
	})

	Describe("IsCancelled", func() {
		var selector *ThemeSelector

		BeforeEach(func() {
			selector = NewThemeSelector("neovim", []types.Theme{})
		})

		It("should return false when not cancelled", func() {
			Expect(selector.IsCancelled()).To(BeFalse())
		})

		It("should return true when cancelled", func() {
			selector.cancelled = true
			Expect(selector.IsCancelled()).To(BeTrue())
		})
	})

	Describe("View", func() {
		var themes []types.Theme
		var selector *ThemeSelector

		BeforeEach(func() {
			themes = []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
				{Name: "Kanagawa", ThemeColor: "#16161D", ThemeBackground: "light"},
			}
			selector = NewThemeSelector("neovim", themes)
		})

		It("should render view with themes", func() {
			view := selector.View()

			// Should contain app name
			Expect(view).To(ContainSubstring("neovim"))

			// Should contain theme names
			Expect(view).To(ContainSubstring("Tokyo Night"))
			Expect(view).To(ContainSubstring("Kanagawa"))

			// Should contain background types
			Expect(view).To(ContainSubstring("dark background"))
			Expect(view).To(ContainSubstring("light background"))

			// Should contain instructions
			Expect(view).To(ContainSubstring("↑/↓ to navigate"))
			Expect(view).To(ContainSubstring("Enter/Space to select"))
			Expect(view).To(ContainSubstring("Esc to cancel"))
		})

		It("should show cursor on current position", func() {
			selector.cursor = 1
			view := selector.View()

			// The view should show cursor positioning but testing exact formatting
			// is brittle since it depends on lipgloss styling
			Expect(view).ToNot(BeEmpty())
		})
	})

	Describe("ShowThemeSelector Edge Cases", func() {
		It("should handle empty themes list", func() {
			themes := []types.Theme{}

			_, err := ShowThemeSelector("neovim", themes)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no themes available"))
		})

		It("should handle nil themes list", func() {
			_, err := ShowThemeSelector("neovim", nil)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no themes available"))
		})
	})

	Describe("Init", func() {
		It("should return nil command", func() {
			themes := []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
			}

			selector := NewThemeSelector("neovim", themes)
			cmd := selector.Init()

			// Init should return nil
			Expect(cmd).To(BeNil())
		})
	})

	Describe("HandleUnknownMessages", func() {
		var themes []types.Theme
		var selector *ThemeSelector

		BeforeEach(func() {
			themes = []types.Theme{
				{Name: "Tokyo Night", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
			}
			selector = NewThemeSelector("neovim", themes)
		})

		It("should ignore non-key messages", func() {
			// Send a mouse message (not a KeyMsg)
			model, cmd := selector.Update("some other message")

			updatedSelector := model.(*ThemeSelector)
			// State should remain unchanged
			Expect(updatedSelector.cursor).To(Equal(0))
			Expect(updatedSelector.confirmed).To(BeFalse())
			Expect(updatedSelector.cancelled).To(BeFalse())
			Expect(cmd).To(BeNil())
		})

		It("should ignore unknown key messages", func() {
			model, cmd := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

			updatedSelector := model.(*ThemeSelector)
			// State should remain unchanged
			Expect(updatedSelector.cursor).To(Equal(0))
			Expect(updatedSelector.confirmed).To(BeFalse())
			Expect(updatedSelector.cancelled).To(BeFalse())
			Expect(cmd).To(BeNil())
		})
	})

	Describe("Arrow Key Navigation", func() {
		var themes []types.Theme
		var selector *ThemeSelector

		BeforeEach(func() {
			themes = []types.Theme{
				{Name: "Theme 1", ThemeColor: "#1A1B26", ThemeBackground: "dark"},
				{Name: "Theme 2", ThemeColor: "#16161D", ThemeBackground: "dark"},
				{Name: "Theme 3", ThemeColor: "#FFFFFF", ThemeBackground: "light"},
			}
			selector = NewThemeSelector("test-app", themes)
		})

		It("should handle arrow key navigation", func() {
			// Test down arrow
			model, _ := selector.Update(tea.KeyMsg{Type: tea.KeyDown})
			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(1))

			// Test up arrow
			model, _ = updatedSelector.Update(tea.KeyMsg{Type: tea.KeyUp})
			updatedSelector = model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(0))
		})

		It("should handle vim-style navigation", func() {
			// Test j (down)
			model, _ := selector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
			updatedSelector := model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(1))

			// Test k (up)
			model, _ = updatedSelector.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
			updatedSelector = model.(*ThemeSelector)
			Expect(updatedSelector.cursor).To(Equal(0))
		})
	})
})
