package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	Layout           Layout
	Dialog           *Dialog
	Mode             string // "normal", "popup", or "quit"
	Popup            *PopupModel
	Shared           map[string][]string // Add Shared to store selections
	Tasks            []string            // List of tasks to display
	TaskLog          string
	TerminalOutput   string
	QuitConfirmation bool
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) View() string {
	if m.QuitConfirmation {
		return "Are you sure you want to quit? (y/n)"
	}

	switch m.Mode {
	case "popup":
		return m.Popup.View()
	default:
		return m.Layout.Render(m.TaskLog, m.TerminalOutput)
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit

		case "up", "k":
			if m.Dialog != nil {
				m.Dialog.Active = max(0, m.Dialog.Active-1)
				return m, nil
			}

		case "down", "j":
			if m.Dialog != nil {
				m.Dialog.Active = min(len(m.Dialog.Buttons)-1, m.Dialog.Active+1)
				return m, nil
			}

		case "enter":
			if m.Dialog != nil {
				selected := m.Dialog.Buttons[m.Dialog.Active]
				m.TaskLog += "Selected: " + selected + "\n"
				m.Dialog = nil // Hide dialog after selection
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.Layout.Width = msg.Width
		m.Layout.Height = msg.Height
	}

	return m, nil
}
