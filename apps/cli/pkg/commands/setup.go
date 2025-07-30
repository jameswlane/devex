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
	step          int
	shells        []string
	languages     []string
	databases     []string
	desktopApps   []string
	selectedShell int
	selectedLangs map[int]bool
	selectedDBs   map[int]bool
	selectedApps  map[int]bool
	cursor        int
	installing    bool
	installStatus string
	progress      float64
	installErrors []string
	hasErrors     bool
	shellSwitched bool
	repo          types.Repository
	settings      config.CrossPlatformSettings
}

// setupSteps defines the guided setup process
const (
	StepWelcome = iota
	StepShellSelection
	StepLanguages
	StepDatabases
	StepDesktopApps
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

	// Initialize the setup model
	model := &SetupModel{
		step:          StepWelcome,
		selectedShell: 0, // Default to zsh (first option)
		selectedLangs: make(map[int]bool),
		selectedDBs:   make(map[int]bool),
		selectedApps:  make(map[int]bool),
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

	// Detect desktop environment and set desktop apps accordingly
	plat := platform.DetectPlatform()
	if plat.DesktopEnv != "none" {
		model.desktopApps = []string{
			"Neovim",
			"Typora",
			"Ulauncher",
		}
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

	case StepShellSelection:
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

	case StepDesktopApps:
		if len(m.desktopApps) == 0 {
			newModel, _ := m.nextStep()
			return newModel.View()
		}

		s = titleStyle.Render("🖥️  Select Desktop Applications")
		s += "\n\n"
		s += subtitleStyle.Render("Choose desktop applications to install:")
		s += "\n\n"

		for i, app := range m.desktopApps {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			checked := " "
			if m.selectedApps[i] {
				checked = selectedStyle.Render("✓")
			}

			s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, app)
		}

		s += "\n\n"
		s += "Use ↑/↓ to navigate, Space to select/deselect, Enter to continue"

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
	case StepShellSelection, StepLanguages, StepDatabases, StepDesktopApps:
		return m.nextStep()
	case StepConfirmation:
		m.step = StepInstalling
		m.installing = true
		return m, m.startInstallation()
	default:
		panic("unhandled default case")
	}
	return m, nil
}

func (m *SetupModel) handleDown() (*SetupModel, tea.Cmd) {
	var maxItems int
	switch m.step {
	case StepShellSelection:
		maxItems = len(m.shells)
	case StepLanguages:
		maxItems = len(m.languages)
	case StepDatabases:
		maxItems = len(m.databases)
	case StepDesktopApps:
		maxItems = len(m.desktopApps)
	default:
		panic("unhandled default case")
	}

	if m.cursor < maxItems-1 {
		m.cursor++
	}
	return m, nil
}

func (m *SetupModel) handleSpace() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepShellSelection:
		m.selectedShell = m.cursor
	case StepLanguages:
		m.selectedLangs[m.cursor] = !m.selectedLangs[m.cursor]
	case StepDatabases:
		m.selectedDBs[m.cursor] = !m.selectedDBs[m.cursor]
	case StepDesktopApps:
		m.selectedApps[m.cursor] = !m.selectedApps[m.cursor]
	default:
		panic("unhandled default case")
	}
	return m, nil
}

func (m *SetupModel) nextStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
	case StepWelcome:
		m.step = StepShellSelection
	case StepShellSelection:
		m.step = StepLanguages
	case StepLanguages:
		m.step = StepDatabases
	case StepDatabases:
		if len(m.desktopApps) > 0 {
			m.step = StepDesktopApps
		} else {
			m.step = StepConfirmation
		}
	case StepDesktopApps:
		m.step = StepConfirmation
	default:
		panic("unhandled default case")
	}
	return m, nil
}

func (m *SetupModel) prevStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
	case StepLanguages:
		m.step = StepShellSelection
	case StepDatabases:
		m.step = StepLanguages
	case StepDesktopApps:
		m.step = StepDatabases
	case StepConfirmation:
		if len(m.desktopApps) > 0 {
			m.step = StepDesktopApps
		} else {
			m.step = StepDatabases
		}
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

func (m *SetupModel) performInstallation() {
	ctx := context.Background()

	// Step 1: Install mise (required for language management)
	m.updateProgress("Installing mise...", 0.05)
	if err := m.installMise(ctx); err != nil {
		m.addError("mise", err.Error())
	} else {
		// Validate mise installation
		if err := m.validateMiseInstallation(); err != nil {
			m.addError("mise validation", err.Error())
		}
	}

	// Step 2: Install Docker (required for database management)
	m.updateProgress("Installing Docker...", 0.1)
	if err := m.installDocker(ctx); err != nil {
		m.addError("Docker", err.Error())
	} else {
		// Validate Docker installation
		if err := m.validateDockerInstallation(); err != nil {
			m.addError("Docker validation", err.Error())
		}
	}

	// Step 3: Install other essential tools
	m.updateProgress("Installing essential tools...", 0.15)
	if err := m.installEssentialTools(); err != nil {
		m.addError("Essential tools", err.Error())
		// Essential tools failure is more critical, but continue
	}

	// Step 4: Update environment and PATH
	m.updateProgress("Updating environment...", 0.2)
	if err := m.updateEnvironmentPath(); err != nil {
		m.addError("Environment PATH", err.Error())
	}

	// Step 5: Install selected languages via mise (only if mise is available)
	if len(m.getSelectedLanguages()) > 0 {
		m.updateProgress("Installing programming languages...", 0.4)
		if err := m.installLanguages(ctx); err != nil {
			m.addError("Programming languages", err.Error())
		} else {
			// Validate language installations
			if err := m.validateInstalledLanguages(); err != nil {
				m.addError("Language validation", err.Error())
			}
		}
	}

	// Step 6: Install selected databases via Docker (only if docker is available)
	if len(m.getSelectedDatabases()) > 0 {
		m.updateProgress("Installing databases...", 0.6)
		if err := m.installDatabases(ctx); err != nil {
			m.addError("Databases", err.Error())
		}
	}

	// Step 7: Install desktop applications
	if len(m.getSelectedDesktopApps()) > 0 {
		m.updateProgress("Installing desktop applications...", 0.8)
		if err := m.installDesktopApps(); err != nil {
			m.addError("Desktop applications", err.Error())
		}
	}

	// Step 8: Final setup and shell configuration
	m.updateProgress("Completing setup...", 0.9)
	selectedShell := m.getSelectedShell()
	if err := m.finalizeSetup(ctx); err != nil {
		m.addError("Shell setup", err.Error())
	} else {
		// Validate shell configuration
		if err := m.validateShellConfiguration(selectedShell); err != nil {
			m.addError("Shell validation", err.Error())
		}
	}

	// Final status based on errors
	if m.hasErrors {
		m.updateProgress(fmt.Sprintf("Setup completed with %d issues", len(m.installErrors)), 1.0)
	} else {
		m.updateProgress("Installation complete!", 1.0)
	}
}

func (m *SetupModel) updateProgress(status string, progress float64) {
	m.installStatus = status
	m.progress = progress
}

func (m *SetupModel) installEssentialTools() error {
	// Get default apps from configuration
	defaultApps := m.settings.GetDefaultApps()

	// Filter for essential tools (excluding mise and Docker which are installed separately)
	var essentialApps []types.CrossPlatformApp
	for _, app := range defaultApps {
		// Include git, curl, wget, zsh and other essential tools but exclude mise and Docker
		if app.Name == "git" || app.Name == "curl" || app.Name == "wget" || app.Name == "zsh" ||
			app.Name == "bat" || app.Name == "Eza" || app.Name == "fzf" || app.Name == "ripgrep" {
			essentialApps = append(essentialApps, app)
		}
	}

	return installers.InstallCrossPlatformApps(essentialApps, m.settings, m.repo)
}

func (m *SetupModel) installMise(ctx context.Context) error {
	selectedShell := m.getSelectedShell()
	log.Info("Installing mise using official installer", "shell", selectedShell)

	// Use the shell-specific mise installer
	installCmd := fmt.Sprintf("curl https://mise.run/%s | sh", selectedShell)

	err := m.runCommandWithLogging(ctx, "bash", "-c", installCmd)
	if err != nil {
		log.Error("Failed to install mise", err, "shell", selectedShell)
		return fmt.Errorf("failed to install mise: %w", err)
	}

	log.Info("Successfully installed mise", "shell", selectedShell)

	// Update PATH to include mise
	homeDir := os.Getenv("HOME")
	miseDir := homeDir + "/.local/bin"
	currentPath := os.Getenv("PATH")
	if !contains(currentPath, miseDir) {
		err := os.Setenv("PATH", miseDir+":"+currentPath)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *SetupModel) installDocker(ctx context.Context) error {
	log.Info("Installing Docker using dedicated installer")

	// Use the dedicated Docker installer utility
	return utils.InstallDocker(ctx)
}

func (m *SetupModel) updateEnvironmentPath() error {
	// Update PATH to include common installation directories
	homeDir := os.Getenv("HOME")
	pathsToAdd := []string{
		homeDir + "/.local/bin",
		homeDir + "/.cargo/bin",
		"/usr/local/bin",
	}

	currentPath := os.Getenv("PATH")
	for _, path := range pathsToAdd {
		if !contains(currentPath, path) {
			currentPath = path + ":" + currentPath
		}
	}

	err := os.Setenv("PATH", currentPath)
	if err != nil {
		return err
	}
	log.Info("Updated PATH environment variable")
	return nil
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr ||
		strings.HasPrefix(s, substr+":") ||
		strings.Contains(s, ":"+substr+":") ||
		strings.HasSuffix(s, ":"+substr))
}

func (m *SetupModel) isToolAvailable(tool string) bool {
	_, err := exec.LookPath(tool)
	return err == nil
}

func (m *SetupModel) installLanguages(ctx context.Context) error {
	// Check if mise is available
	if !m.isToolAvailable("mise") {
		log.Warn("mise not available, skipping language installations")
		log.Info("Languages can be installed manually later using: mise install <language>@latest")
		return nil
	}

	// Install languages using mise
	selectedLangs := m.getSelectedLanguages()

	// Map UI names to mise package names
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
			log.Info("Installing language via mise", "language", lang, "package", packageName)

			// Install the language using mise
			installCmd := fmt.Sprintf("mise install %s", packageName)
			output, err := exec.CommandContext(ctx, "bash", "-c", installCmd).CombinedOutput()
			if err != nil {
				log.Error("Failed to install language", err, "language", lang, "output", string(output))
				continue
			}

			// Use the language globally
			useCmd := fmt.Sprintf("mise use -g %s", packageName)
			output, err = exec.CommandContext(ctx, "bash", "-c", useCmd).CombinedOutput()
			if err != nil {
				log.Error("Failed to set language globally", err, "language", lang, "output", string(output))
				continue
			}

			log.Info("Successfully installed language", "language", lang)
		}
	}
	return nil
}

func (m *SetupModel) installDatabases(ctx context.Context) error {
	// Check if Docker is available
	if !m.isToolAvailable("docker") {
		log.Warn("Docker not available, skipping database installations")
		log.Info("Databases can be installed manually later using: docker run ...")
		return nil
	}

	// Install databases using Docker
	selectedDBs := m.getSelectedDatabases()

	// Map UI names to Docker configurations
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
		if config, exists := dbConfigs[db]; exists {
			log.Info("Installing database via Docker", "database", db, "image", config["image"])

			// Stop and remove the existing container if it exists
			stopCmd := fmt.Sprintf("docker stop %s || true", config["container"])
			_ = exec.CommandContext(ctx, "bash", "-c", stopCmd).Run()

			removeCmd := fmt.Sprintf("docker rm %s || true", config["container"])
			_ = exec.CommandContext(ctx, "bash", "-c", removeCmd).Run()

			// Build docker run command
			dockerCmd := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -p 127.0.0.1:%s",
				config["container"], config["port"])

			if config["env"] != "" {
				dockerCmd += fmt.Sprintf(" -e %s", config["env"])
			}

			dockerCmd += fmt.Sprintf(" %s", config["image"])

			// Run the database container
			output, err := exec.CommandContext(ctx, "bash", "-c", dockerCmd).CombinedOutput()
			if err != nil {
				outputStr := string(output)
				if strings.Contains(outputStr, "permission denied") && strings.Contains(outputStr, "docker.sock") {
					m.addError("Docker permissions", fmt.Sprintf("%s: Docker permission denied. Run 'newgrp docker' or log out/in to refresh group membership", db))
				} else {
					m.addError("Database installation", fmt.Sprintf("%s: %s", db, err.Error()))
				}
				continue
			}

			log.Info("Successfully installed database", "database", db, "container", config["container"])
		}
	}
	return nil
}

func (m *SetupModel) installDesktopApps() error {
	// Install desktop applications
	selectedApps := m.getSelectedDesktopApps()

	// Get all available apps from configuration
	allApps := m.settings.GetAllApps()

	// Map UI names to app configurations
	appMap := make(map[string]types.CrossPlatformApp)
	for _, app := range allApps {
		appMap[app.Name] = app
	}

	for _, appName := range selectedApps {
		if app, exists := appMap[appName]; exists {
			log.Info("Installing desktop application", "app", appName)

			// Install using the existing installer system
			if err := installers.InstallCrossPlatformApp(app, m.settings, m.repo); err != nil {
				log.Error("Failed to install desktop application", err, "app", appName)
				continue
			}

			log.Info("Successfully installed desktop application", "app", appName)
		} else {
			log.Warn("Desktop application not found in configuration", "app", appName)
		}
	}
	return nil
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
	if m.isToolAvailable(shell) {
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
		err := sourceFile.Close()
		if err != nil {

		}
	}(sourceFile)

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer func(destFile *os.File) {
		err := destFile.Close()
		if err != nil {

		}
	}(destFile)

	_, err = sourceFile.WriteTo(destFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// Helper functions to run commands and log their output
func (m *SetupModel) runCommandWithLogging(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()

	// Log command execution using centralized logger
	commandStr := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	log.Info("Running command", "command", commandStr)

	if len(output) > 0 {
		log.Info("Command output", "command", commandStr, "output", string(output))
	}

	if err != nil {
		log.Error("Command failed", err, "command", commandStr, "output", string(output))
	}

	return err
}

// Error tracking and validation methods
func (m *SetupModel) addError(component, message string) {
	errorMsg := fmt.Sprintf("%s: %s", component, message)
	m.installErrors = append(m.installErrors, errorMsg)
	m.hasErrors = true
	log.Error("Installation error", fmt.Errorf("%s", errorMsg), "component", component, "message", message)
}

func (m *SetupModel) validateMiseInstallation() error {
	if !m.isToolAvailable("mise") {
		return fmt.Errorf("mise command not found")
	}

	// Test that mise can list installed tools
	ctx := context.Background()
	err := m.runCommandWithLogging(ctx, "mise", "list")
	if err != nil {
		return fmt.Errorf("mise list failed: %w", err)
	}

	log.Info("Mise validation successful")
	return nil
}

func (m *SetupModel) validateDockerInstallation() error {
	// Use the dedicated Docker validation utility
	return utils.ValidateDockerInstallation()
}

func (m *SetupModel) validateShellConfiguration(shell string) error {
	homeDir := os.Getenv("HOME")

	switch shell {
	case "zsh":
		if _, err := os.Stat(homeDir + "/.zshrc"); err != nil {
			return fmt.Errorf("zsh configuration not found: %w", err)
		}
	case "bash":
		if _, err := os.Stat(homeDir + "/.bashrc"); err != nil {
			return fmt.Errorf("bash configuration not found: %w", err)
		}
	case "fish":
		if _, err := os.Stat(homeDir + "/.config/fish/config.fish"); err != nil {
			return fmt.Errorf("fish configuration not found: %w", err)
		}
	}

	log.Info("Shell configuration validation successful", "shell", shell)
	return nil
}

func (m *SetupModel) validateInstalledLanguages() error {
	if !m.isToolAvailable("mise") {
		return fmt.Errorf("mise not available for language validation")
	}

	selectedLangs := m.getSelectedLanguages()
	if len(selectedLangs) == 0 {
		return nil // No languages selected, nothing to validate
	}

	// Check if mise can see the installed languages
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "mise", "list")
	output, err := cmd.CombinedOutput()

	// Log the command
	_ = m.runCommandWithLogging(ctx, "mise", "list")

	if err != nil {
		return fmt.Errorf("failed to list mise tools: %w", err)
	}

	miseOutput := string(output)
	var missingLangs []string

	langMap := map[string]string{
		"Node.js":       "node",
		"Python":        "python",
		"Go":            "go",
		"Ruby on Rails": "ruby",
		"PHP":           "php",
		"Java":          "java",
		"Rust":          "rust",
		"Elixir":        "elixir",
	}

	for _, lang := range selectedLangs {
		if toolName, exists := langMap[lang]; exists {
			if !strings.Contains(miseOutput, toolName) {
				missingLangs = append(missingLangs, lang)
			}
		}
	}

	if len(missingLangs) > 0 {
		return fmt.Errorf("missing languages: %v", missingLangs)
	}

	log.Info("Language validation successful", "installedLanguages", selectedLangs)
	return nil
}

// buildAppList converts user selections into CrossPlatformApp objects for the streaming installer
func (m *SetupModel) buildAppList() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp

	// Always include essential tools first
	essentialApps := m.getEssentialApps()
	apps = append(apps, essentialApps...)

	// Add a selected shell
	selectedShell := m.getSelectedShell()
	if shellApp := m.getShellApp(selectedShell); shellApp != nil {
		apps = append(apps, *shellApp)
	}

	// Add mise for language management if languages are selected
	if len(m.getSelectedLanguages()) > 0 {
		if miseApp := m.getMiseApp(); miseApp != nil {
			apps = append(apps, *miseApp)
		}

		// Add language-specific apps
		languageApps := m.getLanguageApps()
		apps = append(apps, languageApps...)
	}

	// Add Docker if databases are selected
	if len(m.getSelectedDatabases()) > 0 {
		if dockerApp := m.getDockerApp(); dockerApp != nil {
			apps = append(apps, *dockerApp)
		}

		// Add database apps
		databaseApps := m.getDatabaseApps()
		apps = append(apps, databaseApps...)
	}

	// Add selected desktop applications
	desktopApps := m.getSelectedDesktopApps()
	for _, appName := range desktopApps {
		if app := m.getDesktopAppByName(appName); app != nil {
			apps = append(apps, *app)
		}
	}

	return apps
}

// getEssentialApps returns essential development tools
func (m *SetupModel) getEssentialApps() []types.CrossPlatformApp {
	allApps := m.settings.GetAllApps()
	var essential []types.CrossPlatformApp

	essentialNames := []string{"git", "curl", "wget", "bat", "Eza", "fzf", "ripgrep"}
	for _, app := range allApps {
		for _, name := range essentialNames {
			if app.Name == name {
				essential = append(essential, app)
				break
			}
		}
	}

	return essential
}

// getShellApp returns the CrossPlatformApp for the selected shell
func (m *SetupModel) getShellApp(shell string) *types.CrossPlatformApp {
	allApps := m.settings.GetAllApps()
	for _, app := range allApps {
		if app.Name == shell {
			return &app
		}
	}
	return nil
}

// getMiseApp returns a CrossPlatformApp for mise installation
func (m *SetupModel) getMiseApp() *types.CrossPlatformApp {
	return &types.CrossPlatformApp{
		Name:        "mise",
		Description: "Development environment manager for programming languages",
		Linux: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run/zsh",
			InstallCommand: "curl https://mise.run/zsh | sh",
		},
		MacOS: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run/zsh",
			InstallCommand: "curl https://mise.run/zsh | sh",
		},
		Windows: types.OSConfig{
			InstallMethod:  "curlpipe",
			DownloadURL:    "https://mise.run/zsh",
			InstallCommand: "curl https://mise.run/zsh | sh",
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
					InstallMethod:  "curlpipe",
					InstallCommand: fmt.Sprintf("mise install %s && mise use -g %s", packageName, packageName),
				},
				MacOS: types.OSConfig{
					InstallMethod:  "curlpipe",
					InstallCommand: fmt.Sprintf("mise install %s && mise use -g %s", packageName, packageName),
				},
				Windows: types.OSConfig{
					InstallMethod:  "curlpipe",
					InstallCommand: fmt.Sprintf("mise install %s && mise use -g %s", packageName, packageName),
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
			InstallMethod:  "docker",
			InstallCommand: "echo 'Docker installation handled by dedicated installer'",
		},
		MacOS: types.OSConfig{
			InstallMethod:  "docker",
			InstallCommand: "echo 'Docker installation handled by dedicated installer'",
		},
		Windows: types.OSConfig{
			InstallMethod:  "docker",
			InstallCommand: "echo 'Docker installation handled by dedicated installer'",
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
		if config, exists := dbConfigs[db]; exists {
			dockerCmd := fmt.Sprintf("docker run -d --name %s --restart unless-stopped -p 127.0.0.1:%s",
				config["container"], config["port"])

			if config["env"] != "" {
				dockerCmd += fmt.Sprintf(" -e %s", config["env"])
			}

			dockerCmd += fmt.Sprintf(" %s", config["image"])

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
