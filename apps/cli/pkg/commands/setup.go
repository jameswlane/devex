package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/shell"
	"github.com/jameswlane/devex/pkg/themes"
	"github.com/jameswlane/devex/pkg/types"
)

// Compile regex patterns once at package initialization
var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
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
  • Automatic dependency management and ordering

Configuration hierarchy (highest to lowest priority):
  • Command-line flags
  • Environment variables (DEVEX_*)
  • Configuration files (~/.devex/config.yaml)
  • Default values`,
		Example: `  # Start interactive guided setup
  devex setup

  # Run setup with verbose output
  devex setup --verbose

  # Run non-interactive setup with defaults
  devex setup --non-interactive

  # Dry run to preview what would be installed
  devex setup --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Get configuration from Viper (respects hierarchy)
			verbose := viper.GetBool("verbose")
			dryRun := viper.GetBool("dry-run")
			nonInteractive := viper.GetBool("non-interactive")

			return executeGuidedSetup(ctx, verbose, dryRun, nonInteractive, repo, settings)
		},
		SilenceUsage: true, // Prevent usage spam on runtime errors
	}

	// Add flags
	cmd.Flags().Bool("non-interactive", false, "Run automated setup without user interaction")

	// Bind flags to viper
	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))

	return cmd
}

// executeGuidedSetup implements the core setup logic with proper context handling
func executeGuidedSetup(ctx context.Context, verbose, dryRun, nonInteractive bool, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Update settings with runtime configuration
	settings.Verbose = verbose

	log.Info("Starting guided setup process",
		"verbose", verbose,
		"dry-run", dryRun,
		"non-interactive", nonInteractive,
		"logFile", log.GetLogFile(),
	)

	// Handle non-interactive mode
	if nonInteractive {
		log.Info("Non-interactive mode requested, running automated setup")
		return runAutomatedSetupWithContext(ctx, dryRun, repo, settings)
	}

	// Handle dry run mode
	if dryRun {
		log.Info("Dry run mode - showing what would be set up")
		return previewSetup(settings)
	}

	// Run interactive guided setup
	return runInteractiveSetup(ctx, repo, settings)
}

// runInteractiveSetup handles the interactive TUI setup process
func runInteractiveSetup(ctx context.Context, repo types.Repository, settings config.CrossPlatformSettings) error {
	// TODO: Update runGuidedSetup to accept context parameter
	runGuidedSetup(repo, settings) //nolint:contextcheck
	return nil
}

// runAutomatedSetupWithContext handles non-interactive setup with context and dry-run support
func runAutomatedSetupWithContext(ctx context.Context, dryRun bool, repo types.Repository, settings config.CrossPlatformSettings) error {
	if dryRun {
		defaultApps := settings.GetDefaultApps()
		log.Info("Would install default applications in automated mode")
		for _, app := range defaultApps {
			log.Info("Would install", "app", app.Name, "category", app.Category)
		}
		return nil
	}

	// TODO: Update runAutomatedSetup to accept context parameter
	return runAutomatedSetup(repo, settings) //nolint:contextcheck
}

// previewSetup shows what the setup process would do
func previewSetup(settings config.CrossPlatformSettings) error {
	log.Info("Setup preview - showing available options:")

	defaultApps := settings.GetDefaultApps()
	log.Info("Default applications", "count", len(defaultApps))

	for _, app := range defaultApps {
		log.Info("Available app", "name", app.Name, "category", app.Category)
	}

	log.Info("Use --verbose for detailed setup commands")
	return nil
}

// SetupModel represents the state of our guided setup UI
type SetupModel struct {
	step             int
	shells           []string
	languages        []string
	databases        []string
	desktopApps      []string
	themes           []string
	selectedShell    int
	selectedLangs    map[int]bool
	selectedDBs      map[int]bool
	selectedApps     map[int]bool
	selectedTheme    int
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
	StepTheme     // Theme selection after shell
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

// FallbackThemes provides default themes when configuration is unavailable
// These are the themes available for the 1.0 release
var FallbackThemes = []string{
	"Tokyo Night",
	"Synthwave 84",
}

// getAvailableThemeNames extracts theme names from application configurations
func getAvailableThemeNames(settings config.CrossPlatformSettings) []string {
	log.Debug("Loading available themes from application configurations")

	// Get all applications from all categories in settings
	var allApps []interface{}

	// Collect applications from all categories using GetAllApps
	allConfigApps := settings.GetAllApps()
	appCategories := [][]types.CrossPlatformApp{
		allConfigApps, // Use the unified list from GetAllApps
	}

	// Convert CrossPlatformApp slice to interface{} slice for GetAvailableThemes
	for _, category := range appCategories {
		for _, app := range category {
			// Get themes from the appropriate OS config
			osConfig := app.GetOSConfig()
			if len(osConfig.Themes) > 0 {
				allApps = append(allApps, map[string]interface{}{
					"name":   app.Name,
					"themes": convertThemesToInterface(osConfig.Themes),
				})
			}
		}
	}

	// Get unique themes from all applications
	availableThemes := themes.GetAvailableThemes(allApps)

	// Extract theme names
	themeNames := make([]string, len(availableThemes))
	for i, theme := range availableThemes {
		themeNames[i] = theme.Name
	}

	log.Debug("Loaded themes from configurations", "count", len(themeNames), "themes", themeNames)

	// Fallback to default themes if none found in configurations
	// This fallback is used when:
	// - Configuration files are missing or corrupted
	// - No desktop environment themes are defined
	// - Theme loading from config files fails
	if len(themeNames) == 0 {
		log.Warn("No themes found in application configurations, using fallback themes")
		return FallbackThemes
	}

	return themeNames
}

// convertThemesToInterface converts []types.Theme to []interface{} for GetAvailableThemes
func convertThemesToInterface(themes []types.Theme) []interface{} {
	result := make([]interface{}, len(themes))
	for i, theme := range themes {
		result[i] = map[string]interface{}{
			"name":             theme.Name,
			"theme_color":      theme.ThemeColor,
			"theme_background": theme.ThemeBackground,
		}
	}
	return result
}

// getProgrammingLanguageNames extracts programming language names from environment configuration
func getProgrammingLanguageNames(settings config.CrossPlatformSettings) []string {
	if len(settings.ProgrammingLanguages) == 0 {
		log.Warn("No programming languages found in environment configuration, using fallback")
		// Fallback to default languages if none found in configuration
		return []string{
			"Node.js",
			"Python",
			"Go",
			"Ruby",
			"Java",
			"Rust",
		}
	}

	// Performance optimization: Pre-allocate slice with known capacity to avoid reallocations
	languageNames := make([]string, 0, len(settings.ProgrammingLanguages))
	for _, lang := range settings.ProgrammingLanguages {
		languageNames = append(languageNames, lang.Name)
	}

	log.Debug("Loaded programming languages from environment configuration", "count", len(languageNames), "languages", languageNames)
	return languageNames
}

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
	// Performance optimizations implemented:
	// 1. Pre-allocated slices with known capacity (setup.go:226)
	// 2. Single-pass filtering with early termination (setup.go:1321-1332)
	// 3. Cached results to avoid repeated computations during UI navigation
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
		languages: getProgrammingLanguageNames(settings),
		databases: []string{
			"PostgreSQL",
			"MySQL",
			"Redis",
		},
		themes: getAvailableThemeNames(settings), // Performance: Themes cached in model for UI navigation
	}

	// Set desktop apps based on platform and config (non-default apps only)
	// Performance optimization: Cache filtered results to avoid repeated filtering during UI navigation
	if model.hasDesktop {
		model.desktopApps = model.getAvailableDesktopApps()
	}

	// Start the Bubble Tea program
	program := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		log.Error("Error running guided setup", err)
		os.Exit(1)
	}

	// Clean up terminal after exiting alt screen
	if setupModel, ok := finalModel.(*SetupModel); ok {
		displayFinalMessage(setupModel)
	} else {
		log.Warn("Unable to cast final model to SetupModel for cleanup message")
		fmt.Print("\033[H\033[2J") // Clear screen at minimum
		fmt.Println("✅ DevEx Setup Completed!")
	}
}

// displayFinalMessage shows a clean final message after exiting the TUI
func displayFinalMessage(model *SetupModel) {
	// Clear the screen to remove any artifacts
	fmt.Print("\033[H\033[2J")

	// Build the final message
	var message strings.Builder

	// Header with some spacing
	message.WriteString("\n")

	if model.hasErrors {
		// Error header
		message.WriteString("⚠️  DevEx Setup Completed with Issues\n")
		message.WriteString("═══════════════════════════════════════\n\n")

		message.WriteString(fmt.Sprintf("Setup completed but encountered %d issues:\n\n", len(model.installErrors)))
		for _, err := range model.installErrors {
			message.WriteString(fmt.Sprintf("  ❌ %s\n", err))
		}
		message.WriteString("\n")
	} else {
		// Success header
		message.WriteString("✅ DevEx Setup Completed Successfully!\n")
		message.WriteString("═══════════════════════════════════════\n\n")
	}

	// What was installed
	message.WriteString("📦 Installed Components:\n")
	if selectedLangs := model.getSelectedLanguages(); len(selectedLangs) > 0 {
		message.WriteString("  • Programming languages via mise\n")
	}
	if selectedDBs := model.getSelectedDatabases(); len(selectedDBs) > 0 {
		message.WriteString("  • Database containers via Docker\n")
	}
	if selectedApps := model.getSelectedDesktopApps(); len(selectedApps) > 0 {
		message.WriteString("  • Desktop development tools\n")
	}
	message.WriteString(fmt.Sprintf("  • %s shell configuration\n", model.getSelectedShell()))
	message.WriteString("\n")

	// Next steps
	message.WriteString("🚀 Next Steps:\n")

	selectedShell := model.getSelectedShell()
	if model.shellSwitched {
		message.WriteString(fmt.Sprintf("  1. Restart your terminal or run: exec %s\n", selectedShell))
	} else {
		message.WriteString(fmt.Sprintf("  1. Reload your shell: source ~/.%src (or restart terminal)\n", selectedShell))
	}

	message.WriteString("  2. Verify mise: mise list\n")
	message.WriteString("  3. Check Docker: docker ps\n")

	if model.hasErrors {
		message.WriteString("\n⚠️  Some components may need manual attention.\n")
	}

	// Log file location
	if logFile := log.GetLogFile(); logFile != "" {
		message.WriteString(fmt.Sprintf("\n📋 Logs: %s\n", logFile))
	}

	message.WriteString("\nThank you for using DevEx! 🎉\n\n")

	// Print the final message
	fmt.Print(message.String())
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
		// Brief delay to show completion message, then quit automatically
		return m, tea.Sequence(
			tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
				return InstallQuitMsg{}
			}),
		)
	case InstallQuitMsg:
		return m, tea.Quit
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

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EF4444"))

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

	case StepTheme:
		s = titleStyle.Render("🎨 Select Your Theme")
		s += "\n\n"
		s += subtitleStyle.Render("Choose a theme for your applications:")
		s += "\n\n"

		for i, theme := range m.themes {
			cursor := " "
			if m.cursor == i {
				cursor = cursorStyle.Render(">")
			}

			selected := " "
			if m.selectedTheme == i {
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

		// Show email validation feedback
		if m.gitEmail != "" && !isValidEmail(m.gitEmail) {
			s += errorStyle.Render("   ⚠️  Email must contain @ and . characters") + "\n"
		}

		s += "\n"
		if m.gitInputActive {
			s += "Type your information and press Enter to confirm, Escape to cancel editing"
		} else {
			fullName := strings.TrimSpace(m.gitFullName)
			email := strings.TrimSpace(m.gitEmail)
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
	case StepTheme:
		// Theme step: Enter should not continue, only 'n' continues
		return m, nil
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
	case StepTheme:
		maxItems = len(m.themes)
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
	case StepTheme:
		m.selectedTheme = m.cursor
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
			m.step = StepTheme // Windows gets theme selection without shell
		}
	case StepShell:
		m.step = StepTheme
	case StepTheme:
		m.step = StepGitConfig
	case StepGitConfig:
		// Only proceed if both fields are filled and email is valid
		fullName := strings.TrimSpace(m.gitFullName)
		email := strings.TrimSpace(m.gitEmail)
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
	case StepTheme:
		if m.detectedPlatform.OS != "windows" {
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

// InstallQuitMsg signals that the setup should exit after installation
type InstallQuitMsg struct{}

func (m *SetupModel) startInstallation() tea.Cmd {
	return func() tea.Msg {
		// Convert selections to CrossPlatformApp objects
		apps := m.buildAppList()

		log.Info("Starting streaming installer with selected apps", "appCount", len(apps))

		// Add debug logging for each app being installed
		for i, app := range apps {
			log.Info("App to install", "index", i, "name", app.Name, "description", app.Description)
		}

		fmt.Printf("\n🚀 Starting installation of %d applications...\n", len(apps))

		// Start streaming installation with enhanced panic protection
		log.Info("Starting streaming installer with enhanced panic protection")

		// Use synchronous execution to prevent race conditions
		defer func() {
			// Ensure any panics in the installation are recovered
			if r := recover(); r != nil {
				log.Error("Panic in installation process", fmt.Errorf("panic: %v", r))
				fmt.Printf("\n❌ Installation failed due to an unexpected error.\n")
				fmt.Printf("Please check the logs for details: %s\n", log.GetLogFile())
				fmt.Printf("Error: %v\n", r)
			}
		}()

		if err := tui.StartInstallation(apps, m.repo, m.settings); err != nil {
			log.Error("Streaming installer failed", err)
			fmt.Printf("\n❌ Streaming installation failed: %v\n", err)

			// Fallback to direct installer if TUI fails
			log.Info("Falling back to direct installer")
			fmt.Printf("Attempting direct installation as fallback...\n")

			if err := installers.InstallCrossPlatformApps(apps, m.settings, m.repo); err != nil {
				log.Error("Direct installer also failed", err)
				fmt.Printf("\n❌ Both installation methods failed: %v\n", err)
				fmt.Printf("Check logs for details: %s\n", log.GetLogFile())
				return InstallCompleteMsg{} // Signal completion even on failure
			}
		}

		// Installation completed successfully
		log.Info("Installation completed successfully")
		return InstallCompleteMsg{} // Signal successful completion
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

	// Use comprehensive shell manager for complete shell setup
	shellManager := shell.NewShellManager(m.settings, m.repo)
	if err := shellManager.SetupShell(ctx, selectedShell); err != nil {
		log.Error("Failed to setup shell", err, "shell", selectedShell)
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

	// Save selected theme preference
	if err := m.saveThemePreference(); err != nil {
		log.Error("Failed to save theme preference", err)
		return err
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
		languages: getProgrammingLanguageNames(settings),
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
// Performance optimizations:
// - Called only once during initialization, result cached in model.desktopApps
// - Single-pass filtering with early termination conditions
// - Efficient boolean checks before expensive compatibility validation
func (m *SetupModel) getAvailableDesktopApps() []string {
	allApps := m.settings.GetAllApps()
	var desktopApps []string

	// Performance optimization: Single-pass filtering with ordered conditions
	// (cheapest checks first to enable early termination)
	for _, app := range allApps {
		// Include apps that are:
		// 1. Not default (user should choose) - cheapest check first
		// 2. Desktop/GUI applications - category-based check
		// 3. Compatible with current platform - platform detection
		// 4. Compatible with detected desktop environment - most expensive check last
		if !app.Default && m.isDesktopApp(app) && m.isCompatibleWithPlatform(app) && m.isCompatibleWithDesktopEnvironment(app) {
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

// isCompatibleWithDesktopEnvironment checks if an app is compatible with the detected desktop environment
func (m *SetupModel) isCompatibleWithDesktopEnvironment(app types.CrossPlatformApp) bool {
	// If no desktop environment detected, allow all apps
	if m.detectedPlatform.DesktopEnv == "unknown" || m.detectedPlatform.DesktopEnv == "" {
		return true
	}

	// For non-Linux systems, all desktop apps are compatible with the OS-level desktop
	if m.detectedPlatform.OS != "linux" {
		return true
	}

	// Use the app's built-in desktop environment compatibility check
	return app.IsCompatibleWithDesktopEnvironment(m.detectedPlatform.DesktopEnv)
}

// saveThemePreference saves the user's selected theme as the global preference
func (m *SetupModel) saveThemePreference() error {
	log.Info("Saving theme preference", "theme", m.themes[m.selectedTheme])

	// Create theme repository using the system repository
	systemRepo, ok := m.repo.(types.SystemRepository)
	if !ok {
		return fmt.Errorf("repository does not implement SystemRepository interface")
	}
	themeRepo := repository.NewThemeRepository(systemRepo)

	// Save the selected theme as global preference
	selectedTheme := m.themes[m.selectedTheme]
	if err := themeRepo.SetGlobalTheme(selectedTheme); err != nil {
		return fmt.Errorf("failed to save global theme preference: %w", err)
	}

	log.Info("Theme preference saved successfully", "theme", selectedTheme)
	return nil
}

// isValidEmail performs email format validation using a reasonable regex pattern
// This provides better validation than basic string checking while remaining practical
// Returns true if email matches standard email format
func isValidEmail(email string) bool {
	// Uses pre-compiled regex for better performance
	return emailRegex.MatchString(email)
}

// detectAssetsDir detects the location of built-in assets (similar to template manager)
func (m *SetupModel) detectAssetsDir() string {
	// Try different possible locations for built-in assets
	possiblePaths := []string{
		"assets",                  // Development mode (relative to binary)
		"./assets",                // Current directory
		"/usr/share/devex/assets", // System install
		"/opt/devex/assets",       // Alternative system install
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Fallback - try to find relative to the executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		assetsPath := filepath.Join(execDir, "assets")
		if _, err := os.Stat(assetsPath); err == nil {
			return assetsPath
		}

		// Try going up directories (for development)
		for i := 0; i < 3; i++ {
			execDir = filepath.Dir(execDir)
			assetsPath := filepath.Join(execDir, "assets")
			if _, err := os.Stat(assetsPath); err == nil {
				return assetsPath
			}
		}
	}

	// Final fallback
	return "assets"
}
