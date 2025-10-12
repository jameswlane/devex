package setup

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// nextStep advances to the next step in the setup wizard
func (m *SetupModel) nextStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
	case StepSystemOverview:
		m.step = StepPluginInstall
		// Initialize plugin statuses and spinners before starting installation
		m.plugins.pluginStatuses = make([]PluginStatus, len(m.plugins.requiredPlugins))
		m.plugins.pluginSpinners = make([]spinner.Model, len(m.plugins.requiredPlugins))
		for i, pluginName := range m.plugins.requiredPlugins {
			m.plugins.pluginStatuses[i] = PluginStatus{
				Name:   pluginName,
				Status: "pending",
				Error:  "",
			}
			// Create individual spinner for each plugin
			m.plugins.pluginSpinners[i] = spinner.New()
			m.plugins.pluginSpinners[i].Spinner = spinner.Dot
			m.plugins.pluginSpinners[i].Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		}
		// Start plugin installation
		return m, m.startPluginInstallation()
	case StepPluginInstall:
		// Check if we have desktop apps to show first
		if m.system.hasDesktop && len(m.system.desktopApps) > 0 {
			m.step = StepDesktopApps
		} else {
			m.step = StepLanguages
		}
	case StepDesktopApps:
		m.step = StepLanguages
	case StepLanguages:
		m.step = StepDatabases
	case StepDatabases:
		// Only show shell selection on compatible systems
		if m.system.detectedPlatform.OS != "windows" {
			m.step = StepShell
		} else {
			m.step = StepTheme // Windows gets theme selection without shell
		}
	case StepShell:
		m.step = StepTheme
	case StepTheme:
		m.step = StepGitConfig
	case StepGitConfig:
		// Only proceed if both fields are filled and email is valid
		fullName := strings.TrimSpace(m.git.gitFullName)
		email := strings.TrimSpace(m.git.gitEmail)
		if fullName != "" && email != "" && isValidEmail(email) {
			m.step = StepConfirmation
		}
		// If fields are empty or email is invalid, stay on git config step
	case StepConfirmation:
		m.step = StepInstalling
	case StepInstalling:
		m.step = StepComplete
	case StepComplete:
		// Already at final step, no next step
		return m, nil
	default:
		// Unknown step, stay at current step
		return m, nil
	}
	return m, nil
}

// prevStep goes back to the previous step in the setup wizard
func (m *SetupModel) prevStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
	case StepPluginInstall:
		m.step = StepSystemOverview
		m.plugins.confirmPlugins = false
	case StepDesktopApps:
		m.step = StepPluginInstall
	case StepLanguages:
		if m.system.hasDesktop && len(m.system.desktopApps) > 0 {
			m.step = StepDesktopApps
		} else {
			m.step = StepPluginInstall
		}
	case StepDatabases:
		m.step = StepLanguages
	case StepShell:
		m.step = StepDatabases
	case StepTheme:
		if m.system.detectedPlatform.OS != "windows" {
			m.step = StepShell
		} else {
			m.step = StepDatabases
		}
	case StepGitConfig:
		m.step = StepTheme
	case StepConfirmation:
		m.step = StepGitConfig
	case StepInstalling:
		// During installation, don't allow going back
		return m, nil
	case StepComplete:
		// After completion, don't allow going back
		return m, nil
	default:
		// Unknown step, stay at current step
		return m, nil
	}
	return m, nil
}
