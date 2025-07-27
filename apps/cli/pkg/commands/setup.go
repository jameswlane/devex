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
	languages     []string
	databases     []string
	desktopApps   []string
	selectedLangs map[int]bool
	selectedDBs   map[int]bool
	selectedApps  map[int]bool
	cursor        int
	installing    bool
	installStatus string
	progress      float64
	repo          types.Repository
	settings      config.CrossPlatformSettings
}

// setupSteps defines the guided setup process
const (
	StepWelcome = iota
	StepLanguages
	StepDatabases
	StepDesktopApps
	StepConfirmation
	StepInstalling
	StepComplete
)

func runGuidedSetup(repo types.Repository, settings config.CrossPlatformSettings) {
	// Update settings with runtime flags
	settings.DryRun = viper.GetBool("dry_run")
	settings.Verbose = viper.GetBool("verbose")

	log.Info("Starting guided setup process", "dryRun", settings.DryRun, "verbose", settings.Verbose)

	// Initialize the setup model
	model := &SetupModel{
		step:          StepWelcome,
		selectedLangs: make(map[int]bool),
		selectedDBs:   make(map[int]bool),
		selectedApps:  make(map[int]bool),
		repo:          repo,
		settings:      settings,
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
		s += "  • Programming languages and tools\n"
		s += "  • Databases (via Docker)\n"
		s += "  • Essential development applications\n"
		s += "  • Desktop applications (if applicable)\n"
		s += "\n\n"
		s += "Press Enter to continue, or 'q' to quit."

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
		s = titleStyle.Render("🎉 Setup Complete!")
		s += "\n\n"
		s += "Your development environment has been successfully set up!\n\n"
		s += "What's been installed:\n"
		s += "  ✓ Essential development tools\n"
		s += "  ✓ Selected programming languages\n"
		s += "  ✓ Selected databases\n"
		s += "  ✓ Selected desktop applications\n"
		s += "\n\n"
		s += "Your shell has been switched to zsh. Please restart your terminal\n"
		s += "or run 'exec zsh' to start using your new environment.\n\n"
		s += "Press 'q' to exit."
	}

	return s
}

// Helper methods for handling user input and navigation
func (m *SetupModel) handleEnter() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepWelcome:
		return m.nextStep()
	case StepLanguages, StepDatabases, StepDesktopApps:
		return m.nextStep()
	case StepConfirmation:
		m.step = StepInstalling
		m.installing = true
		return m, m.startInstallation()
	}
	return m, nil
}

func (m *SetupModel) handleDown() (*SetupModel, tea.Cmd) {
	var maxItems int
	switch m.step {
	case StepLanguages:
		maxItems = len(m.languages)
	case StepDatabases:
		maxItems = len(m.databases)
	case StepDesktopApps:
		maxItems = len(m.desktopApps)
	}

	if m.cursor < maxItems-1 {
		m.cursor++
	}
	return m, nil
}

func (m *SetupModel) handleSpace() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepLanguages:
		m.selectedLangs[m.cursor] = !m.selectedLangs[m.cursor]
	case StepDatabases:
		m.selectedDBs[m.cursor] = !m.selectedDBs[m.cursor]
	case StepDesktopApps:
		m.selectedApps[m.cursor] = !m.selectedApps[m.cursor]
	}
	return m, nil
}

func (m *SetupModel) nextStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
	case StepWelcome:
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
	}
	return m, nil
}

func (m *SetupModel) prevStep() (*SetupModel, tea.Cmd) {
	m.cursor = 0
	switch m.step {
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

// Installation process and progress tracking
type InstallProgressMsg struct {
	Status   string
	Progress float64
}

type InstallCompleteMsg struct{}

func (m *SetupModel) startInstallation() tea.Cmd {
	return func() tea.Msg {
		// This would be where we call the actual installation logic
		// For now, we'll simulate the installation process
		go m.performInstallation()
		return InstallProgressMsg{Status: "Starting installation...", Progress: 0.0}
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

	// Step 1: Install essential tools (mise, docker, etc.)
	m.updateProgress("Installing essential tools...", 0.1)
	if err := m.installEssentialTools(ctx); err != nil {
		log.Error("Failed to install essential tools", err)
		return
	}

	// Step 1.5: Update environment and PATH
	m.updateProgress("Updating environment...", 0.2)
	if err := m.updateEnvironmentPath(ctx); err != nil {
		log.Error("Failed to update environment", err)
	}

	// Step 2: Install selected languages via mise (only if mise is available)
	if len(m.getSelectedLanguages()) > 0 {
		m.updateProgress("Installing programming languages...", 0.4)
		if err := m.installLanguages(ctx); err != nil {
			log.Error("Failed to install languages", err)
			// Continue with other installations
		}
	}

	// Step 3: Install selected databases via Docker (only if docker is available)
	if len(m.getSelectedDatabases()) > 0 {
		m.updateProgress("Installing databases...", 0.6)
		if err := m.installDatabases(ctx); err != nil {
			log.Error("Failed to install databases", err)
			// Continue with other installations
		}
	}

	// Step 4: Install desktop applications
	if len(m.getSelectedDesktopApps()) > 0 {
		m.updateProgress("Installing desktop applications...", 0.8)
		if err := m.installDesktopApps(ctx); err != nil {
			log.Error("Failed to install desktop applications", err)
			return
		}
	}

	// Step 5: Final setup and shell configuration
	m.updateProgress("Completing setup...", 0.9)
	if err := m.finalizeSetup(ctx); err != nil {
		log.Error("Failed to finalize setup", err)
		return
	}

	m.updateProgress("Installation complete!", 1.0)
}

func (m *SetupModel) updateProgress(status string, progress float64) {
	m.installStatus = status
	m.progress = progress
}

func (m *SetupModel) installEssentialTools(ctx context.Context) error {
	// Get default apps from configuration
	defaultApps := m.settings.GetDefaultApps()

	// Filter for essential tools
	var essentialApps []types.CrossPlatformApp
	for _, app := range defaultApps {
		// Include mise, docker, git, and other essential tools
		if app.Name == "Mise" || app.Name == "Docker" || app.Name == "git" ||
			app.Name == "curl" || app.Name == "wget" || app.Name == "zsh" {
			essentialApps = append(essentialApps, app)
		}
	}

	return installers.InstallCrossPlatformApps(essentialApps, m.settings, m.repo)
}

func (m *SetupModel) updateEnvironmentPath(ctx context.Context) error {
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

	os.Setenv("PATH", currentPath)
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

			// Stop and remove existing container if it exists
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
				log.Error("Failed to install database", err, "database", db, "output", string(output))
				continue
			}

			log.Info("Successfully installed database", "database", db, "container", config["container"])
		}
	}
	return nil
}

func (m *SetupModel) installDesktopApps(ctx context.Context) error {
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
	// Final setup steps like shell configuration
	log.Info("Finalizing setup - switching shell to zsh")

	// Check if zsh is available
	if !m.isToolAvailable("zsh") {
		log.Warn("zsh not available, skipping shell switch")
		log.Info("You can install zsh later with: sudo apt install zsh && chsh -s $(which zsh)")
		return nil
	}

	// Switch to zsh shell (using the existing shell switching logic)
	zshPath, err := exec.LookPath("zsh")
	if err != nil {
		log.Warn("zsh not found, skipping shell switch", "error", err)
		return nil
	}

	// Change user shell to zsh
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		log.Warn("Unable to determine current user, skipping shell switch")
		return nil
	}

	chshCmd := fmt.Sprintf("chsh -s %s %s", zshPath, currentUser)
	output, err := exec.CommandContext(ctx, "bash", "-c", chshCmd).CombinedOutput()
	if err != nil {
		log.Warn("Failed to change shell to zsh", "error", err, "output", string(output))
		log.Info("You can manually switch to zsh later with: chsh -s $(which zsh)")
	} else {
		log.Info("Successfully switched shell to zsh")
	}

	return nil
}
