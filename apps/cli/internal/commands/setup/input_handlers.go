package setup

import (
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
)

// handleEnter processes Enter key presses across different setup steps
func (m *SetupModel) handleEnter() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepSystemOverview:
		if !m.plugins.confirmPlugins {
			m.plugins.confirmPlugins = true
			return m, nil
		}
		return m.nextStep()
	case StepPluginInstall:
		if atomic.LoadInt32(&m.plugins.pluginsInstalled) == 1 || m.installation.hasInstallErrors() {
			return m.nextStep()
		}
		return m, nil
	case StepDesktopApps, StepLanguages, StepDatabases, StepShell:
		return m.nextStep()
	case StepTheme:
		// Theme step: Enter should not continue, only 'n' continues
		return m, nil
	case StepGitConfig:
		if !m.git.gitInputActive {
			// Start editing the selected field
			m.git.gitInputActive = true
			m.git.gitInputField = m.cursor
		}
		return m, nil
	case StepConfirmation:
		m.step = StepInstalling
		m.installation.setInstalling(true)
		return m, m.startInstallation()
	case StepInstalling:
		// During installation, Enter key should not do anything
		return m, nil
	case StepComplete:
		// Installation complete, automatic exit already handled
		return m, nil
	default:
		// Log unhandled case instead of panicking
		return m, nil
	}
}

// handleGitInput handles text input for git configuration fields
func (m *SetupModel) handleGitInput(msg tea.KeyMsg) (*SetupModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Finish editing current field
		m.git.gitInputActive = false
		return m, nil
	case "escape":
		// Cancel editing
		m.git.gitInputActive = false
		return m, nil
	case "backspace":
		// Remove last character
		if m.git.gitInputField == 0 && len(m.git.gitFullName) > 0 {
			m.git.gitFullName = m.git.gitFullName[:len(m.git.gitFullName)-1]
		} else if m.git.gitInputField == 1 && len(m.git.gitEmail) > 0 {
			m.git.gitEmail = m.git.gitEmail[:len(m.git.gitEmail)-1]
		}
		return m, nil
	default:
		// Add character to current field
		if len(msg.Runes) > 0 {
			char := msg.Runes[0]
			switch m.git.gitInputField {
			case 0:
				m.git.gitFullName += string(char)
			case 1:
				m.git.gitEmail += string(char)
			}
		}
		return m, nil
	}
}

// handleDown moves the cursor down in list-based steps
func (m *SetupModel) handleDown() (*SetupModel, tea.Cmd) {
	var maxItems int
	switch m.step {
	case StepDesktopApps:
		maxItems = len(m.system.desktopApps)
	case StepLanguages:
		maxItems = len(m.system.languages)
	case StepDatabases:
		maxItems = len(m.system.databases)
	case StepShell:
		maxItems = len(m.system.shells)
	case StepTheme:
		maxItems = len(m.system.themes)
	case StepGitConfig:
		maxItems = 2 // Full name and email
	default:
		return m, nil // No navigation needed for other steps
	}

	if m.cursor < maxItems-1 {
		m.cursor++
	}
	return m, nil
}

// handleSpace toggles selections or sets radio button options
func (m *SetupModel) handleSpace() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepDesktopApps:
		m.selections.selectedApps[m.cursor] = !m.selections.selectedApps[m.cursor]
	case StepLanguages:
		m.selections.selectedLangs[m.cursor] = !m.selections.selectedLangs[m.cursor]
	case StepDatabases:
		m.selections.selectedDBs[m.cursor] = !m.selections.selectedDBs[m.cursor]
	case StepShell:
		m.selections.selectedShell = m.cursor
	case StepTheme:
		m.selections.selectedTheme = m.cursor
	default:
		return m, nil // No selection needed for other steps
	}
	return m, nil
}
