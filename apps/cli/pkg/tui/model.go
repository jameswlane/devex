package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jameswlane/devex/pkg/types"
)

// Model represents the main TUI state
type Model struct {
	// UI Components
	progress  progress.Model
	textInput textinput.Model
	viewport  viewport.Model

	// Application state
	apps       []types.AppConfig
	currentApp int
	status     string
	logs       []string

	// Installation state
	needsInput    bool
	inputPrompt   string
	inputResponse chan string

	// Layout
	width  int
	height int
	ready  bool
}

// InstallStatus represents the current installation state
type InstallStatus int

const (
	StatusReady InstallStatus = iota
	StatusInstalling
	StatusWaitingInput
	StatusComplete
	StatusError
)

// AppProgress represents progress for a single app
type AppProgress struct {
	Name     string
	Status   InstallStatus
	Progress float64
	Error    error
}

// LogMsg represents a log message for the viewport
type LogMsg struct {
	Message   string
	Timestamp time.Time
	Level     string
}

// InputRequestMsg requests user input
type InputRequestMsg struct {
	Prompt   string
	Response chan string
}

// AppCompleteMsg indicates an app installation completed
type AppCompleteMsg struct {
	AppName string
	Error   error
}

// NewModel creates a new TUI model
func NewModel(apps []types.AppConfig) Model {
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
		status:        "Ready to install applications",
		logs:          []string{},
		needsInput:    false,
		inputResponse: make(chan string, 1),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.startInstallation(),
	)
}

// Update handles messages and updates the model
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
				// Send input response
				response := m.textInput.Value()
				m.inputResponse <- response
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
		// Add log message to viewport
		logLine := fmt.Sprintf("[%s] %s: %s",
			msg.Timestamp.Format("15:04:05"),
			msg.Level,
			msg.Message)
		m.logs = append(m.logs, logLine)
		m.viewport.SetContent(strings.Join(m.logs, "\n"))
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
		if msg.Error != nil {
			m.status = fmt.Sprintf("Error installing %s: %v", msg.AppName, msg.Error)
		} else {
			m.status = fmt.Sprintf("Successfully installed %s", msg.AppName)
			m.currentApp++
		}

		// Start next app or complete
		if m.currentApp < len(m.apps) {
			cmds = append(cmds, m.installNextApp())
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

// View renders the TUI
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
		currentProgress := float64(m.currentApp) / float64(len(m.apps))
		progressView := m.progress.ViewAs(currentProgress)

		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render("Progress:"))
		content.WriteString("\n")
		content.WriteString(progressView)
		content.WriteString("\n")
		content.WriteString(fmt.Sprintf("%d/%d apps", m.currentApp, len(m.apps)))
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

// installNextApp starts installing the next app
func (m Model) installNextApp() tea.Cmd {
	if m.currentApp >= len(m.apps) {
		return nil
	}

	app := m.apps[m.currentApp]
	return func() tea.Msg {
		return LogMsg{
			Message:   fmt.Sprintf("Installing %s...", app.Name),
			Timestamp: time.Now(),
			Level:     "INFO",
		}
	}
}
