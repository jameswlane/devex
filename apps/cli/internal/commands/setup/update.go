package setup

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// Update handles Bubble Tea messages and state updates
func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Special handling for git configuration text input
		if m.step == StepGitConfig && m.git.gitInputActive {
			return m.handleGitInput(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			return m.handleEnter()

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			return m.handleDown()

		case " ":
			return m.handleSpace()

		case "n":
			return m.nextStep()

		case "p":
			return m.prevStep()
		}

	case PluginInstallMsg:
		m.installation.setStatus(msg.Status)
		m.installation.setProgress(msg.Progress)
		if msg.Error != nil {
			m.installation.addError(msg.Error.Error())
		}
		return m, nil

	case PluginInstallCompleteMsg:
		// Update model state based on installation results using atomic operations
		atomic.StoreInt32(&m.plugins.pluginsInstalling, 0)  // Mark installation complete
		m.plugins.successfulPlugins = msg.SuccessfulPlugins // Store successful plugins

		switch {
		case len(msg.Errors) > 0:
			m.installation.hasErrors = true
			for _, err := range msg.Errors {
				m.installation.installErrors = addErrorStringSafe(m.installation.installErrors, err.Error())
			}
			log.Warn("Plugin installation completed with errors", "errors", len(msg.Errors), "successCount", msg.SuccessCount, "totalCount", msg.TotalCount)
			// Check if these are just registry unavailability errors (not critical)
			allRegistryErrors := true
			for _, err := range msg.Errors {
				errStr := err.Error()
				if !strings.Contains(errStr, "registry") && !strings.Contains(errStr, "404") {
					allRegistryErrors = false
					break
				}
			}
			if allRegistryErrors {
				// Registry is unavailable but that's OK for development
				atomic.StoreInt32(&m.plugins.pluginsInstalled, 1)
				log.Info("Plugin system initialized (registry unavailable)")
			}
		case msg.SuccessCount > 0:
			atomic.StoreInt32(&m.plugins.pluginsInstalled, 1) // Mark installation successful
			log.Info("All plugins installed successfully", "successCount", msg.SuccessCount, "totalCount", msg.TotalCount)
		default:
			// No errors but also no successes - this means 0 plugins were actually installed
			log.Warn("Plugin installation completed but no plugins were installed", "successCount", msg.SuccessCount, "totalCount", msg.TotalCount)
			// Don't mark as installed since nothing was actually installed
		}
		// Clear progress
		m.installation.progress = 1.0
		return m, nil

	case InstallProgressMsg:
		m.installation.setStatus(msg.Status)
		m.installation.setProgress(msg.Progress)
		if msg.Progress >= 1.0 {
			m.step = StepComplete
		}
		return m, m.waitForActivity()

	case InstallCompleteMsg:
		m.step = StepComplete
		// Brief delay to show completion message, then quit automatically
		return m, tea.Sequence(
			tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
				return InstallQuitMsg{}
			}),
		)
	case InstallQuitMsg:
		return m, tea.Quit

	case PluginStatusUpdateMsg:
		// Update individual plugin status
		for i, status := range m.plugins.pluginStatuses {
			if status.Name == msg.PluginName {
				m.plugins.pluginStatuses[i].Status = msg.Status
				m.plugins.pluginStatuses[i].Error = msg.Error
				break
			}
		}
		return m, nil

	case spinner.TickMsg:
		// Update all plugin spinners
		cmds := make([]tea.Cmd, 0, len(m.plugins.pluginStatuses))
		for i, status := range m.plugins.pluginStatuses {
			if status.Status == "downloading" || status.Status == "verifying" || status.Status == "installing" {
				var cmd tea.Cmd
				m.plugins.pluginSpinners[i], cmd = m.plugins.pluginSpinners[i].Update(msg)
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}
