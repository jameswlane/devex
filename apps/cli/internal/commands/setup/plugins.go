package setup

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/spf13/viper"
)

// initializePluginStatuses creates the initial plugin status list
func initializePluginStatuses(plugins []string) []PluginStatus {
	statuses := make([]PluginStatus, len(plugins))
	for i, plugin := range plugins {
		statuses[i] = PluginStatus{
			Name:   plugin,
			Status: "pending",
			Error:  "",
		}
	}
	return statuses
}

// initializeSpinner creates a new spinner for plugin installation
func initializeSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	return s
}

// renderPluginStatus renders a single plugin status line with appropriate icon
func (m *SetupModel) renderPluginStatus(status PluginStatus) string {
	statusStyle := lipgloss.NewStyle()
	actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	nameStyle := lipgloss.NewStyle().Bold(true)

	var icon string
	var action string

	switch status.Status {
	case "pending":
		icon = "⏳"
		action = "waiting"
		statusStyle = statusStyle.Foreground(lipgloss.Color("#6B7280"))
	case "downloading":
		icon = m.plugins.spinner.View()
		action = "downloading"
		if status.TotalBytes > 0 {
			percent := min(100, max(0, float64(status.BytesDownloaded)/float64(status.TotalBytes)*100))
			action += fmt.Sprintf(" %.0f%%", percent)
		}
		statusStyle = statusStyle.Foreground(lipgloss.Color("#3B82F6"))
	case "verifying":
		icon = m.plugins.spinner.View()
		action = "verifying"
		statusStyle = statusStyle.Foreground(lipgloss.Color("#8B5CF6"))
	case "installing":
		icon = m.plugins.spinner.View()
		action = "installing"
		statusStyle = statusStyle.Foreground(lipgloss.Color("#10B981"))
	case "success":
		icon = "✅"
		action = "installed"
		statusStyle = statusStyle.Foreground(lipgloss.Color("#10B981"))
	case "error":
		icon = "❌"
		action = "failed"
		statusStyle = statusStyle.Foreground(lipgloss.Color("#EF4444"))
	default:
		icon = "•"
		action = status.Status
	}

	// Format: [icon] [action] plugin-name
	result := fmt.Sprintf(" %s %s %s",
		statusStyle.Render(icon),
		actionStyle.Render(action),
		nameStyle.Render(status.Name))

	// Add error message if present
	if status.Error != "" && status.Status == "error" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Italic(true)
		result += "\n    " + errorStyle.Render("└─ "+status.Error)
	}

	return result
}

func (m *SetupModel) startPluginInstallation() tea.Cmd {
	atomic.StoreInt32(&m.plugins.pluginsInstalling, 1)
	ctx, cancel := context.WithCancel(context.Background())
	m.plugins.pluginCancel = cancel
	events := make(chan tea.Msg)
	m.plugins.pluginEvents = events
	return tea.Batch(
		m.plugins.spinner.Tick,
		m.runPluginInstallation(ctx, events),
		waitForPluginInstallationEvent(events),
	)
}

func waitForPluginInstallationEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *SetupModel) waitForNextPluginEvent() tea.Cmd {
	if m.plugins.pluginEvents == nil {
		return nil
	}
	return waitForPluginInstallationEvent(m.plugins.pluginEvents)
}

// initializePlugins shares one initialization attempt, including failures, across setup stages.
func (m *SetupModel) initializePlugins(ctx context.Context, callbacks ...bootstrap.PluginProgressCallback) (*bootstrap.PluginBootstrap, error) {
	m.pluginOnce.Do(func() {
		m.pluginBootstrap, m.pluginError = bootstrap.NewPluginBootstrap(viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
		if m.pluginError != nil {
			m.pluginError = fmt.Errorf("create setup plugin system: %w; check permissions on $DEVEX_DATA_DIR/plugins", m.pluginError)
			return
		}
		var callback bootstrap.PluginProgressCallback
		if len(callbacks) > 0 {
			callback = callbacks[0]
		}
		m.pluginError = m.pluginBootstrap.InitializeWithProgress(ctx, callback)
		if m.pluginError == nil {
			installers.InitializeWithPluginBootstrap(m.pluginBootstrap)
		}
	})
	return m.pluginBootstrap, m.pluginError
}

// runPluginInstallation captures input on the update loop. Its command reports
// events only; plugin status changes belong to Update.
func (m *SetupModel) runPluginInstallation(parent context.Context, events chan<- tea.Msg) tea.Cmd {
	requiredPlugins := append([]string(nil), m.plugins.requiredPlugins...)
	timeout := m.plugins.timeout
	return func() tea.Msg {
		defer close(events)
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()
		send := func(msg tea.Msg) {
			select {
			case events <- msg:
			case <-parent.Done():
			}
		}
		allErrors := make([]error, 0, MaxErrorMessages)
		pluginBootstrap, initErr := m.initializePlugins(ctx, func(event bootstrap.PluginProgressEvent) {
			status := string(event.Stage)
			if event.Stage == "done" {
				status = "success"
			}
			send(PluginInstallMsg{
				PluginName: event.PluginName, Status: status, Error: event.Err,
				BytesDownloaded: event.BytesDownloaded, TotalBytes: event.TotalBytes,
			})
		})
		if initErr != nil {
			allErrors = addErrorSafe(allErrors, fmt.Errorf("plugin initialization failed: %w", initErr))
		}

		results := make(map[string]PluginValidationResult)
		if pluginBootstrap != nil && ctx.Err() == nil {
			validator := NewPluginValidator(pluginBootstrap, PluginValidatorConfig{
				VerifyChecksums: true, Concurrency: 4,
				CriticalPlugins:     []string{"tool-shell", "desktop-gnome", "desktop-kde", "tool-git"},
				VerificationTimeout: PluginVerifyTimeout,
			})
			for _, result := range validator.ValidatePlugins(ctx, requiredPlugins).Results {
				results[result.PluginName] = result
			}
		}

		var successfulPlugins []string
		for _, name := range requiredPlugins {
			pluginErr := initErr
			if pluginBootstrap != nil {
				if downloadErr := pluginBootstrap.PluginDownloadError(name); downloadErr != nil {
					pluginErr = downloadErr
				}
			}
			result, validated := results[name]
			if pluginErr == nil && (!validated || !result.IsValid) {
				pluginErr = result.Error
				if pluginErr == nil {
					pluginErr = ctx.Err()
				}
				if pluginErr == nil {
					pluginErr = fmt.Errorf("plugin %s is unavailable", name)
				}
			}
			if pluginErr != nil {
				send(PluginInstallMsg{PluginName: name, Status: "error", Error: pluginErr})
				allErrors = addErrorSafe(allErrors, pluginErr)
			} else {
				send(PluginInstallMsg{PluginName: name, Status: "success"})
				successfulPlugins = append(successfulPlugins, name)
			}
		}
		log.Info("Plugin installation completed", "installed", len(successfulPlugins), "total", len(requiredPlugins))
		send(PluginInstallCompleteMsg{
			Errors: allErrors, SuccessCount: len(successfulPlugins),
			TotalCount: len(requiredPlugins), SuccessfulPlugins: successfulPlugins,
		})
		return nil
	}
}
