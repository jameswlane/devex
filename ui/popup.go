package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type PopupModel struct {
	Title    string
	Category string              // Category name (e.g., "Programming Languages")
	Options  []string            // Options for the category
	Selected map[int]bool        // Selected options for this popup
	Cursor   int                 // Current cursor position
	Next     func() *PopupModel  // Function to load the next category
	Shared   map[string][]string // Shared selections for all categories
}

func (p PopupModel) Init() tea.Cmd {
	return nil
}

func (p PopupModel) Update(msg tea.Msg) (PopupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k": // Navigate up
			if p.Cursor > 0 {
				p.Cursor--
			}
		case "down", "j": // Navigate down
			if p.Cursor < len(p.Options)-1 {
				p.Cursor++
			}
		case " ": // Toggle selection
			p.Selected[p.Cursor] = !p.Selected[p.Cursor]
		case "enter": // Confirm selection
			selectedItems := []string{}
			for i, selected := range p.Selected {
				if selected {
					selectedItems = append(selectedItems, p.Options[i])
				}
			}
			p.Shared[p.Category] = selectedItems

			// Debug shared map after selection
			fmt.Printf("Shared after %s: %+v\n", p.Category, p.Shared)

			// Transition to next popup or quit
			if p.Next != nil {
				nextPopup := p.Next()
				return *nextPopup, nil
			}

			return p, tea.Quit
		case "esc": // Cancel popup
			return p, tea.Quit
		}
	}
	return p, nil
}

// View renders the popup
func (p PopupModel) View() string {
	var b strings.Builder

	// Title
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Render(p.Title) + "\n\n")

	// Options
	for i, option := range p.Options {
		cursor := " "
		if i == p.Cursor {
			cursor = ">"
		}

		checkbox := "[ ]"
		if p.Selected[i] {
			checkbox = "[✔]"
		}

		line := lipgloss.NewStyle().
			Render(cursor + " " + checkbox + " " + option)
		b.WriteString(line + "\n")
	}

	// Footer
	b.WriteString("\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑/↓: Navigate  ␣: Select  ⏎: Confirm  Esc: Cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1).
		Render(b.String())
}

func (p *PopupModel) Reset(title, category string, options []string, next func() *PopupModel) {
	p.Title = title
	p.Category = category
	p.Options = options
	p.Cursor = 0
	p.Selected = make(map[int]bool)
	p.Next = next
}
