package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Layout represents the split view
type Layout struct {
	Width  int
	Height int
	IsWide bool // Determines layout orientation: true = Left/Right, false = Top/Bottom
}

// CalculateLayout determines if the screen is wide or narrow
func (l *Layout) CalculateLayout(width int, height int) {
	l.Width = width
	l.Height = height
}

func (l *Layout) RenderLeftPanel() string {
	return "Left panel content"
}

func (l Layout) Render(taskLog string, terminalOutput string) string {
	taskLogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Foreground(lipgloss.Color("205"))

	terminalOutputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Foreground(lipgloss.Color("240"))

	tasks := lipgloss.NewStyle().
		PaddingLeft(2).
		Render(taskLog)

	if l.IsWide {
		leftWidth := l.Width * 30 / 100
		rightWidth := l.Width * 70 / 100

		leftPanel := taskLogStyle.Width(leftWidth).Render(tasks)
		rightPanel := terminalOutputStyle.Width(rightWidth).Render(terminalOutput)

		return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	}

	topHeight := l.Height * 30 / 100
	bottomHeight := l.Height * 70 / 100

	topPanel := taskLogStyle.Height(topHeight).Render(tasks)
	bottomPanel := terminalOutputStyle.Height(bottomHeight).Render(terminalOutput)

	return lipgloss.JoinVertical(lipgloss.Left, topPanel, bottomPanel)
}
