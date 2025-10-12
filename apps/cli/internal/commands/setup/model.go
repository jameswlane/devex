package setup

import (
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

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

// Init satisfies the tea.Model interface
func (m *SetupModel) Init() tea.Cmd {
	return nil
}

// NewSetupModel creates a new SetupModel for interactive setup
func NewSetupModel(repo types.Repository, settings config.CrossPlatformSettings, detectedPlatform platform.DetectionResult) *SetupModel {
	// Get plugin timeout from environment or use default
	pluginTimeout := getPluginTimeout()

	// Detect required plugins based on platform
	requiredPlugins := DetectRequiredPlugins(detectedPlatform)

	// Check for desktop environment
	hasDesktop := detectedPlatform.DesktopEnv != "none" &&
		detectedPlatform.DesktopEnv != "unknown" &&
		detectedPlatform.DesktopEnv != ""

	// Get available desktop apps if desktop is detected
	var desktopApps []string
	if hasDesktop {
		desktopApps = getDesktopAppNames(settings)
	}

	// Create plugin spinners
	pluginSpinners := make([]spinner.Model, len(requiredPlugins))
	for i := range pluginSpinners {
		s := spinner.New()
		s.Spinner = spinner.Dot
		pluginSpinners[i] = s
	}

	// Create plugin statuses
	pluginStatuses := make([]PluginStatus, len(requiredPlugins))
	for i, plugin := range requiredPlugins {
		pluginStatuses[i] = PluginStatus{
			Name:   plugin,
			Status: "pending",
		}
	}

	return &SetupModel{
		step:          StepSystemOverview,
		cursor:        0,
		shellSwitched: false,

		// System information
		system: SystemInfo{
			shells: []string{
				"zsh",
				"bash",
				"fish",
			},
			languages:        getProgrammingLanguageNames(settings),
			databases:        []string{"PostgreSQL", "MySQL", "Redis", "MongoDB"},
			desktopApps:      desktopApps,
			themes:           getAvailableThemeNames(settings),
			hasDesktop:       hasDesktop,
			detectedPlatform: detectedPlatform,
		},

		// User selections
		selections: UISelections{
			selectedShell: 0, // Default to first shell (zsh)
			selectedLangs: make(map[int]bool),
			selectedDBs:   make(map[int]bool),
			selectedApps:  make(map[int]bool),
			selectedTheme: 0,
		},

		// Git configuration
		git: GitConfiguration{
			gitFullName:    "",
			gitEmail:       "",
			gitInputField:  0,
			gitInputActive: false,
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
			requiredPlugins:   requiredPlugins,
			successfulPlugins: []string{},
			pluginsInstalling: 0,
			pluginsInstalled:  0,
			confirmPlugins:    false,
			timeout:           pluginTimeout,
			pluginStatuses:    pluginStatuses,
			pluginSpinners:    pluginSpinners,
			spinner:           spinner.New(),
		},

		// External dependencies
		repo:     repo,
		settings: settings,
	}
}

// HasErrors returns true if there were errors during installation
func (m *SetupModel) HasErrors() bool {
	return m.installation.hasInstallErrors()
}
