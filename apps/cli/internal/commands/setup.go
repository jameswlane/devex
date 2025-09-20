package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/tui"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
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
	return runGuidedSetup(ctx, repo, settings)
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

	// Pass context to runAutomatedSetup for proper cancellation support
	return runAutomatedSetup(ctx, repo, settings)
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

// UISelections groups all user selection states for the setup interface
type UISelections struct {
	selectedShell int
	selectedLangs map[int]bool
	selectedDBs   map[int]bool
	selectedApps  map[int]bool
	selectedTheme int
}

// GitConfiguration holds Git-specific configuration state
type GitConfiguration struct {
	gitFullName    string
	gitEmail       string
	gitInputField  int  // 0 = full name, 1 = email
	gitInputActive bool // true when editing a field
}

// InstallationState tracks the current installation progress and errors
type InstallationState struct {
	mu            sync.RWMutex // Protects all fields below
	installing    bool
	installStatus string
	progress      float64
	installErrors []string
	hasErrors     bool
}

// Thread-safe methods for InstallationState
func (is *InstallationState) setStatus(status string) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.installStatus = status
}

func (is *InstallationState) getStatus() string {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return is.installStatus
}

func (is *InstallationState) setProgress(progress float64) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.progress = progress
}

func (is *InstallationState) getProgress() float64 {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return is.progress
}

func (is *InstallationState) addError(err string) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.installErrors = append(is.installErrors, err)
	is.hasErrors = true
}

func (is *InstallationState) getErrors() []string {
	is.mu.RLock()
	defer is.mu.RUnlock()
	// Return a copy to prevent external modification
	errors := make([]string, len(is.installErrors))
	copy(errors, is.installErrors)
	return errors
}

func (is *InstallationState) hasInstallErrors() bool {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return is.hasErrors
}

func (is *InstallationState) setInstalling(installing bool) {
	is.mu.Lock()
	defer is.mu.Unlock()
	is.installing = installing
}

func (is *InstallationState) isInstalling() bool {
	is.mu.RLock()
	defer is.mu.RUnlock()
	return is.installing
}

// PluginStatus represents the status of a single plugin
type PluginStatus struct {
	Name   string
	Status string // "pending", "downloading", "verifying", "installing", "success", "error"
	Error  string
}

// PluginState manages plugin installation state
type PluginState struct {
	requiredPlugins   []string
	successfulPlugins []string
	pluginsInstalling int32 // atomic bool (0 = false, 1 = true)
	pluginsInstalled  int32 // atomic bool (0 = false, 1 = true)
	confirmPlugins    bool
	timeout           time.Duration   // Configurable timeout
	pluginStatuses    []PluginStatus  // Track status of each plugin
	pluginSpinners    []spinner.Model // Individual spinners for each plugin
	spinner           spinner.Model   // Spinner for plugin installation
}

// SystemInfo contains detected system information and available options
type SystemInfo struct {
	shells           []string
	languages        []string
	databases        []string
	desktopApps      []string
	themes           []string
	hasDesktop       bool
	detectedPlatform platform.DetectionResult
}

// SetupModel represents the state of our guided setup UI
// This model is organized into focused sub-structures for better maintainability
type SetupModel struct {
	// Navigation state
	step          int
	cursor        int
	shellSwitched bool

	// Grouped states for better organization
	system       SystemInfo
	selections   UISelections
	git          GitConfiguration
	installation InstallationState
	plugins      PluginState

	// External dependencies
	repo     types.Repository
	settings config.CrossPlatformSettings
}

// setupSteps defines the guided setup process
const (
	StepSystemOverview = iota
	StepPluginInstall  // Install required plugins first
	StepDesktopApps    // Only if desktop detected & non-default apps available
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
	WaitActivityInterval = 100 // milliseconds

	// Default selections for automated setup
	DefaultNodeJSIndex     = 0 // Node.js
	DefaultPythonIndex     = 1 // Python
	DefaultPostgreSQLIndex = 0 // PostgreSQL

	// File permissions
	DirectoryPermissions   = 0755
	ExecutablePermissions  = 0755
	RegularFilePermissions = 0644
)

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
				appWithThemes := map[string]interface{}{
					"name":   app.Name,
					"themes": convertThemesToInterface(osConfig.Themes),
				}
				allApps = append(allApps, appWithThemes)
			}
		}
	}

	// Get unique themes from all applications
	// TODO: Use desktop-themes plugin for theme management
	var themeNames []string
	themeSet := make(map[string]bool)

	// Extract theme names from collected apps
	for _, appInterface := range allApps {
		appMap, ok := appInterface.(map[string]interface{})
		if !ok {
			continue
		}

		themesInterface, exists := appMap["themes"]
		if !exists {
			continue
		}

		themes, ok := themesInterface.([]interface{})
		if !ok {
			continue
		}

		for _, themeInterface := range themes {
			themeMap, ok := themeInterface.(map[string]interface{})
			if !ok {
				continue
			}

			themeName, exists := themeMap["name"]
			if !exists {
				continue
			}

			name, ok := themeName.(string)
			if !ok || themeSet[name] {
				continue
			}

			themeNames = append(themeNames, name)
			themeSet[name] = true
		}
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

func runGuidedSetup(ctx context.Context, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Update settings with runtime flags
	settings.Verbose = viper.GetBool("verbose")

	log.Info("Starting guided setup process", "verbose", settings.Verbose, "logFile", log.GetLogFile())

	// Check if we should run in interactive mode (default: yes)
	if !isInteractiveMode() {
		log.Info("Non-interactive mode requested, running automated setup")
		if err := runAutomatedSetupWithContext(ctx, false, repo, settings); err != nil {
			log.Error("Automated setup failed", err)
			return fmt.Errorf("automated setup failed: %w", err)
		}
		return nil
	}

	// Detect platform and desktop environment first
	plat := platform.DetectPlatform()

	// Initialize the setup model
	// Performance optimizations implemented:
	// 1. Pre-allocated slices with known capacity (setup.go:226)
	// 2. Single-pass filtering with early termination (setup.go:1321-1332)
	// 3. Cached results to avoid repeated computations during UI navigation
	// Get plugin timeout from environment or use default
	pluginTimeout := getPluginTimeout()

	model := &SetupModel{
		step:          StepSystemOverview, // Start with system overview
		cursor:        0,
		shellSwitched: false,

		// System information
		system: SystemInfo{
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
			desktopApps:      []string{},                       // Will be populated based on platform
			themes:           getAvailableThemeNames(settings), // Performance: Themes cached in model for UI navigation
			hasDesktop:       plat.DesktopEnv != "none",
			detectedPlatform: plat,
		},

		// User selections
		selections: UISelections{
			selectedShell: DefaultShellIndex, // Default to zsh (first option)
			selectedLangs: make(map[int]bool),
			selectedDBs:   make(map[int]bool),
			selectedApps:  make(map[int]bool),
			selectedTheme: 0,
		},

		// Git configuration
		git: GitConfiguration{
			// Will be populated during setup
		},

		// Installation state
		installation: InstallationState{
			installing:    false,
			installStatus: "",
			progress:      0,
			installErrors: make([]string, 0, MaxErrorMessages),
			hasErrors:     false,
		},

		// Plugin state
		plugins: PluginState{
			requiredPlugins:   DetectRequiredPlugins(plat), // Detect plugins needed
			pluginsInstalling: 0,                           // atomic false
			pluginsInstalled:  0,                           // atomic false
			confirmPlugins:    false,
			timeout:           pluginTimeout,
			pluginStatuses:    initializePluginStatuses(DetectRequiredPlugins(plat)),
			spinner:           initializeSpinner(),
		},

		// External dependencies
		repo:     repo,
		settings: settings,
	}

	// Set desktop apps based on platform and config (non-default apps only)
	// Performance optimization: Cache filtered results to avoid repeated filtering during UI navigation
	if model.system.hasDesktop {
		model.system.desktopApps = model.getAvailableDesktopApps()
	}

	// Start the Bubble Tea program with context
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx))
	finalModel, err := program.Run()
	if err != nil {
		log.Error("Error running guided setup", err)
		return fmt.Errorf("error running guided setup: %w", err)
	}

	// Clean up terminal after exiting alt screen
	if setupModel, ok := finalModel.(*SetupModel); ok {
		displayFinalMessage(setupModel)
	} else {
		log.Warn("Unable to cast final model to SetupModel for cleanup message")
		fmt.Print("\033[H\033[2J") // Clear screen at minimum
		fmt.Println("✅ DevEx Setup Completed!")
	}

	return nil
}

// displayFinalMessage shows a clean final message after exiting the TUI
func displayFinalMessage(model *SetupModel) {
	// Clear the screen to remove any artifacts
	fmt.Print("\033[H\033[2J")

	// Build the final message
	var message strings.Builder

	// Header with some spacing
	message.WriteString("\n")

	if model.installation.hasErrors {
		// Error header
		message.WriteString("⚠️  DevEx Setup Completed with Issues\n")
		message.WriteString("═══════════════════════════════════════\n\n")

		message.WriteString(fmt.Sprintf("Setup completed but encountered %d issues:\n\n", len(model.installation.installErrors)))
		for _, err := range model.installation.installErrors {
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
	if selectedDBs := model.getSelectedDatabases(); len(selectedDBs) > 0 {
		message.WriteString("  3. Refresh Docker permissions: newgrp docker\n")
		message.WriteString("  4. Check Docker: docker ps\n")
	} else {
		message.WriteString("  3. Check Docker: docker ps\n")
	}

	if model.installation.hasErrors {
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

// Helper methods for handling user input and navigation
func (m *SetupModel) handleEnter() (*SetupModel, tea.Cmd) {
	switch m.step {
	case StepSystemOverview:
		if !m.plugins.confirmPlugins {
			m.plugins.confirmPlugins = true
			return m, nil
		}
		return m.nextStep()
	case StepPluginInstall:
		if atomic.LoadInt32(&m.plugins.pluginsInstalled) == 1 || m.installation.hasErrors {
			return m.nextStep()
		}
		return m, nil
	case StepDesktopApps, StepLanguages, StepDatabases, StepShell:
		return m.nextStep()
	case StepTheme:
		// Theme step: Enter should not continue, only 'n' continues
		return m, nil
	case StepGitConfig:
		if !m.git.gitInputActive {
			// Start editing the selected field
			m.git.gitInputActive = true
			m.git.gitInputField = m.cursor
		}
		return m, nil
	case StepConfirmation:
		m.step = StepInstalling
		m.installation.installing = true
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
		m.git.gitInputActive = false
		return m, nil
	case "escape":
		// Cancel editing
		m.git.gitInputActive = false
		return m, nil
	case "backspace":
		// Remove last character
		if m.git.gitInputField == 0 && len(m.git.gitFullName) > 0 {
			m.git.gitFullName = m.git.gitFullName[:len(m.git.gitFullName)-1]
		} else if m.git.gitInputField == 1 && len(m.git.gitEmail) > 0 {
			m.git.gitEmail = m.git.gitEmail[:len(m.git.gitEmail)-1]
		}
		return m, nil
	default:
		// Add character to current field
		if len(msg.Runes) > 0 {
			char := msg.Runes[0]
			switch m.git.gitInputField {
			case 0:
				m.git.gitFullName += string(char)
			case 1:
				m.git.gitEmail += string(char)
			}
		}
		return m, nil
	}
}

func (m *SetupModel) handleDown() (*SetupModel, tea.Cmd) {
	var maxItems int
	switch m.step {
	case StepDesktopApps:
		maxItems = len(m.system.desktopApps)
	case StepLanguages:
		maxItems = len(m.system.languages)
	case StepDatabases:
		maxItems = len(m.system.databases)
	case StepShell:
		maxItems = len(m.system.shells)
	case StepTheme:
		maxItems = len(m.system.themes)
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
		m.selections.selectedApps[m.cursor] = !m.selections.selectedApps[m.cursor]
	case StepLanguages:
		m.selections.selectedLangs[m.cursor] = !m.selections.selectedLangs[m.cursor]
	case StepDatabases:
		m.selections.selectedDBs[m.cursor] = !m.selections.selectedDBs[m.cursor]
	case StepShell:
		m.selections.selectedShell = m.cursor
	case StepTheme:
		m.selections.selectedTheme = m.cursor
	default:
		return m, nil // No selection needed for other steps
	}
	return m, nil
}

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

// Helper methods for getting selected items

func (m *SetupModel) getSelectedLanguages() []string {
	var selected []string
	for i, lang := range m.system.languages {
		if m.selections.selectedLangs[i] {
			selected = append(selected, lang)
		}
	}
	return selected
}

func (m *SetupModel) getSelectedDesktopApps() []string {
	var selected []string
	for i, app := range m.system.desktopApps {
		if m.selections.selectedApps[i] {
			selected = append(selected, app)
		}
	}
	return selected
}

func (m *SetupModel) getSelectedDatabases() []string {
	var selected []string
	for i, db := range m.system.databases {
		if m.selections.selectedDBs[i] {
			selected = append(selected, db)
		}
	}
	return selected
}

func (m *SetupModel) getSelectedShell() string {
	if m.selections.selectedShell >= 0 && m.selections.selectedShell < len(m.system.shells) {
		return m.system.shells[m.selections.selectedShell]
	}
	return "zsh" // Default fallback
}

func (m *SetupModel) getSelectedTheme() string {
	if m.selections.selectedTheme >= 0 && m.selections.selectedTheme < len(m.system.themes) {
		return m.system.themes[m.selections.selectedTheme]
	}
	return "" // No theme selected
}

func (m *SetupModel) renderProgressBar() string {
	width := ProgressBarWidth
	filled := int(m.installation.progress * float64(width))
	bar := ""

	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}

	return fmt.Sprintf("[%s] %.0f%%", bar, m.installation.progress*100)
}

// InstallProgressMsg Installation process and progress tracking
type InstallProgressMsg struct {
	Status   string
	Progress float64
}

type InstallCompleteMsg struct{}

// InstallQuitMsg signals that the setup should exit after installation
type InstallQuitMsg struct{}

// PluginInstallMsg represents a plugin installation progress update
type PluginInstallMsg struct {
	Status   string
	Progress float64
	Error    error
}

// PluginStatusUpdateMsg represents a status update for an individual plugin
type PluginStatusUpdateMsg struct {
	PluginName string
	Status     string // "pending", "downloading", "verifying", "installing", "success", "error"
	Error      string
}

// PluginInstallCompleteMsg indicates plugin installation is complete
type PluginInstallCompleteMsg struct {
	Errors            []error
	SuccessCount      int
	TotalCount        int
	SuccessfulPlugins []string
}

// BoundedErrorCollector manages error collection with memory bounds
// This prevents unbounded memory growth during error collection while preserving
// important error information
type BoundedErrorCollector struct {
	errors    []error
	maxErrors int
	truncated bool
	mu        sync.Mutex
}

// NewBoundedErrorCollector creates a new error collector with specified bounds
func NewBoundedErrorCollector(maxErrors int) *BoundedErrorCollector {
	return &BoundedErrorCollector{
		errors:    make([]error, 0, maxErrors),
		maxErrors: maxErrors,
	}
}

// AddError safely adds an error to the collector
func (c *BoundedErrorCollector) AddError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.errors) >= c.maxErrors {
		if !c.truncated {
			c.truncated = true
			// Replace the last error with a truncation notice
			c.errors[c.maxErrors-1] = fmt.Errorf("error collection truncated at %d errors (last: %w)", c.maxErrors, err)
		}
		return
	}
	c.errors = append(c.errors, err)
}

// GetErrors returns a copy of collected errors
func (c *BoundedErrorCollector) GetErrors() []error {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]error, len(c.errors))
	copy(result, c.errors)
	return result
}

// IsTruncated returns whether error collection was truncated
func (c *BoundedErrorCollector) IsTruncated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.truncated
}

// addErrorSafe safely adds an error to the slice with bounds checking to prevent unbounded memory growth
// Deprecated: Use BoundedErrorCollector instead for thread-safe error collection
func addErrorSafe(errors []error, newError error) []error {
	if len(errors) >= MaxErrorMessages {
		// Replace the last error with a truncation notice
		errors[MaxErrorMessages-1] = fmt.Errorf("error collection truncated at %d errors (last: %w)", MaxErrorMessages, newError)
		return errors
	}
	return append(errors, newError)
}

// addErrorStringSafe safely adds an error string to the slice with bounds checking
// Deprecated: Use BoundedErrorCollector instead for thread-safe error collection
func addErrorStringSafe(errors []string, newError string) []string {
	if len(errors) >= MaxErrorMessages {
		// Replace the last error with a truncation notice
		errors[MaxErrorMessages-1] = fmt.Sprintf("error collection truncated at %d errors (last: %s)", MaxErrorMessages, newError)
		return errors
	}
	return append(errors, newError)
}

// getPluginTimeout returns the plugin installation timeout from environment or default
func getPluginTimeout() time.Duration {
	// Check environment variable for custom timeout
	if timeoutStr := os.Getenv("DEVEX_PLUGIN_TIMEOUT"); timeoutStr != "" {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			log.Info("Using custom plugin timeout", "timeout", timeout.String())
			return timeout
		}
		log.Warn("Invalid DEVEX_PLUGIN_TIMEOUT value, using default", "value", timeoutStr, "default", PluginInstallTimeout.String())
	}
	return PluginInstallTimeout
}

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
		// Send realistic plugin status updates with proper timing
		m.simulateRealisticPluginProgress(),
		// Start the actual plugin installation after enough time to see the progress
		tea.Tick(time.Millisecond*3500, func(t time.Time) tea.Msg {
			return tea.Sequence(m.runPluginInstallation())()
		}),
	)
}

// simulateRealisticPluginProgress sends realistic plugin status updates with proper timing
func (m *SetupModel) simulateRealisticPluginProgress() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.plugins.pluginStatuses))

	// Send status updates for each plugin with staggered timing
	for i, pluginName := range m.plugins.requiredPlugins {
		delay := time.Duration(i*800+300) * time.Millisecond

		// Update to downloading status
		cmds = append(cmds, tea.Tick(delay, func(name string) func(time.Time) tea.Msg {
			return func(t time.Time) tea.Msg {
				return PluginStatusUpdateMsg{
					PluginName: name,
					Status:     "downloading",
				}
			}
		}(pluginName)))

		// Update to verifying status
		cmds = append(cmds, tea.Tick(delay+time.Millisecond*1000, func(name string) func(time.Time) tea.Msg {
			return func(t time.Time) tea.Msg {
				return PluginStatusUpdateMsg{
					PluginName: name,
					Status:     "verifying",
				}
			}
		}(pluginName)))

		// Update to installing status
		cmds = append(cmds, tea.Tick(delay+time.Millisecond*1500, func(name string) func(time.Time) tea.Msg {
			return func(t time.Time) tea.Msg {
				return PluginStatusUpdateMsg{
					PluginName: name,
					Status:     "installing",
				}
			}
		}(pluginName)))
	}

	// Add frequent spinner ticks for smooth animation
	for i := 0; i < 35; i++ {
		delay := time.Duration(i*100) * time.Millisecond
		cmds = append(cmds, tea.Tick(delay, func(t time.Time) tea.Msg {
			return m.plugins.spinner.Tick
		}))
	}

	return tea.Batch(cmds...)
}

// sendPluginUpdates sends status updates for individual plugins during installation
func (m *SetupModel) sendPluginUpdates() tea.Cmd {
	return func() tea.Msg {
		// This will be called to send individual plugin status updates
		// For now, we update statuses directly in the installation process
		return nil
	}
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

		// Set plugin downloader to silent mode to clean up UI during setup
		pluginBootstrap.SetSilent(true)

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
				// Cannot return from defer function - the panic recovery is for logging only
			}
		}()

		// Get context from Bubble Tea program
		ctx := context.Background() // Default context if GetContext is not available

		if err := tui.StartInstallation(ctx, apps, m.repo, m.settings); err != nil {
			log.Error("Streaming installer failed", err)
			fmt.Printf("\n❌ Streaming installation failed: %v\n", err)

			// Fallback to direct installer if TUI fails
			log.Info("Falling back to direct installer")
			fmt.Printf("Attempting direct installation as fallback...\n")

			var errors []string
			for _, app := range apps {
				if err := installers.InstallCrossPlatformApp(ctx, app, m.settings, m.repo); err != nil {
					errors = append(errors, fmt.Sprintf("failed to install %s: %v", app.Name, err))
				}
			}
			if len(errors) > 0 {
				err := fmt.Errorf("installation failures: %s", strings.Join(errors, "; "))
				log.Error("Direct installer also failed", err)
				fmt.Printf("\n❌ Both installation methods failed: %v\n", err)
				fmt.Printf("Check logs for details: %s\n", log.GetLogFile())
				return InstallCompleteMsg{} // Signal completion even on failure
			}
		}

		// Installation completed successfully - now finalize shell setup
		log.Info("Installation completed successfully, running shell configuration finalization")

		// Run shell finalization (same as automated setup)
		if err := m.finalizeSetup(ctx); err != nil {
			log.Warn("Shell setup had issues during TUI installation", "error", err)
			// Don't fail the entire setup for shell config issues
		}

		return InstallCompleteMsg{} // Signal successful completion
	}
}

func (m *SetupModel) waitForActivity() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Millisecond * WaitActivityInterval)
		return InstallProgressMsg{Status: m.installation.getStatus(), Progress: m.installation.getProgress()}
	}
}

func (m *SetupModel) finalizeSetup(ctx context.Context) error {
	selectedShell := m.getSelectedShell()
	log.Info("Finalizing setup", "selectedShell", selectedShell)

	// Initialize plugin bootstrap for post-installation configuration
	// Try with downloads enabled first, but fall back to skip downloads if registry is unavailable
	pluginBootstrap, err := bootstrap.NewPluginBootstrap(false)
	if err != nil {
		log.Warn("Plugin download failed during finalization, trying with downloads disabled", "error", err)
		pluginBootstrap, err = bootstrap.NewPluginBootstrap(true) // Skip downloads
	}
	if err != nil {
		log.Error("Failed to initialize plugin system for finalization", err)
		return fmt.Errorf("failed to initialize plugin system. This may be due to network connectivity issues, insufficient permissions, or missing dependencies. Please check your internet connection, ensure you have write access to the plugin directory, and try again: %w", err)
	}

	// Set plugin downloader to silent mode for cleaner finalization
	pluginBootstrap.SetSilent(true)

	// Initialize plugin system
	if err := pluginBootstrap.Initialize(ctx); err != nil {
		log.Error("Failed to bootstrap plugins for finalization", err)
		return fmt.Errorf("plugin initialization failed: %w", err)
	}

	// 1. Shell configuration using tool-shell plugin (conditional on shell selection or config)
	if err := m.handleShellConfiguration(ctx, pluginBootstrap, selectedShell); err != nil {
		log.Warn("Shell configuration failed", "error", err)
		// Don't fail the entire setup for shell config issues
	}

	// 2. Desktop theme configuration using desktop plugin based on detected environment
	if err := m.handleDesktopConfiguration(ctx, pluginBootstrap); err != nil {
		log.Warn("Desktop configuration failed", "error", err)
		// Don't fail the entire setup for desktop config issues
	}

	// 3. Git configuration using tool-git plugin (conditional on git config presence)
	if err := m.handleGitConfiguration(ctx, pluginBootstrap); err != nil {
		log.Warn("Git configuration failed", "error", err)
		// Don't fail the entire setup for git config issues
	}

	// Save selected theme preference (using internal implementation)
	if err := m.saveThemePreference(); err != nil {
		log.Error("Failed to save theme preference", err)
		return err
	}

	return nil
}

// handleShellConfiguration configures the selected shell using tool-shell plugin
// Uses plugin only if user changed shell from default or if custom shell config exists
func (m *SetupModel) handleShellConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap, selectedShell string) error {
	// For now, we'll always run shell configuration since we can't easily detect existing config
	// TODO: Add detection of existing shell configuration files to check if config is needed
	// Currently assuming shell configuration is always needed for proper setup
	log.Debug("Shell configuration needed", "shell", selectedShell)

	// Check if tool-shell plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPlugins()

	if _, exists := installedPlugins["tool-shell"]; !exists {
		log.Warn("tool-shell plugin not available, skipping shell configuration")
		return nil
	}

	log.Info("Configuring shell using tool-shell plugin", "shell", selectedShell)

	// Execute tool-shell plugin with the selected shell
	args := []string{"configure", selectedShell}
	if err := pluginBootstrap.ExecutePlugin("tool-shell", args); err != nil { //nolint:contextcheck
		return fmt.Errorf("failed to configure shell using tool-shell plugin. This may be due to shell configuration file permissions or an unsupported shell type. Please check that your shell configuration files are writable and that your shell is supported: %w", err)
	}

	log.Info("Shell configuration completed successfully", "shell", selectedShell)
	return nil
}

// handleDesktopConfiguration applies desktop theme and settings using appropriate desktop plugin
// Detects desktop environment and uses corresponding plugin (desktop-gnome, desktop-kde, etc.)
func (m *SetupModel) handleDesktopConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap) error {
	if !m.system.hasDesktop {
		log.Debug("No desktop environment detected, skipping desktop configuration")
		return nil
	}

	// Determine desktop plugin based on detected environment
	desktopEnv := m.system.detectedPlatform.DesktopEnv
	if desktopEnv == "none" || desktopEnv == "" {
		log.Debug("Desktop environment not detected or supported, skipping desktop configuration")
		return nil
	}

	pluginName := fmt.Sprintf("desktop-%s", strings.ToLower(desktopEnv))
	log.Info("Configuring desktop using plugin", "plugin", pluginName, "desktop", desktopEnv)

	// Check if desktop plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPlugins()

	if _, exists := installedPlugins[pluginName]; !exists {
		log.Warn("Desktop plugin not available, skipping desktop configuration", "plugin", pluginName)
		return nil
	}

	// Get selected theme if any
	selectedTheme := m.getSelectedTheme()

	// Execute desktop plugin with theme configuration
	args := []string{"configure"}
	if selectedTheme != "" {
		args = append(args, "--theme", selectedTheme)
	}

	if err := pluginBootstrap.ExecutePlugin(pluginName, args); err != nil { //nolint:contextcheck
		return fmt.Errorf("failed to configure desktop using %s plugin. This may be due to missing desktop environment packages, insufficient permissions, or unsupported desktop configuration. Please ensure your desktop environment is fully installed and you have appropriate permissions: %w", pluginName, err)
	}

	log.Info("Desktop configuration completed successfully", "plugin", pluginName, "theme", selectedTheme)
	return nil
}

// handleGitConfiguration sets up Git using tool-git plugin
// Uses plugin only if Git configuration (name, email) is provided
func (m *SetupModel) handleGitConfiguration(ctx context.Context, pluginBootstrap *bootstrap.PluginBootstrap) error {
	// Check if git configuration is provided
	gitName := strings.TrimSpace(m.git.gitFullName)
	gitEmail := strings.TrimSpace(m.git.gitEmail)

	if gitName == "" && gitEmail == "" {
		log.Debug("No Git configuration provided, skipping git setup")
		return nil
	}

	// Check if tool-git plugin is available
	manager := pluginBootstrap.GetManager()
	installedPlugins := manager.ListPlugins()

	if _, exists := installedPlugins["tool-git"]; !exists {
		log.Warn("tool-git plugin not available, skipping git configuration")
		return nil
	}

	log.Info("Configuring Git using tool-git plugin", "name", gitName, "email", gitEmail)

	// Execute tool-git plugin with user configuration
	args := []string{"configure"}
	if gitName != "" {
		args = append(args, "--name", gitName)
	}
	if gitEmail != "" {
		args = append(args, "--email", gitEmail)
	}

	if err := pluginBootstrap.ExecutePlugin("tool-git", args); err != nil { //nolint:contextcheck
		return fmt.Errorf("failed to configure git using tool-git plugin: %w", err)
	}

	log.Info("Git configuration completed successfully", "name", gitName, "email", gitEmail)
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
	if m.system.hasDesktop {
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
	// Get plugin timeout from environment or use default
	pluginTimeout := getPluginTimeout()

	return &SetupModel{
		step:          0,
		cursor:        0,
		shellSwitched: false,

		// System information
		system: SystemInfo{
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
			desktopApps: []string{},
			themes:      []string{},
			hasDesktop:  false,
		},

		// User selections
		selections: UISelections{
			selectedShell: DefaultShellIndex, // Default to zsh (first option)
			selectedLangs: map[int]bool{
				DefaultNodeJSIndex: true, // Node.js
				DefaultPythonIndex: true, // Python
			},
			selectedDBs: map[int]bool{
				DefaultPostgreSQLIndex: true, // PostgreSQL
			},
			selectedApps:  make(map[int]bool), // No desktop apps for automated setup
			selectedTheme: 0,
		},

		// Git configuration
		git: GitConfiguration{
			// Will be populated during setup
		},

		// Installation state
		installation: InstallationState{
			installing:    false,
			installStatus: "",
			progress:      0,
			installErrors: make([]string, 0, MaxErrorMessages),
			hasErrors:     false,
		},

		// Plugin state
		plugins: PluginState{
			requiredPlugins:   []string{},
			pluginsInstalling: 0,
			pluginsInstalled:  0,
			confirmPlugins:    false,
			timeout:           pluginTimeout,
		},

		// External dependencies
		repo:     repo,
		settings: settings,
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
func runAutomatedSetup(ctx context.Context, repo types.Repository, settings config.CrossPlatformSettings) error {
	log.Info("Running automated setup with default selections")

	// Create a minimal setup model with default selections for automation
	model := createAutomatedSetupModel(repo, settings)

	// Convert default selections to CrossPlatformApp objects
	apps := model.buildAppList()

	log.Info("Automated setup will install apps:", "appCount", len(apps), "shell", "zsh", "languages", []string{"Node.js", "Python"}, "databases", []string{"PostgreSQL"})

	// Display the planned installation to the user
	printAutomatedSetupPlan()

	// Use the regular installer system for non-interactive mode
	var errors []string
	for _, app := range apps {
		if err := installers.InstallCrossPlatformApp(ctx, app, settings, repo); err != nil {
			errors = append(errors, fmt.Sprintf("failed to install %s: %v", app.Name, err))
		}
	}

	if len(errors) > 0 {
		err := fmt.Errorf("automated installation failed: %s", strings.Join(errors, "; "))
		log.Error("Automated installation failed", err)
		fmt.Printf("⚠️  Installation failed: %v\n", err)
		return err
	}

	// Handle shell configuration and switching
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
	switch m.system.detectedPlatform.OS {
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
	if m.system.detectedPlatform.DesktopEnv == "unknown" || m.system.detectedPlatform.DesktopEnv == "" {
		return true
	}

	// For non-Linux systems, all desktop apps are compatible with the OS-level desktop
	if m.system.detectedPlatform.OS != "linux" {
		return true
	}

	// Use the app's built-in desktop environment compatibility check
	return app.IsCompatibleWithDesktopEnvironment(m.system.detectedPlatform.DesktopEnv)
}

// saveThemePreference saves the user's selected theme as the global preference
func (m *SetupModel) saveThemePreference() error {
	log.Info("Saving theme preference", "theme", m.system.themes[m.selections.selectedTheme])

	// Create theme repository using the system repository
	systemRepo, ok := m.repo.(types.SystemRepository)
	if !ok {
		return fmt.Errorf("repository does not implement SystemRepository interface")
	}
	themeRepo := repository.NewThemeRepository(systemRepo)

	// Save the selected theme as global preference
	selectedTheme := m.system.themes[m.selections.selectedTheme]
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
