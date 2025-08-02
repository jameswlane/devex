package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
)

func init() {
	Register(NewSetupCmd)
}

func NewSetupCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive guided setup for your development environment",
		Long: `The setup command provides an interactive, guided installation experience.

Choose from popular programming languages, databases, and applications to create
a customized development environment tailored to your needs.

The setup process includes:
  • Programming language selection (Node.js, Python, Go, Ruby, etc.)
  • Database installation (PostgreSQL, MySQL, Redis)
  • Essential development tools and terminal applications
  • Desktop applications (if desktop environment detected)
  • Automatic dependency management and ordering`,
		Example: `  # Start interactive guided setup
  devex setup

  # Run setup with verbose output
  devex setup --verbose
  
  # Run non-interactive setup with defaults
  devex setup --non-interactive`,
		Run: func(cmd *cobra.Command, args []string) {
			runGuidedSetup(repo, settings)
		},
	}

	// Add flags
	cmd.Flags().Bool("non-interactive", false, "Run automated setup without user interaction")

	// Bind flags to viper
	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))

	return cmd
}

// SetupModel represents the state of our guided setup UI
type SetupModel struct {
	step             int
	shells           []string
	languages        []string
	databases        []string
	desktopApps      []string
	selectedShell    int
	selectedLangs    map[int]bool
	selectedDBs      map[int]bool
	selectedApps     map[int]bool
	gitFullName      string
	gitEmail         string
	gitInputField    int  // 0 = full name, 1 = email
	gitInputActive   bool // true when editing a field
	cursor           int
	installing       bool
	installStatus    string
	progress         float64
	installErrors    []string
	hasErrors        bool
	shellSwitched    bool
	hasDesktop       bool
	detectedPlatform platform.Platform
	repo             types.Repository
	settings         config.CrossPlatformSettings
}

// setupSteps defines the guided setup process
const (
	StepWelcome     = iota
	StepDesktopApps // Only if desktop detected & non-default apps available
	StepLanguages
	StepDatabases
	StepShell     // Only for compatible systems (Linux/macOS)
	StepGitConfig // Full name & email for git configuration
	StepConfirmation
	StepInstalling
	StepComplete
)

// Constants for setup configuration
const (
	// UI Constants
	ProgressBarWidth     = 50
	WaitActivityInterval = 100 // milliseconds

	// Default selections for automated setup
	DefaultShellIndex      = 0 // zsh (first option)
	DefaultNodeJSIndex     = 0 // Node.js
	DefaultPythonIndex     = 1 // Python
	DefaultPostgreSQLIndex = 0 // PostgreSQL

	// File permissions
	DirectoryPermissions   = 0755
	ExecutablePermissions  = 0755
	RegularFilePermissions = 0644

	// Database configuration
	PostgreSQLPort = "5432:5432"
	MySQLPort      = "3306:3306"
	RedisPort      = "6379:6379"
)

func runGuidedSetup(repo types.Repository, settings config.CrossPlatformSettings) {
	// Update settings with runtime flags
	settings.Verbose = viper.GetBool("verbose")

	log.Info("Starting guided setup process", "verbose", settings.Verbose, "logFile", log.GetLogFile())

	// Check if we should run in interactive mode (default: yes)
	if !isInteractiveMode() {
		log.Info("Non-interactive mode requested, running automated setup")
		if err := runAutomatedSetup(repo, settings); err != nil {
			log.Error("Automated setup failed", err)
			os.Exit(1) // or handle appropriately
		}
		return
	}

	// Detect platform and desktop environment first
	plat := platform.DetectPlatform()

	// Initialize the setup model
	model := &SetupModel{
		step:             StepWelcome,
		selectedShell:    DefaultShellIndex, // Default to zsh (first option)
		selectedLangs:    make(map[int]bool),
		selectedDBs:      make(map[int]bool),
		selectedApps:     make(map[int]bool),
		installErrors:    make([]string, 0),
		hasErrors:        false,
		shellSwitched:    false,
		hasDesktop:       plat.DesktopEnv != "none",
		detectedPlatform: plat,
		repo:             repo,
		settings:         settings,
		shells: []string{
			"zsh",
			"bash",
			"fish",
		},
		languages: []string{
			"Node.js",
			"Python",
			"Go",
			"Ruby on Rails",
			"PHP",
			"Java",
			"Rust",
			"Elixir",
		},
		databases: []string{
			"PostgreSQL",
			"MySQL",
			"Redis",
		},
	}

	// Set desktop apps based on platform and config (non-default apps only)
	if model.hasDesktop {
		model.desktopApps = model.getAvailableDesktopApps()
	}

	// Start the Bubble Tea program
	program := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		log.Error("Error running guided setup", err)
		os.Exit(1)
	}
}

// Init satisfies the tea.Model interface
func (m *SetupModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model state
func (m *SetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Special handling for git configuration text input
		if m.step == StepGitConfig && m.gitInputActive {
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

	case InstallProgressMsg:
		m.installStatus = msg.Status
		m.progress = msg.Progress
		if msg.Progress >= 1.0 {
			m.step = StepComplete
		}
		return m, m.waitForActivity()

	case InstallCompleteMsg:
		m.step = StepComplete
		return m, nil
	}

	return m, nil
}

// View renders the current UI state
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

	switch m.step {
	case StepWelcome:
		s = titleStyle.Render("🚀 Welcome to DevEx Setup!")
		s += "\n\n"
		s += subtitleStyle.Render("Let's set up your development environment with the tools you need.")
		s += "\n\n"
		s += "This guided setup will help you install:\n"
		s += "  • Shell configuration and tools\n"
		s += "  • Programming languages and tools\n"
		s += "  • Databases (via Docker)\n"
		s += "  • Essential development applications\n"
		s += "  • Desktop applications (if applicable)\n"
		s += "\n\n"
		s += "Press Enter to continue, or 'q' to quit."

	case StepDesktopApps:
		if len(m.desktopApps) == 0 {
			// Skip desktop apps if none available, go to next step
			newModel, _ := m.nextStep()
			return newModel.View()
		}
		s = titleStyle.Render("🖥️  Select Desktop Applications")
		s += "\n\n"
		s += subtitleStyle.Render("Choose additional desktop applications (optional):")
		s += "\n\n"

		for i, app := range m.desktopApps {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			selected := " "
			if m.selectedApps[i] {
				selected = selectedStyle.Render("✓")
			}

			s += fmt.Sprintf("%s [%s] %s\n", cursor, selected, app)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"

	case StepLanguages:
		s = titleStyle.Render("📝 Select Programming Languages")
		s += "\n\n"
		s += subtitleStyle.Render("Choose the programming languages you want to install (via mise):")
		s += "\n\n"

		for i, lang := range m.languages {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			checked := " "
			if m.selectedLangs[i] {
				checked = selectedStyle.Render("✓")
			}

			s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, lang)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"

	case StepDatabases:
		s = titleStyle.Render("🗄️  Select Databases")
		s += "\n\n"
		s += subtitleStyle.Render("Choose the databases you want to install (via Docker):")
		s += "\n\n"

		for i, db := range m.databases {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			checked := " "
			if m.selectedDBs[i] {
				checked = selectedStyle.Render("✓")
			}

			s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, db)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"

	case StepShell:
		// Only show shell selection on compatible systems (Linux/macOS)
		if m.detectedPlatform.OS == "windows" {
			newModel, _ := m.nextStep()
			return newModel.View()
		}

		s = titleStyle.Render("🐚 Select Your Shell")
		s += "\n\n"
		s += subtitleStyle.Render("Choose your preferred shell (zsh is recommended):")
		s += "\n\n"

		for i, shell := range m.shells {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			selected := " "
			if m.selectedShell == i {
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
		nameValue := m.gitFullName
		if m.gitInputActive && m.gitInputField == 0 {
			nameValue += "_" // Show cursor
		}
		s += fmt.Sprintf("%s Full Name: %s\n", cursor, nameValue)

		// Email field
		cursor = " "
		if m.cursor == 1 {
			cursor = cursorStyle.Render(">")
		}
		emailValue := m.gitEmail
		if m.gitInputActive && m.gitInputField == 1 {
			emailValue += "_" // Show cursor
		}
		s += fmt.Sprintf("%s Email: %s\n", cursor, emailValue)

		s += "\n\n"
		if m.gitInputActive {
			s += "Type your information and press Enter to confirm, Escape to cancel editing"
		} else {
			s += "Use ↑/↓ to navigate, Enter to edit field, 'n' to continue when both fields are filled"
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
		s += fmt.Sprintf("Status: %s\n", m.installStatus)
		s += "\n"
		s += m.renderProgressBar()
		s += "\n\n"
		s += "Please wait while we set up your development environment..."

	case StepComplete:
		selectedShell := m.getSelectedShell()

		if m.hasErrors {
			s = titleStyle.Render("⚠️  Setup Completed with Issues")
			s += "\n\n"
			s += fmt.Sprintf("Setup completed but encountered %d issues:\n\n", len(m.installErrors))

			errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
			for _, err := range m.installErrors {
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

		if !m.hasErrors {
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
		s += "Press 'q' to exit."
	}

	return s
}

// Helper methods for handling user input and navigation
func (m *SetupModel) handleEnter() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepWelcome:
		return m.nextStep()
	case StepDesktopApps, StepLanguages, StepDatabases, StepShell:
		return m.nextStep()
	case StepGitConfig:
		if !m.gitInputActive {
			// Start editing the selected field
			m.gitInputActive = true
			m.gitInputField = m.cursor
		}
		return m, nil
	case StepConfirmation:
		m.step = StepInstalling
		m.installing = true
		return m, m.startInstallation()
	default:
		panic("unhandled default case")
	}
}

// handleGitInput handles text input for git configuration fields
func (m *SetupModel) handleGitInput(msg tea.KeyMsg) (*SetupModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Finish editing current field
		m.gitInputActive = false
		return m, nil
	case "escape":
		// Cancel editing
		m.gitInputActive = false
		return m, nil
	case "backspace":
		// Remove last character
		if m.gitInputField == 0 && len(m.gitFullName) > 0 {
			m.gitFullName = m.gitFullName[:len(m.gitFullName)-1]
		} else if m.gitInputField == 1 && len(m.gitEmail) > 0 {
			m.gitEmail = m.gitEmail[:len(m.gitEmail)-1]
		}
		return m, nil
	default:
		// Add character to current field
		if len(msg.Runes) > 0 {
			char := msg.Runes[0]
			switch m.gitInputField {
			case 0:
				m.gitFullName += string(char)
			case 1:
				m.gitEmail += string(char)
			}
		}
		return m, nil
	}
}

func (m *SetupModel) handleDown() (*SetupModel, tea.Cmd) {
	var maxItems int
	switch m.step {
	case StepDesktopApps:
		maxItems = len(m.desktopApps)
	case StepLanguages:
		maxItems = len(m.languages)
	case StepDatabases:
		maxItems = len(m.databases)
	case StepShell:
		maxItems = len(m.shells)
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

func (m *SetupModel) handleSpace() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepDesktopApps:
		m.selectedApps[m.cursor] = !m.selectedApps[m.cursor]
	case StepLanguages:
		m.selectedLangs[m.cursor] = !m.selectedLangs[m.cursor]
	case StepDatabases:
		m.selectedDBs[m.cursor] = !m.selectedDBs[m.cursor]
	case StepShell:
		m.selectedShell = m.cursor
	default:
		return m, nil // No selection needed for other steps
	}
	return m, nil
}

func (m *SetupModel) nextStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
	case StepWelcome:
		// Check if we have desktop apps to show first
		if m.hasDesktop && len(m.desktopApps) > 0 {
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
		if m.detectedPlatform.OS != "windows" {
			m.step = StepShell
		} else {
			m.step = StepGitConfig
		}
	case StepShell:
		m.step = StepGitConfig
	case StepGitConfig:
		// Only proceed if both fields are filled
		if strings.TrimSpace(m.gitFullName) != "" && strings.TrimSpace(m.gitEmail) != "" {
			m.step = StepConfirmation
		}
		// If fields are empty, stay on git config step
	case StepConfirmation:
		m.step = StepInstalling
	default:
		panic("unhandled default case")
	}
	return m, nil
}

func (m *SetupModel) prevStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
	case StepDesktopApps:
		m.step = StepWelcome
	case StepLanguages:
		if m.hasDesktop && len(m.desktopApps) > 0 {
			m.step = StepDesktopApps
		} else {
			m.step = StepWelcome
		}
	case StepDatabases:
		m.step = StepLanguages
	case StepShell:
		m.step = StepDatabases
	case StepGitConfig:
		if m.detectedPlatform.OS != "windows" {
			m.step = StepShell
		} else {
			m.step = StepDatabases
		}
	case StepConfirmation:
		m.step = StepGitConfig
	default:
		panic("unhandled default case")
	}
	return m, nil
}

// Helper methods for getting selected items

func (m *SetupModel) getSelectedLanguages() []string {
	var selected []string
	for i, lang := range m.languages {
		if m.selectedLangs[i] {
			selected = append(selected, lang)
		}
	}
	return selected
}

func (m *SetupModel) getSelectedDesktopApps() []string {
	var selected []string
	for i, app := range m.desktopApps {
		if m.selectedApps[i] {
			selected = append(selected, app)
		}
	}
	return selected
}

func (m *SetupModel) renderProgressBar() string {
	width := ProgressBarWidth
	filled := int(m.progress * float64(width))
	bar := ""

	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	return fmt.Sprintf("[%s] %.0f%%", bar, m.progress*100)
}

// InstallProgressMsg Installation process and progress tracking
type InstallProgressMsg struct {
	Status   string
	Progress float64
}

type InstallCompleteMsg struct{}

func (m *SetupModel) startInstallation() tea.Cmd {
	return func() tea.Msg {
		// Exit the current TUI and start the streaming installer TUI
		go func() {
			defer func() {
				// Ensure any panics in the goroutine are recovered
				if r := recover(); r != nil {
					log.Error("Panic in installation goroutine", fmt.Errorf("panic: %v", r))
					fmt.Printf("\n❌ Installation failed due to an unexpected error.\n")
					fmt.Printf("Please check the logs for details: %s\n", log.GetLogFile())
					fmt.Printf("Error: %v\n", r)
				}
			}()

			// Convert selections to CrossPlatformApp objects
			apps := m.buildAppList()

			log.Info("Starting streaming installer with selected apps", "appCount", len(apps))

			// Add debug logging for each app being installed
			for i, app := range apps {
				log.Info("App to install", "index", i, "name", app.Name, "description", app.Description)
			}

			fmt.Printf("\n🚀 Starting installation of %d applications...\n", len(apps))

			// Try simpler installation first to isolate the issue
			log.Info("Attempting fallback to direct installer to avoid TUI panic")

			// Use direct installer instead of TUI to avoid panic issues temporarily
			if err := installers.InstallCrossPlatformApps(apps, m.settings, m.repo); err != nil {
				log.Error("Direct installer failed", err)
				fmt.Printf("\n❌ Installation failed: %v\n", err)
				fmt.Printf("Check logs for details: %s\n", log.GetLogFile())
				return
			}

			// TODO: Re-enable streaming installer once panic issue is resolved
			// if err := tui.StartInstallation(apps, m.repo, m.settings); err != nil {
			//	log.Error("Streaming installer failed", err)
			//	fmt.Printf("\n❌ Installation failed: %v\n", err)
			//	fmt.Printf("Check logs for details: %s\n", log.GetLogFile())
			//	return
			// }

			// Installation completed successfully
			log.Info("Installation completed successfully")
			fmt.Printf("\n✅ Installation completed successfully!\n")
		}()

		return tea.Quit // Exit the guided setup TUI
	}
}

func (m *SetupModel) waitForActivity() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Millisecond * WaitActivityInterval)
		return InstallProgressMsg{Status: m.installStatus, Progress: m.progress}
	}
}

func (m *SetupModel) finalizeSetup(ctx context.Context) error {
	selectedShell := m.getSelectedShell()
	log.Info("Finalizing setup", "selectedShell", selectedShell)

	// Install the selected shell if not available
	if err := m.ensureShellInstalled(ctx, selectedShell); err != nil {
		log.Error("Failed to ensure shell is installed", err, "shell", selectedShell)
		return err
	}

	// Copy shell configuration files
	if err := m.copyShellConfiguration(selectedShell); err != nil {
		log.Error("Failed to copy shell configuration", err, "shell", selectedShell)
		return err
	}

	// Copy theme files and configurations
	if err := m.copyThemeFiles(); err != nil {
		log.Error("Failed to copy theme files", err)
		return err
	}

	// Copy application configuration files
	if err := m.copyAppConfigFiles(); err != nil {
		log.Error("Failed to copy application configuration files", err)
		return err
	}

	// Setup git configuration with user's name and email
	if err := m.setupGitConfiguration(ctx); err != nil {
		log.Error("Failed to setup git configuration", err)
		return err
	}

	// Switch to the selected shell
	if err := m.switchToShell(ctx, selectedShell); err != nil {
		log.Warn("Failed to switch shell", "error", err, "shell", selectedShell)
		shellPath, _ := exec.LookPath(selectedShell)
		log.Info("You can manually switch later with the DevEx shell command", "command", fmt.Sprintf("devex shell %s", selectedShell))
		log.Info("Or use the system command directly", "command", fmt.Sprintf("chsh -s %s", shellPath))
		log.Info("Note: The system shell change requires your password for security")
	}

	return nil
}

// Error tracking and validation methods

// buildAppList converts user selections into a structured installation plan
func (m *SetupModel) buildAppList() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp

	// Step 1: System update/upgrade - handled separately in installation process
	// This will be called before installing any apps

	// Step 2: Default terminal apps (all apps with default: true)
	defaultApps := m.getDefaultApps()
	apps = append(apps, defaultApps...)

	// Step 3: Setup languages via Mise
	if len(m.getSelectedLanguages()) > 0 {
		// First add mise itself
		if miseApp := m.getMiseApp(); miseApp != nil {
			apps = append(apps, *miseApp)
		}
		// Then add language-specific apps
		languageApps := m.getLanguageApps()
		apps = append(apps, languageApps...)
	}

	// Step 4: Setup databases via Docker
	if len(m.getSelectedDatabases()) > 0 {
		// First add Docker itself
		if dockerApp := m.getDockerApp(); dockerApp != nil {
			apps = append(apps, *dockerApp)
		}
		// Then add database containers
		databaseApps := m.getDatabaseApps()
		apps = append(apps, databaseApps...)
	}

	// Step 5: Desktop apps (if desktop detected and selected)
	if m.hasDesktop {
		desktopApps := m.getSelectedDesktopApps()
		for _, appName := range desktopApps {
			if app := m.getDesktopAppByName(appName); app != nil {
				apps = append(apps, *app)
			}
		}
	}

	// Step 6: Shell configuration (handled after app installation)
	// Step 7: Themes and shell files copying (handled after app installation)
	// Step 8: Git configuration (handled after app installation)

	return apps
}

// getDefaultApps returns all apps marked as default in configuration
func (m *SetupModel) getDefaultApps() []types.CrossPlatformApp {
	allApps := m.settings.GetAllApps()
	var defaultApps []types.CrossPlatformApp

	for _, app := range allApps {
		if app.Default {
			defaultApps = append(defaultApps, app)
		}
	}

	return defaultApps
}

// getShellApp returns the CrossPlatformApp for the selected shell
// getMiseApp returns a CrossPlatformApp for mise installation
func (m *SetupModel) getMiseApp() *types.CrossPlatformApp {
	return &types.CrossPlatformApp{
		Name:        "mise",
		Description: "Development environment manager for programming languages",
		Linux: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run",
			InstallCommand: "curl https://mise.run | sh",
		},
		MacOS: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run",
			InstallCommand: "curl https://mise.run | sh",
		},
		Windows: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run",
			InstallCommand: "curl https://mise.run | sh",
		},
	}
}

// getLanguageApps creates pseudo-apps for language installations via mise
func (m *SetupModel) getLanguageApps() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp
	selectedLangs := m.getSelectedLanguages()

	langMap := map[string]string{
		"Node.js":       "node@lts",
		"Python":        "python@latest",
		"Go":            "go@latest",
		"Ruby on Rails": "ruby@latest",
		"PHP":           "php@latest",
		"Java":          "java@latest",
		"Rust":          "rust@latest",
		"Elixir":        "elixir@latest",
	}

	for _, lang := range selectedLangs {
		if packageName, exists := langMap[lang]; exists {
			app := types.CrossPlatformApp{
				Name:        fmt.Sprintf("mise-%s", strings.ToLower(strings.ReplaceAll(lang, " ", "-"))),
				Description: fmt.Sprintf("Install %s via mise", lang),
				Linux: types.OSConfig{
					InstallMethod:  "mise",
					InstallCommand: packageName,
				},
				MacOS: types.OSConfig{
					InstallMethod:  "mise",
					InstallCommand: packageName,
				},
				Windows: types.OSConfig{
					InstallMethod:  "mise",
					InstallCommand: packageName,
				},
			}
			apps = append(apps, app)
		}
	}

	return apps
}

// getDesktopAppByName finds a desktop app by name from the configuration
func (m *SetupModel) getDesktopAppByName(name string) *types.CrossPlatformApp {
	allApps := m.settings.GetAllApps()
	for _, app := range allApps {
		if app.Name == name {
			return &app
		}
	}
	return nil
}

// createAutomatedSetupModel creates a SetupModel with default selections for automated setup
func createAutomatedSetupModel(repo types.Repository, settings config.CrossPlatformSettings) *SetupModel {
	return &SetupModel{
		selectedShell: DefaultShellIndex, // Default to zsh (first option)
		selectedLangs: map[int]bool{
			DefaultNodeJSIndex: true, // Node.js
			DefaultPythonIndex: true, // Python
		},
		selectedDBs: map[int]bool{
			DefaultPostgreSQLIndex: true, // PostgreSQL
		},
		selectedApps:  make(map[int]bool), // No desktop apps for automated setup
		installErrors: make([]string, 0),
		hasErrors:     false,
		shellSwitched: false,
		repo:          repo,
		settings:      settings,
		shells: []string{
			"zsh",
			"bash",
			"fish",
		},
		languages: []string{
			"Node.js",
			"Python",
			"Go",
			"Ruby on Rails",
			"PHP",
			"Java",
			"Rust",
			"Elixir",
		},
		databases: []string{
			"PostgreSQL",
			"MySQL",
			"Redis",
		},
	}
}

// printAutomatedSetupPlan displays the planned installation to the user
func printAutomatedSetupPlan() {
	fmt.Println("🚀 Starting automated DevEx setup...")
	fmt.Println("Selected for installation:")
	fmt.Println("  • zsh shell with DevEx configuration")
	fmt.Println("  • Essential development tools")
	fmt.Println("  • Programming languages: Node.js, Python")
	fmt.Println("  • Database: PostgreSQL")
	fmt.Println()
}

// printAutomatedSetupCompletion displays the completion message and next steps
func printAutomatedSetupCompletion(model *SetupModel) {
	selectedShell := model.getSelectedShell()

	fmt.Printf("\n🎉 Automated setup complete!\n")
	fmt.Printf("Your development environment has been set up with:\n")
	fmt.Printf("  • %s shell with DevEx configuration\n", selectedShell)
	fmt.Printf("  • Essential development tools\n")
	fmt.Printf("  • Programming languages: Node.js, Python\n")
	fmt.Printf("  • Database: PostgreSQL\n")

	if model.shellSwitched {
		fmt.Printf("\nYour shell has been switched to %s. Please restart your terminal\n", selectedShell)
		fmt.Printf("or run 'exec %s' to start using your new environment.\n", selectedShell)
	} else {
		fmt.Printf("\nYour environment is configured for %s.\n", selectedShell)
	}

	fmt.Printf("\nTo verify mise is working: 'mise list' or 'mise doctor'\n")
	fmt.Printf("To check Docker: 'docker ps' (if permission denied, run 'newgrp docker' or log out/in)\n")

	if logFile := log.GetLogFile(); logFile != "" {
		fmt.Printf("\n📋 Installation logs: %s\n", logFile)
		fmt.Printf("   (Submit this file for debugging if you encounter issues)\n")
	}
}

// isInteractiveMode checks if we should run in interactive mode (default: yes)
// Only goes non-interactive if explicitly requested or in CI environments
func isInteractiveMode() bool {
	// Check for explicit non-interactive request (like Homebrew)
	if os.Getenv("DEVEX_NONINTERACTIVE") == "1" || viper.GetBool("non-interactive") {
		return false
	}

	// Check for known CI environments
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" || os.Getenv("GITLAB_CI") != "" {
		return false
	}

	// Check for explicitly non-interactive terminals
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	// Default to interactive for better user experience
	return true
}

// runAutomatedSetup runs a non-interactive setup with sensible defaults
func runAutomatedSetup(repo types.Repository, settings config.CrossPlatformSettings) error {
	log.Info("Running automated setup with default selections")

	// Create a minimal setup model with default selections for automation
	model := createAutomatedSetupModel(repo, settings)

	// Convert default selections to CrossPlatformApp objects
	apps := model.buildAppList()

	log.Info("Automated setup will install apps:", "appCount", len(apps), "shell", "zsh", "languages", []string{"Node.js", "Python"}, "databases", []string{"PostgreSQL"})

	// Display the planned installation to the user
	printAutomatedSetupPlan()

	// Use the regular installer system for non-interactive mode
	if err := installers.InstallCrossPlatformApps(apps, settings, repo); err != nil {
		log.Error("Automated installation failed", err)
		fmt.Printf("⚠️  Installation failed: %v\n", err)
		return err
	}

	// Handle shell configuration and switching
	ctx := context.Background()
	if err := model.finalizeSetup(ctx); err != nil {
		log.Warn("Shell setup had issues", "error", err)
		fmt.Printf("⚠️  Shell setup issues: %v\n", err)
	}

	// Display completion message and next steps
	printAutomatedSetupCompletion(model)

	return nil
}

// getAvailableDesktopApps returns non-default desktop apps compatible with the detected desktop environment
func (m *SetupModel) getAvailableDesktopApps() []string {
	allApps := m.settings.GetAllApps()
	var desktopApps []string

	for _, app := range allApps {
		// Include apps that are:
		// 1. Not default (user should choose)
		// 2. Desktop/GUI applications
		// 3. Compatible with current platform
		if !app.Default && m.isDesktopApp(app) && m.isCompatibleWithPlatform(app) {
			desktopApps = append(desktopApps, app.Name)
		}
	}

	return desktopApps
}

// isDesktopApp determines if an app is a desktop/GUI application
func (m *SetupModel) isDesktopApp(app types.CrossPlatformApp) bool {
	desktopCategories := []string{
		"Text Editors", "IDEs", "Browsers", "Communication",
		"Media", "Graphics", "Productivity", "Utility",
	}

	for _, category := range desktopCategories {
		if app.Category == category {
			return true
		}
	}

	// Also check for known desktop apps by name
	desktopApps := []string{
		"Visual Studio Code", "IntelliJ IDEA", "Firefox", "Chrome",
		"Discord", "Slack", "VLC", "GIMP", "Typora", "Ulauncher",
	}

	for _, desktopApp := range desktopApps {
		if app.Name == desktopApp {
			return true
		}
	}

	return false
}

// isCompatibleWithPlatform checks if an app is available for the current platform
func (m *SetupModel) isCompatibleWithPlatform(app types.CrossPlatformApp) bool {
	switch m.detectedPlatform.OS {
	case "linux":
		return app.Linux.InstallCommand != ""
	case "darwin":
		return app.MacOS.InstallCommand != ""
	case "windows":
		return app.Windows.InstallCommand != ""
	default:
		return false
	}
}
