package ui

import (
	"github.com/charmbracelet/lipgloss"
)

type Dialog struct {
	Title   string
	Message string
	Buttons []string
	Active  int
}

func (d *Dialog) Render() string {
	// Define button styles
	buttonStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF7DB")).
		Background(lipgloss.Color("#888B7E")).
		Padding(0, 3).
		MarginTop(1)

	activeButtonStyle := buttonStyle.
		Background(lipgloss.Color("#F25D94")).
		Underline(true)

	// Render buttons
	buttons := ""
	for i, btn := range d.Buttons {
		if i == d.Active {
			buttons += activeButtonStyle.Render(btn) + " "
		} else {
			buttons += buttonStyle.Render(btn) + " "
		}
	}

	// Render dialog box
	dialogBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(1, 2).
		Width(50).
		Align(lipgloss.Center)

	return dialogBoxStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Render(d.Title),
			lipgloss.NewStyle().MarginTop(1).Render(d.Message),
			buttons,
		),
	)
}
