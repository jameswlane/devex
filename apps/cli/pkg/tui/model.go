package tui

import (
	"fmt"
	"strings"
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
	maxLogLines = 1000 // Maximum number of log lines to keep in memory
)

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
	appStatus     map[string]bool // Track which apps have completed to prevent double-counting

	// Installation state
	needsInput    bool
	inputPrompt   string
	inputResponse chan *SecureString // Changed to SecureString

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

// AppCompleteMsg indicates that an application installation has completed,
// either successfully or with an error.
type AppCompleteMsg struct {
	AppName string
	Error   error
}

// NewModel creates and initializes a new TUI model with the provided list of
// applications to install. It sets up the progress bar, text input, and viewport
// components with appropriate styling and buffer sizes.
func NewModel(apps []types.CrossPlatformApp) Model {
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

	return Model{
		progress:      prog,
		textInput:     ti,
		viewport:      vp,
		apps:          apps,
		currentApp:    0,
		completedApps: 0,
		status:        "Ready to install applications",
		logs:          NewCircularBuffer(maxLogLines), // PERFORMANCE: Use circular buffer
		appStatus:     make(map[string]bool),
		needsInput:    false,
		inputResponse: make(chan *SecureString, channelBufferSize), // Prevent deadlocks
	}
}

// Init initializes the Bubble Tea model and returns the initial commands to start the TUI.
// This method is called once when the Bubble Tea program starts. It returns a batch of
// commands that begin text input cursor blinking and start the installation process.
//
// Returns:
//   - tea.Cmd: Batch command containing text input blink command and installation starter
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.startInstallation(),
	)
}

// Update handles incoming Bubble Tea messages and updates the model state accordingly.
// It processes window resize events, keyboard input, log messages, input requests,
// and application completion notifications.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "ctrl+c", "q":
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
		} else {
			m.viewport.SetContent("")
		}
		m.viewport.GotoBottom()

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

	case AppCompleteMsg:
		// App installation completed
		// Prevent double-counting by checking if this app has already been processed
		if _, alreadyProcessed := m.appStatus[msg.AppName]; alreadyProcessed {
			// App already processed, ignore duplicate message
			break
		}

		// Mark app as processed
		m.appStatus[msg.AppName] = true

		if msg.Error != nil {
			m.status = fmt.Sprintf("Error installing %s: %v", msg.AppName, msg.Error)
		} else {
			m.status = fmt.Sprintf("Successfully installed %s", msg.AppName)
		}

		// Atomically increment completed app counter
		completed := atomic.AddInt64(&m.completedApps, 1)

		// Use atomic counter for progress tracking instead of currentApp
		if int(completed) < len(m.apps) {
			// More apps to install - but don't start next app here to avoid race conditions
			// The installer handles sequential installation
		} else {
			m.status = "All applications installed successfully!"
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

// View renders the complete TUI interface with a 30/70 split layout.
// The left pane shows installation progress and status, while the right pane
// displays real-time terminal output from installation commands.
func (m Model) View() string {
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
func (m Model) renderLeftPane(width int) string {
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
		Render("DevEx Installation"))
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

	// Current app
	if m.currentApp < len(m.apps) {
		app := m.apps[m.currentApp]
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render("Installing:"))
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Render(app.Name))
		content.WriteString("\n")
		content.WriteString(app.Description)
		content.WriteString("\n\n")
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

// renderRightPane renders the terminal output pane
func (m Model) renderRightPane(width int) string {
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
func (m Model) startInstallation() tea.Cmd {
	return func() tea.Msg {
		return LogMsg{
			Message:   "Starting DevEx installation...",
			Timestamp: time.Now(),
			Level:     "INFO",
		}
	}
}
