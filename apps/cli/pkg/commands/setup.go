package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/gitconfig"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/tui"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
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
  devex setup --verbose`,
		Run: func(cmd *cobra.Command, args []string) {
			runGuidedSetup(repo, settings)
		},
	}

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

func runGuidedSetup(repo types.Repository, settings config.CrossPlatformSettings) {
	// Update settings with runtime flags
	settings.Verbose = viper.GetBool("verbose")

	log.Info("Starting guided setup process", "verbose", settings.Verbose, "logFile", log.GetLogFile())

	// Check if we're running in an interactive terminal
	if !isInteractiveTerminal() {
		log.Info("Non-interactive terminal detected, running automated setup")
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
		selectedShell:    0, // Default to zsh (first option)
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

		s += fmt.Sprintf("Full Name: %s\n", m.gitFullName)
		s += fmt.Sprintf("Email: %s\n", m.gitEmail)
		s += "\n\n"
		s += "Type your information and press Enter to continue"

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
	case StepDesktopApps, StepLanguages, StepDatabases, StepShell, StepGitConfig:
		return m.nextStep()
	case StepConfirmation:
		m.step = StepInstalling
		m.installing = true
		return m, m.startInstallation()
	default:
		panic("unhandled default case")
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
		m.step = StepConfirmation
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
func (m *SetupModel) getSelectedShell() string {
	if m.selectedShell >= 0 && m.selectedShell < len(m.shells) {
		return m.shells[m.selectedShell]
	}
	return m.shells[0] // Default to zsh
}

func (m *SetupModel) getSelectedLanguages() []string {
	var selected []string
	for i, lang := range m.languages {
		if m.selectedLangs[i] {
			selected = append(selected, lang)
		}
	}
	return selected
}

func (m *SetupModel) getSelectedDatabases() []string {
	var selected []string
	for i, db := range m.databases {
		if m.selectedDBs[i] {
			selected = append(selected, db)
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
	width := 50
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
			// Convert selections to CrossPlatformApp objects
			apps := m.buildAppList()

			log.Info("Starting streaming installer with selected apps", "appCount", len(apps))

			// Use the new streaming installer TUI for actual installation
			if err := tui.StartInstallation(apps, m.repo, m.settings); err != nil {
				log.Error("Streaming installer failed", err)
				m.addError("Streaming installer", err.Error())
			}

			// After the streaming installer completes, exit the program
			os.Exit(0)
		}()

		return tea.Quit // Exit the guided setup TUI
	}
}

func (m *SetupModel) waitForActivity() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Millisecond * 100)
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
	if err := m.setupGitConfiguration(); err != nil {
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

func (m *SetupModel) ensureShellInstalled(ctx context.Context, shell string) error {
	if isToolAvailable(shell) {
		log.Info("Shell already available", "shell", shell)
		return nil
	}

	log.Info("Installing shell", "shell", shell)

	// Get shell app from configuration
	allApps := m.settings.GetAllApps()
	for _, app := range allApps {
		if app.Name == shell {
			return installers.InstallCrossPlatformApp(app, m.settings, m.repo)
		}
	}

	log.Warn("Shell not found in configuration, installing via system package manager", "shell", shell)

	// Fallback to system package manager
	installCmd := fmt.Sprintf("sudo apt-get update && sudo apt-get install -y %s", shell)
	output, err := exec.CommandContext(ctx, "bash", "-c", installCmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to install shell via apt", err, "shell", shell, "output", string(output))
		return fmt.Errorf("failed to install %s: %w", shell, err)
	}

	return nil
}

func (m *SetupModel) copyShellConfiguration(shell string) error {
	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"

	switch shell {
	case "zsh":
		return m.copyZshConfiguration(homeDir, devexDir)
	case "bash":
		return m.copyBashConfiguration(homeDir, devexDir)
	case "fish":
		return m.copyFishConfiguration(homeDir, devexDir)
	default:
		log.Warn("No specific configuration available for shell", "shell", shell)
		return nil
	}
}

func (m *SetupModel) copyZshConfiguration(homeDir, devexDir string) error {
	log.Info("Copying zsh configuration files")

	// Copy main zshrc
	srcZshrc := devexDir + "/assets/zsh/zshrc"
	dstZshrc := homeDir + "/.zshrc"

	if err := m.copyFile(srcZshrc, dstZshrc); err != nil {
		return fmt.Errorf("failed to copy .zshrc: %w", err)
	}

	// Create a destination directory for zsh config modules
	zshConfigDir := devexDir + "/defaults/zsh"
	if err := os.MkdirAll(zshConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create zsh config directory: %w", err)
	}

	// Copy zsh configuration modules
	zshFiles := []string{"aliases", "extra", "init", "oh-my-zsh", "prompt", "rc", "shell", "zplug"}
	for _, file := range zshFiles {
		src := devexDir + "/assets/zsh/zsh/" + file
		dst := zshConfigDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy zsh config file", "file", file, "error", err)
		}
	}

	// Copy inputrc
	inputrcSrc := devexDir + "/assets/zsh/inputrc"
	inputrcDst := homeDir + "/.inputrc"
	if err := m.copyFile(inputrcSrc, inputrcDst); err != nil {
		log.Warn("Failed to copy .inputrc", "error", err)
	}

	return nil
}

func (m *SetupModel) copyBashConfiguration(homeDir, devexDir string) error {
	log.Info("Copying bash configuration files")

	// Copy main bashrc
	srcBashrc := devexDir + "/assets/bash/bashrc"
	dstBashrc := homeDir + "/.bashrc"

	if err := m.copyFile(srcBashrc, dstBashrc); err != nil {
		return fmt.Errorf("failed to copy .bashrc: %w", err)
	}

	// Create a destination directory for bash config modules
	bashConfigDir := devexDir + "/defaults/bash"
	if err := os.MkdirAll(bashConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create bash config directory: %w", err)
	}

	// Copy bash configuration modules
	bashFiles := []string{"aliases", "extra", "init", "oh-my-bash", "prompt", "rc", "shell"}
	for _, file := range bashFiles {
		src := devexDir + "/assets/bash/bash/" + file
		dst := bashConfigDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy bash config file", "file", file, "error", err)
		}
	}

	// Copy inputrc
	inputrcSrc := devexDir + "/assets/bash/inputrc"
	inputrcDst := homeDir + "/.inputrc"
	if err := m.copyFile(inputrcSrc, inputrcDst); err != nil {
		log.Warn("Failed to copy .inputrc", "error", err)
	}

	// Copy bash_profile if it exists
	bashProfileSrc := devexDir + "/assets/bash/bash_profile"
	bashProfileDst := homeDir + "/.bash_profile"
	if err := m.copyFile(bashProfileSrc, bashProfileDst); err != nil {
		log.Warn("Failed to copy .bash_profile", "error", err)
	}

	return nil
}

func (m *SetupModel) copyFishConfiguration(homeDir, devexDir string) error {
	log.Info("Copying fish configuration files")

	// Create fish config directory
	fishConfigDir := homeDir + "/.config/fish"
	if err := os.MkdirAll(fishConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create fish config directory: %w", err)
	}

	// Copy main config.fish
	srcConfig := devexDir + "/assets/fish/config.fish"
	dstConfig := fishConfigDir + "/config.fish"

	if err := m.copyFile(srcConfig, dstConfig); err != nil {
		return fmt.Errorf("failed to copy config.fish: %w", err)
	}

	// Create a destination directory for fish config modules
	fishDefaultsDir := devexDir + "/defaults/fish"
	if err := os.MkdirAll(fishDefaultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create fish defaults directory: %w", err)
	}

	// Copy fish configuration modules
	fishFiles := []string{"aliases", "shell", "init", "prompt"}
	for _, file := range fishFiles {
		src := devexDir + "/assets/fish/" + file
		dst := fishDefaultsDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy fish config file", "file", file, "error", err)
		}
	}

	// Copy fish modules from the fish subdirectory if they exist
	fishSubFiles := []string{"extra", "oh-my-fish"}
	for _, file := range fishSubFiles {
		src := devexDir + "/assets/fish/fish/" + file
		dst := fishDefaultsDir + "/" + file
		if err := m.copyFile(src, dst); err != nil {
			log.Warn("Failed to copy fish config file", "file", file, "error", err)
		}
	}

	return nil
}

func (m *SetupModel) switchToShell(ctx context.Context, shell string) error {
	shellPath, err := exec.LookPath(shell)
	if err != nil {
		return fmt.Errorf("%s not found: %w", shell, err)
	}

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		return fmt.Errorf("unable to determine current user")
	}

	// Get current shell for comparison
	currentShell, err := utils.GetUserShell(currentUser)
	if err != nil {
		log.Warn("Could not detect current shell", "error", err, "user", currentUser)
	} else {
		// Check if the current shell matches the desired shell (compare shell names, not full paths)
		currentShellName := filepath.Base(currentShell)
		selectedShellName := filepath.Base(shellPath)

		if currentShellName == selectedShellName {
			log.Info("User is already using the selected shell", "shell", shell, "currentPath", currentShell, "selectedPath", shellPath, "user", currentUser)
			m.shellSwitched = false // No switch occurred
			return nil
		}
		log.Info("Current shell differs from selected", "current", currentShell, "selected", shellPath, "user", currentUser)
	}

	log.Info("Switching to shell", "shell", shell, "path", shellPath, "user", currentUser)

	chshCmd := fmt.Sprintf("chsh -s %s %s", shellPath, currentUser)
	output, err := exec.CommandContext(ctx, "bash", "-c", chshCmd).CombinedOutput()
	if err != nil {
		m.shellSwitched = false // Switch failed
		return fmt.Errorf("failed to change shell: %w (output: %s)", err, string(output))
	}

	m.shellSwitched = true // Switch succeeded
	log.Info("Successfully switched shell", "shell", shell)
	return nil
}

func (m *SetupModel) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer func(sourceFile *os.File) {
		_ = sourceFile.Close()
	}(sourceFile)

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer func(destFile *os.File) {
		_ = destFile.Close()
	}(destFile)

	_, err = sourceFile.WriteTo(destFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

func isToolAvailable(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

// copyThemeFiles copies theme assets including backgrounds, neovim colorschemes, and application themes
func (m *SetupModel) copyThemeFiles() error {
	log.Info("Copying theme files and configurations")

	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"
	assetsDir := devexDir + "/assets"

	// Create necessary directories
	if err := os.MkdirAll(homeDir+"/.config", 0755); err != nil {
		return fmt.Errorf("failed to create .config directory: %w", err)
	}

	// Copy background images
	if err := m.copyThemeDirectory(assetsDir+"/themes/backgrounds", homeDir+"/.local/share/backgrounds"); err != nil {
		log.Warn("Failed to copy background images", "error", err)
	}

	// Copy Alacritty themes
	alacrittyConfigDir := homeDir + "/.config/alacritty"
	if err := os.MkdirAll(alacrittyConfigDir, 0755); err == nil {
		if err := m.copyThemeDirectory(assetsDir+"/themes/alacritty", alacrittyConfigDir+"/themes"); err != nil {
			log.Warn("Failed to copy Alacritty themes", "error", err)
		}
	}

	// Copy Neovim colorschemes
	neovimConfigDir := homeDir + "/.config/nvim"
	if err := os.MkdirAll(neovimConfigDir+"/colors", 0755); err == nil {
		if err := m.copyThemeDirectory(assetsDir+"/themes/neovim", neovimConfigDir+"/colors"); err != nil {
			log.Warn("Failed to copy Neovim colorschemes", "error", err)
		}
	}

	// Copy Zellij themes
	zellijConfigDir := homeDir + "/.config/zellij"
	if err := os.MkdirAll(zellijConfigDir+"/themes", 0755); err == nil {
		if err := m.copyThemeDirectory(assetsDir+"/themes/zellij", zellijConfigDir+"/themes"); err != nil {
			log.Warn("Failed to copy Zellij themes", "error", err)
		}
	}

	// Copy Oh My Posh themes
	ompConfigDir := homeDir + "/.config/oh-my-posh"
	if err := os.MkdirAll(ompConfigDir+"/themes", 0755); err == nil {
		if err := m.copyThemeDirectory(assetsDir+"/themes/oh-my-posh", ompConfigDir+"/themes"); err != nil {
			log.Warn("Failed to copy Oh My Posh themes", "error", err)
		}
	}

	// Copy Typora themes
	typoraThemeDir := homeDir + "/.config/Typora/themes"
	if err := os.MkdirAll(typoraThemeDir, 0755); err == nil {
		if err := m.copyThemeDirectory(assetsDir+"/themes/typora", typoraThemeDir); err != nil {
			log.Warn("Failed to copy Typora themes", "error", err)
		}
	}

	// Copy GNOME theme scripts (make them executable)
	gnomeScriptDir := devexDir + "/themes/gnome"
	if err := os.MkdirAll(gnomeScriptDir, 0755); err == nil {
		if err := m.copyThemeDirectory(assetsDir+"/themes/gnome", gnomeScriptDir); err != nil {
			log.Warn("Failed to copy GNOME theme scripts", "error", err)
		} else {
			// Make scripts executable
			m.makeScriptsExecutable(gnomeScriptDir)
		}
	}

	log.Info("Theme files copied successfully")
	return nil
}

// copyAppConfigFiles copies application configuration files and defaults
func (m *SetupModel) copyAppConfigFiles() error {
	log.Info("Copying application configuration files")

	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"
	assetsDir := devexDir + "/assets"

	// Create defaults directory in devex
	defaultsDir := devexDir + "/defaults"
	if err := os.MkdirAll(defaultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create defaults directory: %w", err)
	}

	// Copy default application configurations
	if err := m.copyThemeDirectory(assetsDir+"/defaults", defaultsDir); err != nil {
		log.Warn("Failed to copy default application configurations", "error", err)
	}

	// Copy XCompose file for special characters
	xcomposeFile := assetsDir + "/defaults/xcompose"
	if err := m.copyFile(xcomposeFile, homeDir+"/.XCompose"); err != nil {
		log.Warn("Failed to copy .XCompose file", "error", err)
	}

	log.Info("Application configuration files copied successfully")
	return nil
}

// setupGitConfiguration applies git configuration using user's name and email
func (m *SetupModel) setupGitConfiguration() error {
	log.Info("Setting up git configuration", "name", m.gitFullName, "email", m.gitEmail)

	// Set git user name
	if m.gitFullName != "" {
		if _, err := exec.Command("git", "config", "--global", "user.name", m.gitFullName).CombinedOutput(); err != nil {
			log.Warn("Failed to set git user name", "error", err)
		} else {
			log.Info("Git user name set successfully", "name", m.gitFullName)
		}
	}

	// Set git user email
	if m.gitEmail != "" {
		if _, err := exec.Command("git", "config", "--global", "user.email", m.gitEmail).CombinedOutput(); err != nil {
			log.Warn("Failed to set git user email", "error", err)
		} else {
			log.Info("Git user email set successfully", "email", m.gitEmail)
		}
	}

	// Apply additional git configuration from system.yaml
	homeDir := os.Getenv("HOME")
	devexDir := homeDir + "/.local/share/devex"
	systemConfigPath := devexDir + "/config/system.yaml"

	// Load and apply git config if the file exists
	if gitConfig, err := gitconfig.LoadGitConfig(systemConfigPath); err != nil {
		log.Warn("Failed to load git configuration from system.yaml", "error", err)
	} else {
		if err := gitconfig.ApplyGitConfig(gitConfig); err != nil {
			log.Warn("Failed to apply git configuration", "error", err)
		} else {
			log.Info("Additional git configuration applied successfully")
		}
	}

	return nil
}

// copyThemeDirectory copies all files from source directory to destination directory
func (m *SetupModel) copyThemeDirectory(srcDir, dstDir string) error {
	// Check if source directory exists
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("source directory does not exist: %s", srcDir)
	}

	// Create destination directory
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each file
	for _, entry := range entries {
		if !entry.IsDir() {
			srcPath := filepath.Join(srcDir, entry.Name())
			dstPath := filepath.Join(dstDir, entry.Name())

			if err := m.copyFile(srcPath, dstPath); err != nil {
				log.Warn("Failed to copy theme file", "src", srcPath, "dst", dstPath, "error", err)
			}
		}
	}

	return nil
}

// makeScriptsExecutable makes shell scripts executable
func (m *SetupModel) makeScriptsExecutable(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Warn("Failed to read directory for making scripts executable", "dir", dir, "error", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sh") {
			scriptPath := filepath.Join(dir, entry.Name())
			if err := os.Chmod(scriptPath, 0755); err != nil {
				log.Warn("Failed to make script executable", "script", scriptPath, "error", err)
			}
		}
	}
}

// Error tracking and validation methods
func (m *SetupModel) addError(component, message string) {
	errorMsg := fmt.Sprintf("%s: %s", component, message)
	m.installErrors = append(m.installErrors, errorMsg)
	m.hasErrors = true
	log.Error("Installation error", fmt.Errorf("%s", errorMsg), "component", component, "message", message)
}

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

// getDockerApp returns a CrossPlatformApp for Docker installation
func (m *SetupModel) getDockerApp() *types.CrossPlatformApp {
	return &types.CrossPlatformApp{
		Name:        "docker",
		Description: "Container platform for databases and services",
		Linux: types.OSConfig{
			InstallMethod:  "apt",
			InstallCommand: "docker.io",
		},
		MacOS: types.OSConfig{
			InstallMethod:  "brew",
			InstallCommand: "docker",
		},
		Windows: types.OSConfig{
			InstallMethod:  "winget",
			InstallCommand: "Docker.DockerDesktop",
		},
	}
}

// getDatabaseApps creates pseudo-apps for database installations via Docker
func (m *SetupModel) getDatabaseApps() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp
	selectedDBs := m.getSelectedDatabases()

	dbConfigs := map[string]map[string]string{
		"PostgreSQL": {
			"image":     "postgres:16",
			"container": "postgres16",
			"port":      "5432:5432",
			"env":       "POSTGRES_HOST_AUTH_METHOD=trust",
		},
		"MySQL": {
			"image":     "mysql:8.4",
			"container": "mysql8",
			"port":      "3306:3306",
			"env":       "MYSQL_ALLOW_EMPTY_PASSWORD=true",
		},
		"Redis": {
			"image":     "redis:7",
			"container": "redis",
			"port":      "6379:6379",
			"env":       "",
		},
	}

	for _, db := range selectedDBs {
		if dbConfig, exists := dbConfigs[db]; exists {
			dockerCmd := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -p 127.0.0.1:%s",
				dbConfig["container"], dbConfig["port"])

			if dbConfig["env"] != "" {
				dockerCmd += fmt.Sprintf(" -e %s", dbConfig["env"])
			}

			dockerCmd += fmt.Sprintf(" %s", dbConfig["image"])

			app := types.CrossPlatformApp{
				Name:        fmt.Sprintf("docker-%s", strings.ToLower(db)),
				Description: fmt.Sprintf("Install %s database via Docker", db),
				Linux: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
				MacOS: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
				Windows: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
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

// isInteractiveTerminal checks if we're running in an interactive terminal environment
func isInteractiveTerminal() bool {
	// Check if stdin is a terminal
	if fileInfo, err := os.Stdin.Stat(); err == nil {
		// If it's a character device (terminal) and not a pipe/redirect
		if (fileInfo.Mode() & os.ModeCharDevice) != 0 {
			return true
		}
	}

	// Additional checks for common non-interactive environments
	if os.Getenv("CI") != "" || os.Getenv("TERM") == "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	return false
}

// runAutomatedSetup runs a non-interactive setup with sensible defaults
func runAutomatedSetup(repo types.Repository, settings config.CrossPlatformSettings) error {
	log.Info("Running automated setup with default selections")

	// Create a minimal setup model with default selections for automation
	model := &SetupModel{
		selectedShell: 0, // Default to zsh (first option)
		selectedLangs: map[int]bool{
			0: true, // Node.js
			1: true, // Python
		},
		selectedDBs: map[int]bool{
			0: true, // PostgreSQL
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

	// Convert default selections to CrossPlatformApp objects
	apps := model.buildAppList()

	log.Info("Automated setup will install apps:", "appCount", len(apps), "shell", "zsh", "languages", []string{"Node.js", "Python"}, "databases", []string{"PostgreSQL"})

	// For automated setup, show the selections but skip the streaming TUI
	// run the installations directly using the existing installer system
	fmt.Println("🚀 Starting automated DevEx setup...")
	fmt.Println("Selected for installation:")
	fmt.Println("  • zsh shell with DevEx configuration")
	fmt.Println("  • Essential development tools")
	fmt.Println("  • Programming languages: Node.js, Python")
	fmt.Println("  • Database: PostgreSQL")
	fmt.Println()

	// Use the regular installer system for non-interactive mode
	if err := installers.InstallCrossPlatformApps(apps, settings, repo); err != nil {
		log.Error("Automated installation failed", err)
		fmt.Printf("⚠️  Installation failed: %v\n", err)
		return err
	}

	// Handle shell configuration and switching
	selectedShell := model.getSelectedShell()
	ctx := context.Background()
	if err := model.finalizeSetup(ctx); err != nil {
		log.Warn("Shell setup had issues", "error", err)
		fmt.Printf("⚠️  Shell setup issues: %v\n", err)
	}

	// Print completion message
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
