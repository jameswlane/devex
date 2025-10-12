package setup

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/charmbracelet/lipgloss"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// View renders the current setup step as a string for display
func (m *SetupModel) View() string {
	var s string

	// Define styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true).
		Margin(1, 0)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280")).
		Margin(0, 0, 1, 0)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)

	cursorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7C3AED")).
		Bold(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444"))

	switch m.step {
	case StepSystemOverview:
		s = titleStyle.Render("🚀 Welcome to DevEx Setup!")
		s += "\n\n"
		s += subtitleStyle.Render("Let's set up your development environment.")
		s += "\n\n"
		s += "System Information:\n"
		s += fmt.Sprintf("  • OS: %s\n", m.system.detectedPlatform.OS)
		if m.system.detectedPlatform.Distribution != "" {
			s += fmt.Sprintf("  • Distribution: %s\n", m.system.detectedPlatform.Distribution)
		}
		// Always show desktop info, but handle cases where it's not detected
		desktop := m.system.detectedPlatform.DesktopEnv
		if desktop == "unknown" || desktop == "" || desktop == "none" {
			desktop = "not detected"
		}
		s += fmt.Sprintf("  • Desktop: %s\n", desktop)
		s += fmt.Sprintf("  • Architecture: %s\n", m.system.detectedPlatform.Architecture)
		s += "\n"
		s += "Required DevEx plugins:\n"
		for _, plugin := range m.plugins.requiredPlugins {
			s += fmt.Sprintf("  • %s\n", plugin)
		}
		s += "\n"
		if !m.plugins.confirmPlugins {
			s += "Press Enter to download and install plugins, or 'q' to quit."
		} else {
			s += selectedStyle.Render("✓ Ready to proceed")
			s += "\n\nPress Enter to continue."
		}

	case StepPluginInstall:
		s = titleStyle.Render("📦 Installing DevEx Plugins")
		s += "\n\n"
		switch {
		case atomic.LoadInt32(&m.plugins.pluginsInstalling) == 1:
			// Show individual plugin statuses with spinners
			for _, status := range m.plugins.pluginStatuses {
				s += m.renderPluginStatus(status)
				s += "\n"
			}
			s += "\nThis may take a moment. Please wait..."
		case atomic.LoadInt32(&m.plugins.pluginsInstalled) == 1:
			if m.installation.hasErrors {
				// Plugin registry unavailable but that's ok for development
				s += selectedStyle.Render("✓ Plugin system initialized")
				s += "\n\nNote: Plugin registry unavailable - continuing without plugins\n"
			} else {
				// Show final status of all plugins
				for _, status := range m.plugins.pluginStatuses {
					s += m.renderPluginStatus(status)
					s += "\n"
				}
			}
			s += "\nPress Enter to continue with setup."
		case m.installation.hasErrors:
			// Show plugin statuses with errors
			for _, status := range m.plugins.pluginStatuses {
				s += m.renderPluginStatus(status)
				s += "\n"
			}

			s += "\n" + errorStyle.Render("⚠️  Some plugins could not be installed")
			s += "\nDevEx will continue with core functionality."
			s += "\n\nPress Enter to continue setup, or 'q' to quit."
		}

	case StepDesktopApps:
		if len(m.system.desktopApps) == 0 {
			// Skip desktop apps if none available, go to next step
			newModel, _ := m.nextStep()
			return newModel.View()
		}
		s = titleStyle.Render("🖥️  Select Desktop Applications")
		s += "\n\n"
		s += subtitleStyle.Render("Choose additional desktop applications (optional):")
		s += "\n\n"

		for i, app := range m.system.desktopApps {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			selected := " "
			if m.selections.selectedApps[i] {
				selected = selectedStyle.Render("✓")
			}

			s += fmt.Sprintf("%s [%s] %s\n", cursor, selected, app)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"

	case StepLanguages:
		s = titleStyle.Render("📝 Select Programming Languages")
		s += "\n\n"
		s += subtitleStyle.Render("Choose the programming languages you want to install:")
		s += "\n"
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
		s += infoStyle.Render("ℹ️  Languages will be managed using mise (https://mise.jdx.dev)")
		s += "\n"
		s += infoStyle.Render("   Mise will be installed automatically if not present")
		s += "\n\n"

		for i, lang := range m.system.languages {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			checked := " "
			if m.selections.selectedLangs[i] {
				checked = selectedStyle.Render("✓")
			}

			s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, lang)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"

	case StepDatabases:
		s = titleStyle.Render("🗄️  Select Databases")
		s += "\n\n"
		s += subtitleStyle.Render("Choose the databases you want to install:")
		s += "\n"
		infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
		s += infoStyle.Render("ℹ️  Databases will run as Docker containers")
		s += "\n"
		s += infoStyle.Render("   Docker will be installed automatically if not present")
		s += "\n\n"

		for i, db := range m.system.databases {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			checked := " "
			if m.selections.selectedDBs[i] {
				checked = selectedStyle.Render("✓")
			}

			s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, db)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"

	case StepShell:
		// Only show shell selection on compatible systems (Linux/macOS)
		if m.system.detectedPlatform.OS == "windows" {
			newModel, _ := m.nextStep()
			return newModel.View()
		}

		s = titleStyle.Render("🐚 Select Your Shell")
		s += "\n\n"
		s += subtitleStyle.Render("Choose your preferred shell (zsh is recommended):")
		s += "\n\n"

		for i, shell := range m.system.shells {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			selected := " "
			if m.selections.selectedShell == i {
				selected = selectedStyle.Render("●")
			}

			description := ""
			switch shell {
			case "zsh":
				description = " (recommended - modern features, plugins, themes)"
			case "bash":
				description = " (classic - widely compatible)"
			case "fish":
				description = " (user-friendly - smart completions)"
			}

			s += fmt.Sprintf("%s [%s] %s%s\n", cursor, selected, shell, description)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select, Enter to continue"

	case StepTheme:
		s = titleStyle.Render("🎨 Select Your Theme")
		s += "\n\n"
		s += subtitleStyle.Render("Choose a theme for your applications:")
		s += "\n\n"

		for i, theme := range m.system.themes {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			selected := " "
			if m.selections.selectedTheme == i {
				selected = selectedStyle.Render("●")
			}

			themeName := theme
			if len(themeName) > 30 {
				themeName = themeName[:27] + "..."
			}

			s += fmt.Sprintf("%s %s %s\n", cursor, selected, themeName)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select, 'n' to continue"

	case StepGitConfig:
		s = titleStyle.Render("🔧 Git Configuration")
		s += "\n\n"
		s += subtitleStyle.Render("Enter your git configuration details:")
		s += "\n\n"

		// Full Name field
		cursor := " "
		if m.cursor == 0 {
			cursor = cursorStyle.Render(">")
		}
		nameValue := m.git.gitFullName
		if m.git.gitInputActive && m.git.gitInputField == 0 {
			nameValue += "_" // Show cursor
		}
		s += fmt.Sprintf("%s Full Name: %s\n", cursor, nameValue)

		// Email field
		cursor = " "
		if m.cursor == 1 {
			cursor = cursorStyle.Render(">")
		}
		emailValue := m.git.gitEmail
		if m.git.gitInputActive && m.git.gitInputField == 1 {
			emailValue += "_" // Show cursor
		}
		s += fmt.Sprintf("%s Email: %s\n", cursor, emailValue)

		// Show email validation feedback
		if m.git.gitEmail != "" && !isValidEmail(m.git.gitEmail) {
			s += errorStyle.Render("   ⚠️  Email must contain @ and . characters") + "\n"
		}

		s += "\n"
		if m.git.gitInputActive {
			s += "Type your information and press Enter to confirm, Escape to cancel editing"
		} else {
			fullName := strings.TrimSpace(m.git.gitFullName)
			email := strings.TrimSpace(m.git.gitEmail)
			if fullName != "" && email != "" && isValidEmail(email) {
				s += "Use ↑/↓ to navigate, Enter to edit field, 'n' to continue"
			} else {
				s += "Use ↑/↓ to navigate, Enter to edit field, 'n' to continue when both fields are filled with valid email"
			}
		}

	case StepConfirmation:
		s = titleStyle.Render("✅ Confirm Installation")
		s += "\n\n"
		s += "You've selected the following for installation:\n\n"

		s += "🐚 Shell:\n"
		s += fmt.Sprintf("  • %s\n", m.getSelectedShell())
		s += "\n"

		if len(m.getSelectedLanguages()) > 0 {
			s += "📝 Programming Languages:\n"
			for _, lang := range m.getSelectedLanguages() {
				s += fmt.Sprintf("  • %s\n", lang)
			}
			s += "\n"
		}

		if len(m.getSelectedDatabases()) > 0 {
			s += "🗄️  Databases:\n"
			for _, db := range m.getSelectedDatabases() {
				s += fmt.Sprintf("  • %s\n", db)
			}
			s += "\n"
		}

		if len(m.getSelectedDesktopApps()) > 0 {
			s += "🖥️  Desktop Applications:\n"
			for _, app := range m.getSelectedDesktopApps() {
				s += fmt.Sprintf("  • %s\n", app)
			}
			s += "\n"
		}

		s += "Essential terminal tools will also be installed.\n\n"
		s += "Press Enter to start installation, 'p' to go back, or 'q' to quit."

	case StepInstalling:
		s = titleStyle.Render("⚙️  Installing...")
		s += "\n\n"
		s += fmt.Sprintf("Status: %s\n", m.installation.getStatus())
		s += "\n"
		s += m.renderProgressBar()
		s += "\n\n"
		s += "Please wait while we set up your development environment..."

	case StepComplete:
		selectedShell := m.getSelectedShell()

		if m.installation.hasErrors {
			s = titleStyle.Render("⚠️  Setup Completed with Issues")
			s += "\n\n"
			s += fmt.Sprintf("Setup completed but encountered %d issues:\n\n", len(m.installation.installErrors))

			errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
			for _, err := range m.installation.installErrors {
				s += errorStyle.Render("  ❌ "+err) + "\n"
			}
			s += "\n"
		} else {
			s = titleStyle.Render("🎉 Setup Complete!")
			s += "\n\n"
			s += "Your development environment has been successfully set up!\n\n"
		}

		s += "What was attempted:\n"
		s += fmt.Sprintf("  • %s shell with DevEx configuration\n", selectedShell)
		s += "  • Essential development tools\n"
		if len(m.getSelectedLanguages()) > 0 {
			s += "  • Programming languages via mise\n"
		}
		if len(m.getSelectedDatabases()) > 0 {
			s += "  • Database containers via Docker\n"
		}
		if len(m.getSelectedDesktopApps()) > 0 {
			s += "  • Desktop applications\n"
		}
		s += "\n\n"

		if !m.installation.hasErrors {
			if m.shellSwitched {
				s += fmt.Sprintf("Your shell has been switched to %s. Please restart your terminal\n", selectedShell)
				s += fmt.Sprintf("or run 'exec %s' to start using your new environment.\n\n", selectedShell)
			} else {
				s += fmt.Sprintf("Your environment is configured for %s.\n\n", selectedShell)
			}
			s += "To verify mise is working: 'mise list' or 'mise doctor'\n"
			s += "To check Docker: 'docker ps' (if permission denied, run 'newgrp docker' or log out/in)\n\n"
		} else {
			s += "Please review the issues above. You may need to manually complete\n"
			s += fmt.Sprintf("some installations. To activate %s: exec %s\n\n", selectedShell, selectedShell)
			s += "Troubleshooting:\n"
			s += "• Check mise: 'mise doctor' or reinstall with 'curl https://mise.jdx.dev/install.sh | sh'\n"
			s += "• Check Docker: 'sudo systemctl start docker' and run 'newgrp docker' for permissions\n"
			s += "• Reload shell config: 'source ~/.zshrc' (or ~/.bashrc, ~/.config/fish/config.fish)\n\n"
		}

		if logFile := log.GetLogFile(); logFile != "" {
			s += fmt.Sprintf("📋 Installation logs: %s\n", logFile)
			s += "   (Submit this file for debugging if you encounter issues)\n\n"
		}
	}

	return s
}

// renderProgressBar renders a terminal progress bar
func (m *SetupModel) renderProgressBar() string {
	width := ProgressBarWidth
	filled := int(m.installation.getProgress() * float64(width))
	bar := ""

	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	return fmt.Sprintf("[%s] %.0f%%", bar, m.installation.getProgress()*100)
}
