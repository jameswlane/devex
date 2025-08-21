package tui

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jameswlane/devex/pkg/types"
)

const (
	defaultMaxLogLines = 1000 // Default maximum number of log lines to keep in memory
)

var (
	// maxLogLines is configurable to allow customization of memory usage
	maxLogLines = defaultMaxLogLines
)

// SetMaxLogLines configures the maximum number of log lines to keep in memory
// PERFORMANCE: Lower values reduce memory usage, higher values preserve more history
func SetMaxLogLines(max int) {
	if max <= 0 {
		maxLogLines = defaultMaxLogLines
	} else {
		maxLogLines = max
	}
}

// CircularBuffer implements an efficient circular buffer for log storage
// PERFORMANCE: Avoids memory allocations during log rotation
type CircularBuffer struct {
	buffer []string
	head   int
	size   int
	cap    int
}

// NewCircularBuffer creates a new circular buffer with the specified capacity
func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		buffer: make([]string, capacity),
		head:   0,
		size:   0,
		cap:    capacity,
	}
}

// Add adds a new log entry to the circular buffer
func (cb *CircularBuffer) Add(log string) {
	cb.buffer[cb.head] = log
	cb.head = (cb.head + 1) % cb.cap
	if cb.size < cb.cap {
		cb.size++
	}
}

// GetAll returns all log entries in chronological order
func (cb *CircularBuffer) GetAll() []string {
	if cb.size == 0 {
		return nil
	}

	result := make([]string, cb.size)
	if cb.size < cb.cap {
		// Buffer not full yet, data is at the beginning
		copy(result, cb.buffer[:cb.size])
	} else {
		// Buffer is full, data wraps around
		tailSize := cb.cap - cb.head
		copy(result, cb.buffer[cb.head:])
		copy(result[tailSize:], cb.buffer[:cb.head])
	}
	return result
}

// Size returns the current number of entries in the buffer
func (cb *CircularBuffer) Size() int {
	return cb.size
}

// Model represents the main TUI state for the split-pane installation interface.
// It manages the installation progress display (left pane) and real-time command
// output streaming (right pane), along with user input handling for password prompts.
type Model struct {
	// UI Components
	progress  progress.Model
	textInput textinput.Model
	viewport  viewport.Model

	// Application state
	apps          []types.CrossPlatformApp
	currentApp    int
	completedApps int64 // Atomic counter for completed apps to prevent race conditions
	status        string
	logs          *CircularBuffer // PERFORMANCE: Use circular buffer for efficient log storage
	appStatus     sync.Map        // SECURITY: Thread-safe map for concurrent app status tracking

	// Installation state
	needsInput     bool
	inputPrompt    string
	inputResponse  chan *SecureString // Changed to SecureString
	channelCleaned bool               // Track channel cleanup to prevent memory leaks
	cleanupMux     sync.Mutex         // Protect cleanup operations
	startTime      time.Time          // Track installation start time

	// Layout
	width  int
	height int
	ready  bool
}

// InstallStatus represents the current installation state for tracking
// progress through the installation lifecycle.
type InstallStatus int

const (
	StatusReady InstallStatus = iota
	StatusInstalling
	StatusWaitingInput
	StatusComplete
	StatusError
)

// AppProgress represents installation progress and status information
// for a single application in the installation queue.
type AppProgress struct {
	Name     string
	Status   InstallStatus
	Progress float64
	Error    error
}

// LogMsg represents a timestamped log message that will be displayed
// in the terminal output viewport of the TUI.
type LogMsg struct {
	Message   string
	Timestamp time.Time
	Level     string
}

// InputRequestMsg requests user input for interactive prompts such as
// password requests during package installation.
type InputRequestMsg struct {
	Prompt   string
	Response chan *SecureString
}

// AppStartedMsg indicates that an application installation has started
type AppStartedMsg struct {
	AppName  string
	AppIndex int
}

// AppCompleteMsg indicates that an application installation has completed,
// either successfully or with an error.
type AppCompleteMsg struct {
	AppName string
	Error   error
}

// NewModel creates and initializes a new TUI model with the provided list of
// applications to install. It sets up the progress bar, text input, and viewport
// components with appropriate styling and buffer sizes.
func NewModel(apps []types.CrossPlatformApp) *Model {
	// Initialize progress bar
	prog := progress.New(progress.WithDefaultGradient())
	prog.Width = 25

	// Initialize text input for password/prompts
	ti := textinput.New()
	ti.Placeholder = "Enter input..."
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 156

	// Initialize viewport for logs
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	return &Model{
		progress:       prog,
		textInput:      ti,
		viewport:       vp,
		apps:           apps,
		currentApp:     0,
		completedApps:  0,
		status:         "Ready to install applications",
		logs:           NewCircularBuffer(maxLogLines), // PERFORMANCE: Use circular buffer with configurable size
		appStatus:      sync.Map{},                     // SECURITY: Thread-safe concurrent map
		needsInput:     false,
		inputResponse:  make(chan *SecureString, 5), // Default channel buffer size to prevent deadlocks
		channelCleaned: false,
		startTime:      time.Now(), // Track when installation starts
	}
}

// Init initializes the Bubble Tea model and returns the initial commands to start the TUI.
// This method is called once when the Bubble Tea program starts. It returns a batch of
// commands that begin text input cursor blinking and start the installation process.
//
// Returns:
//   - tea.Cmd: Batch command containing text input blink command and installation starter
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.startInstallation(),
	)
}

// Update handles incoming Bubble Tea messages and updates the model state accordingly.
// It processes window resize events, keyboard input, log messages, input requests,
// and application completion notifications.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update viewport size (70% width, most of height)
		m.viewport.Width = int(float64(msg.Width)*0.7) - 4
		m.viewport.Height = msg.Height - 4

		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit

		case "enter":
			if m.needsInput {
				// Send secure input response with non-blocking send to prevent deadlocks
				response := NewSecureString(m.textInput.Value())
				select {
				case m.inputResponse <- response:
					// Response sent successfully
				default:
					// Channel is full or closed, clean up the response
					response.Clear()
				}
				m.textInput.SetValue("")
				m.needsInput = false
				m.inputPrompt = ""
				return m, nil
			}
		}

		// Handle text input
		if m.needsInput {
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
		}

	case LogMsg:
		// Add log message to viewport using efficient circular buffer
		logLine := fmt.Sprintf("[%s] %s: %s",
			msg.Timestamp.Format("15:04:05"),
			msg.Level,
			msg.Message)

		// PERFORMANCE: Use circular buffer for efficient log storage (no more slice reallocations)
		m.logs.Add(logLine)

		// Update viewport content with all logs
		allLogs := m.logs.GetAll()
		if allLogs != nil {
			m.viewport.SetContent(strings.Join(allLogs, "\n"))
			// Only go to bottom if viewport is ready and has content
			if m.ready && len(allLogs) > 0 {
				m.viewport.GotoBottom()
			}
		} else {
			m.viewport.SetContent("")
		}

	case InputRequestMsg:
		// Request user input
		m.needsInput = true
		m.inputPrompt = msg.Prompt
		m.inputResponse = msg.Response
		m.textInput.Focus()

		// Determine if this is a password prompt
		if strings.Contains(strings.ToLower(msg.Prompt), "password") {
			m.textInput.EchoMode = textinput.EchoPassword
		} else {
			m.textInput.EchoMode = textinput.EchoNormal
		}

	case AppStartedMsg:
		// App installation started - update current app display
		m.currentApp = msg.AppIndex
		m.status = fmt.Sprintf("Installing %s...", msg.AppName)

	case AppCompleteMsg:
		// App installation completed
		// SECURITY: Prevent double-counting using thread-safe sync.Map
		if _, alreadyProcessed := m.appStatus.LoadOrStore(msg.AppName, true); alreadyProcessed {
			// App already processed, ignore duplicate message
			break
		}

		if msg.Error != nil {
			m.status = fmt.Sprintf("Error installing %s: %v", msg.AppName, msg.Error)
		} else {
			m.status = fmt.Sprintf("Successfully installed %s", msg.AppName)
		}

		// Count completed apps by iterating over the sync.Map
		completed := int64(0)
		m.appStatus.Range(func(key, value interface{}) bool {
			completed++
			return true
		})

		// Store the count atomically for thread-safe access
		atomic.StoreInt64(&m.completedApps, completed)

		// Update progress model internal state for tests
		if len(m.apps) > 0 {
			currentProgress := float64(completed) / float64(len(m.apps))
			m.progress.SetPercent(currentProgress)
		}

		// Use the count for progress tracking
		if int(completed) < len(m.apps) {
			// More apps to install - but don't start next app here to avoid race conditions
			// The installer handles sequential installation
		} else {
			// All apps completed - update display
			m.status = "All applications installed successfully!"
			m.currentApp = len(m.apps) // Hide current app display
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		if progressModel, ok := progressModel.(progress.Model); ok {
			m.progress = progressModel
		}
		cmds = append(cmds, cmd)
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// CleanupChannels safely closes and cleans up channels to prevent memory leaks
// SECURITY: Prevents memory leaks from abandoned channels during context cancellation
func (m *Model) CleanupChannels() {
	m.cleanupMux.Lock()
	defer m.cleanupMux.Unlock()

	if !m.channelCleaned && m.inputResponse != nil {
		// Drain any pending messages before closing
		select {
		case response := <-m.inputResponse:
			if response != nil {
				response.Clear() // Clean up any secure strings
			}
		default:
			// No pending messages
		}

		close(m.inputResponse)
		m.channelCleaned = true
	}
}

// View renders the complete TUI interface with a 30/70 split layout.
// The left pane shows installation progress and status, while the right pane
// displays real-time terminal output from installation commands.
func (m *Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Calculate layout dimensions
	leftWidth := int(float64(m.width) * 0.3)
	rightWidth := int(float64(m.width) * 0.7)

	// Left pane content
	leftContent := m.renderLeftPane(leftWidth)

	// Right pane content
	rightContent := m.renderRightPane(rightWidth)

	// Combine panes
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftContent,
		rightContent,
	)
}

// renderLeftPane renders the status/progress pane
func (m *Model) renderLeftPane(width int) string {
	leftStyle := lipgloss.NewStyle().
		Width(width).
		Height(m.height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1)

	var content strings.Builder

	// Title
	content.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render("DevEx Application Installer"))
	content.WriteString("\n\n")

	// Current status
	content.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("246")).
		Render("Status:"))
	content.WriteString("\n")
	content.WriteString(m.status)
	content.WriteString("\n\n")

	// Progress
	if len(m.apps) > 0 {
		// Use atomic counter for thread-safe progress tracking
		completed := atomic.LoadInt64(&m.completedApps)
		currentProgress := float64(completed) / float64(len(m.apps))
		progressView := m.progress.ViewAs(currentProgress)

		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render("Progress:"))
		content.WriteString("\n")
		content.WriteString(progressView)
		content.WriteString("\n")
		content.WriteString(fmt.Sprintf("%d/%d apps", completed, len(m.apps)))
		content.WriteString("\n\n")
	}

	// Current app detailed information
	if m.currentApp < len(m.apps) {
		app := m.apps[m.currentApp]
		appDetails := m.getAppDetails(app)

		// App Name and Category
		content.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Render(fmt.Sprintf("📦 %s", appDetails.Name)))
		content.WriteString("\n")

		if appDetails.Category != "" {
			content.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("246")).
				Italic(true).
				Render(fmt.Sprintf("Category: %s", appDetails.Category)))
			content.WriteString("\n")
		}

		// Description
		content.WriteString(appDetails.Description)
		content.WriteString("\n\n")

		// Installation Details Section
		content.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")).
			Render("🔧 Installation Details"))
		content.WriteString("\n")

		// Installation method with icon
		methodIcon := getMethodIcon(appDetails.InstallMethod)
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render(fmt.Sprintf("Method: %s %s", methodIcon, appDetails.InstallMethod)))
		content.WriteString("\n")

		// Official support status
		supportStatus := "❌ Community"
		if appDetails.OfficialSupport {
			supportStatus = "✅ Official"
		}
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render(fmt.Sprintf("Support: %s", supportStatus)))
		content.WriteString("\n")

		// Size and time estimates
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render(fmt.Sprintf("Size: %s", appDetails.EstimatedSize)))
		content.WriteString("\n")

		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render(fmt.Sprintf("Time: ~%s", formatDuration(appDetails.EstimatedTime))))
		content.WriteString("\n")

		// Install location
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render(fmt.Sprintf("Location: %s", appDetails.InstallLocation)))
		content.WriteString("\n\n")

		// Dependencies (if any)
		if len(appDetails.Dependencies) > 0 {
			content.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214")).
				Render("📋 Dependencies"))
			content.WriteString("\n")
			for _, dep := range appDetails.Dependencies {
				content.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("246")).
					Render(fmt.Sprintf("• %s", dep)))
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}

		// Conflicts (if any)
		if len(appDetails.Conflicts) > 0 {
			content.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("203")).
				Render("⚠️  Conflicts"))
			content.WriteString("\n")
			for _, conflict := range appDetails.Conflicts {
				content.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("203")).
					Render(fmt.Sprintf("• %s", conflict)))
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}

		// Time tracking
		if !m.startTime.IsZero() {
			elapsed := time.Since(m.startTime)
			content.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("214")).
				Render("⏱️  Timing"))
			content.WriteString("\n")
			content.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color("246")).
				Render(fmt.Sprintf("Elapsed: %s", formatDuration(elapsed))))
			content.WriteString("\n")

			// Calculate remaining time estimate
			if len(m.apps) > 0 {
				completed := atomic.LoadInt64(&m.completedApps)
				if completed > 0 {
					avgTimePerApp := elapsed / time.Duration(completed)
					remaining := time.Duration(len(m.apps)-int(completed)) * avgTimePerApp
					content.WriteString(lipgloss.NewStyle().
						Foreground(lipgloss.Color("246")).
						Render(fmt.Sprintf("Remaining: ~%s", formatDuration(remaining))))
					content.WriteString("\n")
				}
			}
			content.WriteString("\n")
		}
	}

	// Input prompt
	if m.needsInput {
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true).
			Render("Input Required:"))
		content.WriteString("\n")
		content.WriteString(m.inputPrompt)
		content.WriteString("\n\n")
		content.WriteString(m.textInput.View())
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render("Press Enter to submit"))
	}

	return leftStyle.Render(content.String())
}

// getAppDetails extracts detailed information about an app for display
func (m *Model) getAppDetails(app types.CrossPlatformApp) AppDisplayInfo {
	osConfig := app.GetOSConfig()

	return AppDisplayInfo{
		Name:            app.Name,
		Description:     app.Description,
		Category:        app.Category,
		InstallMethod:   osConfig.InstallMethod,
		OfficialSupport: osConfig.OfficialSupport,
		Dependencies:    osConfig.Dependencies,
		Conflicts:       osConfig.Conflicts,
		EstimatedSize:   estimateAppSize(app),
		EstimatedTime:   estimateInstallTime(osConfig.InstallMethod),
		InstallLocation: getInstallLocation(osConfig),
	}
}

// AppDisplayInfo holds all the information we want to display about an app
type AppDisplayInfo struct {
	Name            string
	Description     string
	Category        string
	InstallMethod   string
	OfficialSupport bool
	Dependencies    []string
	Conflicts       []string
	EstimatedSize   string
	EstimatedTime   time.Duration
	InstallLocation string
}

// estimateAppSize provides a rough estimate of app installation size using simple heuristics
func estimateAppSize(app types.CrossPlatformApp) string {
	// Use simple heuristics similar to the performance analyzer's known sizes
	knownSizes := map[string]string{
		"docker":         "~450MB",
		"docker-compose": "~50MB",
		"node":           "~80MB",
		"nodejs":         "~80MB",
		"python":         "~100MB",
		"rust":           "~250MB",
		"go":             "~350MB",
		"java":           "~180MB",
		"vscode":         "~350MB",
		"chrome":         "~200MB",
		"firefox":        "~150MB",
		"postgresql":     "~150MB",
		"mysql":          "~450MB",
		"mongodb":        "~250MB",
		"redis":          "~50MB",
		"nginx":          "~30MB",
		"apache2":        "~50MB",
		"git":            "~20MB",
		"vim":            "~15MB",
		"emacs":          "~50MB",
		"curl":           "~5MB",
		"wget":           "~3MB",
		"zsh":            "~10MB",
		"fish":           "~15MB",
		"tmux":           "~5MB",
	}

	appNameLower := strings.ToLower(app.Name)
	if size, exists := knownSizes[appNameLower]; exists {
		return size
	}

	// Check for partial matches
	for knownApp, size := range knownSizes {
		if strings.Contains(appNameLower, knownApp) || strings.Contains(knownApp, appNameLower) {
			return size
		}
	}

	osConfig := app.GetOSConfig()

	// Size estimates based on installation method
	switch osConfig.InstallMethod {
	case "snap":
		return "~50-200MB"
	case "flatpak":
		return "~100-500MB"
	case "apt", "dnf", "pacman":
		if strings.Contains(strings.ToLower(app.Category), "development") {
			return "~20-100MB"
		}
		return "~5-50MB"
	case "brew":
		return "~10-100MB"
	case "curlpipe", "mise":
		return "~1-20MB"
	default:
		return "~50MB"
	}
}

// estimateInstallTime provides rough time estimates based on installation method and app type
func estimateInstallTime(method string) time.Duration {
	switch method {
	case "apt", "dnf", "pacman":
		return 30 * time.Second
	case "snap":
		return 60 * time.Second
	case "flatpak":
		return 90 * time.Second
	case "brew":
		return 45 * time.Second
	case "curlpipe":
		return 20 * time.Second
	case "mise":
		return 60 * time.Second // Language installations can take longer
	case "docker":
		return 120 * time.Second // Docker installs can be lengthy
	default:
		return 30 * time.Second
	}
}

// getInstallLocation determines where the app will be installed
func getInstallLocation(osConfig types.OSConfig) string {
	if osConfig.Destination != "" {
		return osConfig.Destination
	}

	switch osConfig.InstallMethod {
	case "apt", "dnf", "pacman":
		return "/usr/bin"
	case "snap":
		return "/snap/bin"
	case "flatpak":
		return "~/.local/share/flatpak"
	case "brew":
		return "/opt/homebrew/bin"
	case "curlpipe":
		return "~/.local/bin"
	case "mise":
		return "~/.local/share/mise"
	default:
		return "System default"
	}
}

// getMethodIcon returns an icon for the installation method
func getMethodIcon(method string) string {
	switch method {
	case "apt", "dnf", "pacman", "zypper":
		return "📦"
	case "snap":
		return "🫰"
	case "flatpak":
		return "📱"
	case "brew":
		return "🍺"
	case "curlpipe":
		return "⬇️"
	case "mise":
		return "🔧"
	case "docker":
		return "🐳"
	case "pip", "pip3":
		return "🐍"
	case "npm", "yarn", "pnpm":
		return "📦"
	case "cargo":
		return "🦀"
	case "go":
		return "🐹"
	default:
		return "⚙️"
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// renderRightPane renders the terminal output pane
func (m *Model) renderRightPane(width int) string {
	rightStyle := lipgloss.NewStyle().
		Width(width).
		Height(m.height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1)

	var content strings.Builder

	// Title
	content.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render("Terminal Output"))
	content.WriteString("\n\n")

	// Viewport with logs
	content.WriteString(m.viewport.View())

	return rightStyle.Render(content.String())
}

// startInstallation begins the installation process
func (m *Model) startInstallation() tea.Cmd {
	return func() tea.Msg {
		return LogMsg{
			Message:   "Starting DevEx installation...",
			Timestamp: time.Now(),
			Level:     "INFO",
		}
	}
}
