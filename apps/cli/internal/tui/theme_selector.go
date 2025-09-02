package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// ThemeSelector represents the theme selection UI model
type ThemeSelector struct {
	appName   string
	themes    []types.Theme
	cursor    int
	selected  int
	confirmed bool
	cancelled bool
}

// NewThemeSelector creates a new theme selector for the given app
func NewThemeSelector(appName string, themes []types.Theme) *ThemeSelector {
	return &ThemeSelector{
		appName:  appName,
		themes:   themes,
		cursor:   0,
		selected: 0,
	}
}

// Init satisfies the tea.Model interface
func (ts *ThemeSelector) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (ts *ThemeSelector) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "up", "k":
			if ts.cursor > 0 {
				ts.cursor--
			}
		case "down", "j":
			if ts.cursor < len(ts.themes)-1 {
				ts.cursor++
			}
		case "enter", " ":
			ts.selected = ts.cursor
			ts.confirmed = true
			return ts, tea.Quit
		case "esc", "q":
			ts.cancelled = true
			return ts, tea.Quit
		}
	}
	return ts, nil
}

// View renders the theme selection UI
func (ts *ThemeSelector) View() string {
	var s strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Margin(1, 0)

	s.WriteString(titleStyle.Render(fmt.Sprintf("🎨 Select Theme for %s", ts.appName)))
	s.WriteString("\n\n")

	// Theme options
	for i, theme := range ts.themes {
		cursor := " "
		if ts.cursor == i {
			cursor = ">"
		}

		selected := " "
		if ts.selected == i {
			selected = "●"
		}

		// Theme info with color preview
		themeInfo := fmt.Sprintf("%s (%s background)",
			theme.Name,
			theme.ThemeBackground,
		)

		if theme.ThemeColor != "" {
			colorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ThemeColor))
			themeInfo = colorStyle.Render(themeInfo)
		}

		s.WriteString(fmt.Sprintf("%s %s %s\n", cursor, selected, themeInfo))
	}

	// Instructions
	s.WriteString("\n")
	s.WriteString("Use ↑/↓ to navigate, Enter/Space to select, Esc to cancel")

	return s.String()
}

// GetSelectedTheme returns the selected theme, if any
func (ts *ThemeSelector) GetSelectedTheme() (types.Theme, bool) {
	if ts.confirmed && ts.selected < len(ts.themes) {
		return ts.themes[ts.selected], true
	}
	return types.Theme{}, false
}

// IsCancelled returns true if the user cancelled the selection
func (ts *ThemeSelector) IsCancelled() bool {
	return ts.cancelled
}

// ShowThemeSelector displays a theme selection UI and returns the selected theme
func ShowThemeSelector(appName string, themes []types.Theme) (types.Theme, error) {
	if len(themes) == 0 {
		return types.Theme{}, fmt.Errorf("no themes available for %s", appName)
	}

	log.Debug("Showing theme selector", "app", appName, "themeCount", len(themes))

	selector := NewThemeSelector(appName, themes)
	program := tea.NewProgram(selector)

	// Ensure proper cleanup even if Run() panics
	defer func() {
		if program != nil {
			program.Kill()
		}
	}()

	finalModel, err := program.Run()
	if err != nil {
		return types.Theme{}, fmt.Errorf("theme selection failed: %w", err)
	}

	finalSelector, ok := finalModel.(*ThemeSelector)
	if !ok {
		return types.Theme{}, fmt.Errorf("invalid model type returned from theme selection")
	}

	if finalSelector.IsCancelled() {
		return types.Theme{}, fmt.Errorf("theme selection cancelled by user")
	}

	selectedTheme, ok := finalSelector.GetSelectedTheme()
	if !ok {
		return types.Theme{}, fmt.Errorf("no theme selected")
	}

	log.Info("Theme selected", "app", appName, "theme", selectedTheme.Name)
	return selectedTheme, nil
}
