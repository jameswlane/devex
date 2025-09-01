package tui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	progresspkg "github.com/jameswlane/devex/apps/cli/internal/progress"
)

// ProgressModel represents an enhanced TUI model for progress tracking
// that supports multiple operation types beyond just application installation
type ProgressModel struct {
	// UI Components
	viewport  viewport.Model
	textInput textinput.Model

	// Progress tracking
	manager    *progresspkg.ProgressManager
	operations map[string]*progresspkg.Operation
	opMutex    sync.RWMutex

	// Display state
	logs         *CircularBuffer
	currentView  ViewMode
	selectedOpID string
	showDetails  bool

	// Input handling
	needsInput     bool
	inputPrompt    string
	inputResponse  chan *SecureString
	channelCleaned bool
	cleanupMux     sync.Mutex

	// Layout
	width  int
	height int
	ready  bool

	// Operation-specific UI components
	progressBars map[string]progress.Model
	barMutex     sync.RWMutex
}

// ViewMode represents different display modes for the progress UI
type ViewMode string

const (
	ViewOverview ViewMode = "overview" // Show all operations summary
	ViewDetailed ViewMode = "detailed" // Show detailed view of operations
	ViewLogs     ViewMode = "logs"     // Show operation logs
)

// NewProgressModel creates a new enhanced progress model
func NewProgressModel(manager *progresspkg.ProgressManager) *ProgressModel {
	// Initialize viewport for logs/output
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62"))

	// Initialize text input for prompts
	ti := textinput.New()
	ti.Placeholder = "Enter input..."
	ti.EchoMode = textinput.EchoPassword
	ti.CharLimit = 156

	model := &ProgressModel{
		viewport:       vp,
		textInput:      ti,
		manager:        manager,
		operations:     make(map[string]*progresspkg.Operation),
		logs:           NewCircularBuffer(maxLogLines),
		currentView:    ViewOverview,
		progressBars:   make(map[string]progress.Model),
		inputResponse:  make(chan *SecureString, 5),
		channelCleaned: false,
	}

	// Add ourselves as a progress listener
	manager.GetTracker().AddListener(model)

	return model
}

// OnProgressUpdate implements ProgressListener interface
func (m *ProgressModel) OnProgressUpdate(state *progresspkg.ProgressState) {
	m.opMutex.Lock()
	defer m.opMutex.Unlock()

	// Update our operations map
	if op := m.manager.GetTracker().GetOperation(state.ID); op != nil {
		m.operations[state.ID] = op
	}

	// Create or update progress bar for this operation
	m.barMutex.Lock()
	if _, exists := m.progressBars[state.ID]; !exists {
		bar := progress.New(progress.WithDefaultGradient())
		bar.Width = 40
		m.progressBars[state.ID] = bar
	}
	m.barMutex.Unlock()
}

// Init initializes the progress model
func (m *ProgressModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.watchProgress(),
	)
}

// Update handles Bubble Tea messages
func (m *ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8 // Leave room for header and footer
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab":
			// Cycle through view modes
			switch m.currentView {
			case ViewOverview:
				m.currentView = ViewDetailed
			case ViewDetailed:
				m.currentView = ViewLogs
			case ViewLogs:
				m.currentView = ViewOverview
			}
			return m, nil

		case "enter":
			if m.needsInput {
				response := NewSecureString(m.textInput.Value())
				select {
				case m.inputResponse <- response:
					// Response sent successfully
				default:
					response.Clear()
				}
				m.textInput.SetValue("")
				m.needsInput = false
				m.inputPrompt = ""
				return m, nil
			}

		case "d":
			// Toggle details view
			m.showDetails = !m.showDetails
			return m, nil

		case "up", "k":
			// Navigate up in operations list
			m.navigateOperations(-1)
			return m, nil

		case "down", "j":
			// Navigate down in operations list
			m.navigateOperations(1)
			return m, nil
		}

		// Handle text input
		if m.needsInput {
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
		}

	case progresspkg.ProgressUpdateMsg:
		// Handle progress updates from our tracking system
		m.OnProgressUpdate(msg.State)

	case LogMsg:
		// Add log message to viewport
		logLine := fmt.Sprintf("[%s] %s: %s",
			msg.Timestamp.Format("15:04:05"),
			msg.Level,
			msg.Message)

		m.logs.Add(logLine)
		if m.currentView == ViewLogs {
			m.updateViewportContent()
		}

	case InputRequestMsg:
		// Handle input requests
		m.needsInput = true
		m.inputPrompt = msg.Prompt
		m.inputResponse = msg.Response
		m.textInput.Focus()

		if strings.Contains(strings.ToLower(msg.Prompt), "password") {
			m.textInput.EchoMode = textinput.EchoPassword
		} else {
			m.textInput.EchoMode = textinput.EchoNormal
		}

	case progress.FrameMsg:
		// Update all progress bars
		m.barMutex.Lock()
		for id, bar := range m.progressBars {
			updatedBar, barCmd := bar.Update(msg)
			if updatedBar, ok := updatedBar.(progress.Model); ok {
				m.progressBars[id] = updatedBar
				cmds = append(cmds, barCmd)
			}
		}
		m.barMutex.Unlock()
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the progress UI
func (m *ProgressModel) View() string {
	if !m.ready {
		return "Initializing progress tracking..."
	}

	var content strings.Builder

	// Header
	content.WriteString(m.renderHeader())
	content.WriteString("\n\n")

	// Main content based on current view
	switch m.currentView {
	case ViewOverview:
		content.WriteString(m.renderOverview())
	case ViewDetailed:
		content.WriteString(m.renderDetailed())
	case ViewLogs:
		content.WriteString(m.renderLogs())
	}

	// Input prompt if needed
	if m.needsInput {
		content.WriteString("\n\n")
		content.WriteString(m.renderInputPrompt())
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(m.renderFooter())

	return content.String()
}

// renderHeader renders the header with navigation tabs
func (m *ProgressModel) renderHeader() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render("DevEx Progress Tracker")

	// View tabs
	tabs := []string{"Overview", "Details", "Logs"}
	tabViews := make([]string, 0, len(tabs))

	for i, tab := range tabs {
		style := lipgloss.NewStyle().Padding(0, 2)

		isActive := (i == 0 && m.currentView == ViewOverview) ||
			(i == 1 && m.currentView == ViewDetailed) ||
			(i == 2 && m.currentView == ViewLogs)

		if isActive {
			style = style.Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
		} else {
			style = style.Foreground(lipgloss.Color("246"))
		}

		tabViews = append(tabViews, style.Render(tab))
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)

	return lipgloss.JoinVertical(lipgloss.Left, title, tabBar)
}

// renderOverview renders the overview of all operations
func (m *ProgressModel) renderOverview() string {
	m.opMutex.RLock()
	defer m.opMutex.RUnlock()

	if len(m.operations) == 0 {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render("No operations in progress...")
	}

	var content strings.Builder

	// Group operations by type
	opsByType := make(map[progresspkg.OperationType][]*progresspkg.Operation)
	for _, op := range m.operations {
		state := op.GetState()
		opsByType[state.Type] = append(opsByType[state.Type], op)
	}

	// Sort operation types for consistent display
	types := make([]progresspkg.OperationType, 0, len(opsByType))
	for opType := range opsByType {
		types = append(types, opType)
	}
	sort.Slice(types, func(i, j int) bool {
		return string(types[i]) < string(types[j])
	})

	for _, opType := range types {
		ops := opsByType[opType]

		// Type header
		typeIcon := progresspkg.GetOperationTypeIcon(opType)
		opTypeName := string(opType)
		if len(opTypeName) > 0 {
			opTypeName = strings.ToUpper(opTypeName[:1]) + opTypeName[1:]
		}
		typeHeader := fmt.Sprintf("%s %s Operations (%d)", typeIcon, opTypeName, len(ops))
		content.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			Render(typeHeader))
		content.WriteString("\n")

		// Operations summary
		for _, op := range ops {
			content.WriteString(m.renderOperationSummary(op))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	return content.String()
}

// renderDetailed renders detailed view of operations
func (m *ProgressModel) renderDetailed() string {
	m.opMutex.RLock()
	defer m.opMutex.RUnlock()

	if len(m.operations) == 0 {
		return "No operations to display"
	}

	var content strings.Builder

	// Get operations sorted by start time
	ops := make([]*progresspkg.Operation, 0, len(m.operations))
	for _, op := range m.operations {
		ops = append(ops, op)
	}

	sort.Slice(ops, func(i, j int) bool {
		return ops[i].GetState().StartTime.Before(ops[j].GetState().StartTime)
	})

	for i, op := range ops {
		if i > 0 {
			content.WriteString("\n")
			content.WriteString(strings.Repeat("─", m.width-4))
			content.WriteString("\n\n")
		}
		content.WriteString(m.renderOperationDetail(op))
	}

	return content.String()
}

// renderLogs renders the logs view
func (m *ProgressModel) renderLogs() string {
	m.updateViewportContent()
	return m.viewport.View()
}

// renderOperationSummary renders a single operation summary
func (m *ProgressModel) renderOperationSummary(op *progresspkg.Operation) string {
	state := op.GetState()

	statusIcon := progresspkg.GetStatusIcon(state.Status)
	progressPercent := int(state.Progress * 100)

	// Progress bar
	m.barMutex.RLock()
	bar, hasBar := m.progressBars[state.ID]
	m.barMutex.RUnlock()

	var progressView string
	if hasBar {
		progressView = bar.ViewAs(state.Progress)
	} else {
		progressView = fmt.Sprintf("[%3d%%]", progressPercent)
	}

	summary := fmt.Sprintf("%s %s %s %s",
		statusIcon,
		state.Name,
		progressView,
		progresspkg.FormatDuration(op.GetDuration()))

	// Add details if available
	if state.Details != "" {
		summary += fmt.Sprintf("\n   %s", state.Details)
	}

	// Highlight selected operation
	style := lipgloss.NewStyle()
	if state.ID == m.selectedOpID {
		style = style.Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	}

	return style.Render(summary)
}

// renderOperationDetail renders detailed information about an operation
func (m *ProgressModel) renderOperationDetail(op *progresspkg.Operation) string {
	state := op.GetState()
	var content strings.Builder

	// Header
	statusIcon := progresspkg.GetStatusIcon(state.Status)
	typeIcon := progresspkg.GetOperationTypeIcon(state.Type)
	header := fmt.Sprintf("%s %s %s", statusIcon, typeIcon, state.Name)
	content.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		Render(header))
	content.WriteString("\n")

	// Description
	if state.Description != "" {
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("246")).
			Render(state.Description))
		content.WriteString("\n")
	}

	// Progress bar
	m.barMutex.RLock()
	bar, hasBar := m.progressBars[state.ID]
	m.barMutex.RUnlock()

	if hasBar {
		content.WriteString(bar.ViewAs(state.Progress))
	} else {
		content.WriteString(fmt.Sprintf("Progress: %.1f%%", state.Progress*100))
	}
	content.WriteString("\n")

	// Status and timing
	content.WriteString(fmt.Sprintf("Status: %s %s", statusIcon, state.Status))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("Duration: %s", progresspkg.FormatDuration(op.GetDuration())))
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("Started: %s", state.StartTime.Format("15:04:05")))
	content.WriteString("\n")

	if state.EndTime != nil {
		content.WriteString(fmt.Sprintf("Ended: %s", state.EndTime.Format("15:04:05")))
		content.WriteString("\n")
	}

	// Current details
	if state.Details != "" {
		content.WriteString(fmt.Sprintf("Details: %s", state.Details))
		content.WriteString("\n")
	}

	// Error if present
	if state.Error != nil {
		content.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("Error: %v", state.Error)))
		content.WriteString("\n")
	}

	// Metadata if present and details are shown
	if m.showDetails && len(state.Metadata) > 0 {
		content.WriteString("\nMetadata:\n")
		for key, value := range state.Metadata {
			content.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
	}

	// Child operations
	children := op.GetChildren()
	if len(children) > 0 {
		content.WriteString(fmt.Sprintf("\nChild Operations (%d):\n", len(children)))
		for _, child := range children {
			childState := child.GetState()
			childIcon := progresspkg.GetStatusIcon(childState.Status)
			content.WriteString(fmt.Sprintf("  %s %s (%.1f%%)\n",
				childIcon, childState.Name, childState.Progress*100))
		}
	}

	return content.String()
}

// renderInputPrompt renders the input prompt area
func (m *ProgressModel) renderInputPrompt() string {
	var content strings.Builder

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

	return content.String()
}

// renderFooter renders the footer with navigation help
func (m *ProgressModel) renderFooter() string {
	help := []string{
		"Tab: Switch view",
		"↑/↓: Navigate",
		"d: Toggle details",
		"q: Quit",
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("246")).
		Render(strings.Join(help, " • "))
}

// navigateOperations handles navigation through the operations list
func (m *ProgressModel) navigateOperations(direction int) {
	m.opMutex.RLock()
	defer m.opMutex.RUnlock()

	if len(m.operations) == 0 {
		return
	}

	// Get sorted operation IDs
	opIDs := make([]string, 0, len(m.operations))
	for id := range m.operations {
		opIDs = append(opIDs, id)
	}
	sort.Strings(opIDs)

	// Find current selection index
	currentIndex := -1
	for i, id := range opIDs {
		if id == m.selectedOpID {
			currentIndex = i
			break
		}
	}

	// Calculate new index
	newIndex := currentIndex + direction
	if newIndex < 0 {
		newIndex = len(opIDs) - 1
	} else if newIndex >= len(opIDs) {
		newIndex = 0
	}

	m.selectedOpID = opIDs[newIndex]
}

// updateViewportContent updates the viewport content based on current view
func (m *ProgressModel) updateViewportContent() {
	if m.currentView == ViewLogs {
		allLogs := m.logs.GetAll()
		if allLogs != nil {
			m.viewport.SetContent(strings.Join(allLogs, "\n"))
			m.viewport.GotoBottom()
		} else {
			m.viewport.SetContent("No logs available")
		}
	}
}

// watchProgress returns a command that watches for progress updates
func (m *ProgressModel) watchProgress() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return progress.FrameMsg{}
	})
}

// CleanupChannels safely closes and cleans up channels
func (m *ProgressModel) CleanupChannels() {
	m.cleanupMux.Lock()
	defer m.cleanupMux.Unlock()

	if !m.channelCleaned && m.inputResponse != nil {
		select {
		case response := <-m.inputResponse:
			if response != nil {
				response.Clear()
			}
		default:
			// No pending messages
		}

		close(m.inputResponse)
		m.channelCleaned = true
	}
}
