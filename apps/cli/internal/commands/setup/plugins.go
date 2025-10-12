package setup

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/log"
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
	return tea.Batch(
		// Mark installation as in progress and start spinners
		func() tea.Msg {
			atomic.StoreInt32(&m.plugins.pluginsInstalling, 1)
			return m.plugins.spinner.Tick
		},
		// Start the actual plugin installation immediately
		m.runPluginInstallation(),
	)
}

func (m *SetupModel) runPluginInstallation() tea.Cmd {
	return func() tea.Msg {
		// Pre-allocate error collection with bounds checking for memory safety
		allErrors := make([]error, 0, MaxErrorMessages)

		// Initialize plugin bootstrap with smart download fallback
		// Try with downloads enabled first, but fall back to skip downloads if registry is unavailable
		pluginBootstrap, err := bootstrap.NewPluginBootstrap(false)
		if err != nil {
			log.Warn("Plugin download failed, trying with downloads disabled", "error", err)
			pluginBootstrap, err = bootstrap.NewPluginBootstrap(true) // Skip downloads
		}
		if err != nil {
			log.Error("Failed to initialize plugin system", err)
			allErrors = addErrorSafe(allErrors, fmt.Errorf("failed to initialize plugin system: %w", err))
			return PluginInstallCompleteMsg{
				Errors:            allErrors,
				SuccessCount:      0,
				TotalCount:        len(m.plugins.requiredPlugins),
				SuccessfulPlugins: []string{},
			}
		}

		// Note: Plugin downloader uses logger adapter that writes to log file
		// No need to call SetSilent() - logger is already configured for TUI compatibility

		// Create context with configurable timeout for plugin installation
		ctx, cancel := context.WithTimeout(context.Background(), m.plugins.timeout)
		defer cancel()

		log.Info("Initializing plugin system with required plugins", "plugins", m.plugins.requiredPlugins, "timeout", m.plugins.timeout.String())

		// Update plugin statuses to show downloading
		for i := range m.plugins.requiredPlugins {
			if i < len(m.plugins.pluginStatuses) {
				m.plugins.pluginStatuses[i].Status = "downloading"
			}
		}

		// Initialize plugins and collect any errors
		if err := pluginBootstrap.Initialize(ctx); err != nil {
			log.Error("Failed to bootstrap plugins", err)
			allErrors = addErrorSafe(allErrors, fmt.Errorf("plugin initialization failed: %w", err))

			// Check for context cancellation/timeout
			if ctx.Err() != nil {
				timeoutErr := fmt.Errorf("plugin installation timed out after %v. This may indicate network issues or system resource constraints. Try increasing DEVEX_PLUGIN_TIMEOUT or check your network connection: %w", m.plugins.timeout, ctx.Err())
				allErrors = addErrorSafe(allErrors, timeoutErr)
			}
			// Continue to verify what plugins were installed despite errors
		}

		// Don't immediately update plugin statuses - let the simulation complete first
		// The final status updates will be sent after the delay
		var finalSuccessfulPlugins []string
		if err == nil {
			// If bootstrap succeeded, plugins are considered successful
			finalSuccessfulPlugins = append(finalSuccessfulPlugins, m.plugins.requiredPlugins...)
			log.Info("Plugin system initialized successfully", "plugins", len(finalSuccessfulPlugins))
		} else {
			log.Warn("Plugin system initialization had issues", "error", err)
		}

		// The Initialize function already downloads required plugins based on platform detection
		// Only perform validation if we have a working plugin system (not in skip-download mode)
		var validationSummary *ValidationSummary

		// Check if we're in a mode where plugins should be validated
		// Skip validation if we know the registry is unavailable
		skipValidation := false
		if len(allErrors) > 0 {
			// Check if all errors are registry-related
			for _, err := range allErrors {
				if strings.Contains(err.Error(), "registry") || strings.Contains(err.Error(), "404") {
					skipValidation = true
					break
				}
			}
		}

		if !skipValidation && len(m.plugins.requiredPlugins) > 0 {
			// Perform enhanced plugin validation with security and performance improvements
			validatorConfig := PluginValidatorConfig{
				VerifyChecksums:     true,  // Enable checksum verification
				VerifySignatures:    false, // Enable in Phase 2
				Concurrency:         4,     // Reasonable parallel verification limit
				FailOnCritical:      false, // Don't fail early for missing plugins in dev
				CriticalPlugins:     []string{"tool-shell", "desktop-gnome", "desktop-kde", "tool-git"},
				VerificationTimeout: PluginVerifyTimeout, // Per-plugin timeout
			}

			validator := NewPluginValidator(pluginBootstrap, validatorConfig)
			validationSummary = validator.ValidatePlugins(ctx, m.plugins.requiredPlugins)
		} else {
			// Create a dummy validation summary for skipped validation
			validationSummary = &ValidationSummary{
				TotalPlugins:   len(m.plugins.requiredPlugins),
				ValidPlugins:   0,
				InvalidPlugins: 0,
				ValidationTime: 0,
				Results:        []PluginValidationResult{},
				Errors:         []error{},
			}
			log.Info("Skipping plugin validation - registry unavailable")
		}

		log.Info("Plugin validation completed",
			"totalPlugins", validationSummary.TotalPlugins,
			"validPlugins", validationSummary.ValidPlugins,
			"invalidPlugins", validationSummary.InvalidPlugins,
			"validationTime", validationSummary.ValidationTime,
		)

		// Add validation errors to the error collection, but be graceful about plugin availability
		if validationSummary.InvalidPlugins > 0 && validationSummary.ValidPlugins == 0 {
			// If all plugins failed validation and none succeeded, it's likely a registry/network issue
			// Log as warning instead of hard error
			log.Warn("Plugin validation failed - likely due to registry unavailability",
				"invalidPlugins", validationSummary.InvalidPlugins,
				"validPlugins", validationSummary.ValidPlugins,
			)
			// Don't add these as hard errors that would fail the setup
		} else {
			// Add validation errors to the error collection for genuine plugin issues
			for _, err := range validationSummary.Errors {
				allErrors = addErrorSafe(allErrors, err)
			}
		}

		// Log detailed results for debugging
		for _, result := range validationSummary.Results {
			if result.IsValid {
				log.Info("Plugin validated successfully",
					"name", result.PluginName,
					"checksumValid", result.ChecksumValid,
					"validationTime", result.ValidationTime,
				)
			} else {
				log.Warn("Plugin validation failed",
					"name", result.PluginName,
					"error", result.Error,
					"validationTime", result.ValidationTime,
				)
			}
		}

		// Extract successful plugin names from validation results
		var successfulPlugins []string
		for _, result := range validationSummary.Results {
			if result.IsValid {
				successfulPlugins = append(successfulPlugins, result.PluginName)
			}
		}

		// Update final plugin statuses immediately
		if err == nil && len(allErrors) == 0 {
			// Mark all plugins as successful if no errors
			for i := range m.plugins.requiredPlugins {
				if i < len(m.plugins.pluginStatuses) {
					m.plugins.pluginStatuses[i].Status = "success"
				}
			}
		} else {
			// Mark plugins based on actual validation results
			for i := range m.plugins.requiredPlugins {
				if i < len(m.plugins.pluginStatuses) {
					if validationSummary != nil && validationSummary.InvalidPlugins > 0 {
						// Show as warning if plugins just aren't available (registry issue)
						m.plugins.pluginStatuses[i].Status = "error"
						m.plugins.pluginStatuses[i].Error = "Registry unavailable"
					} else {
						m.plugins.pluginStatuses[i].Status = "success"
					}
				}
			}
		}

		return PluginInstallCompleteMsg{
			Errors:            allErrors,
			SuccessCount:      validationSummary.ValidPlugins,
			TotalCount:        len(m.plugins.requiredPlugins),
			SuccessfulPlugins: finalSuccessfulPlugins,
		}
	}
}
