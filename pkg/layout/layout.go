// layout/layout.go
package layout

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

type LayoutModel struct {
	StepsPane   viewport.Model
	LogsPane    viewport.Model
	ProgressBar progress.Model
	SystemInfo  string
}

// Initialize the layout model with dynamic pane sizes
func NewLayoutModel(systemInfo string, stepsWidth, logsWidth, height int) LayoutModel {
	// Reserve height for top bar and progress bar
	availableHeight := height - 4 // 3 lines for top bar + 1 line for progress bar
	stepsPane := viewport.New(stepsWidth, availableHeight)
	logsPane := viewport.New(logsWidth, availableHeight)
	progressBar := progress.New(progress.WithDefaultGradient())

	return LayoutModel{
		StepsPane:   stepsPane,
		LogsPane:    logsPane,
		ProgressBar: progressBar,
		SystemInfo:  systemInfo,
	}
}

// Render the top bar with system info
func renderTopBar(systemInfo string) string {
	topBarStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("42")).
		Background(lipgloss.Color("0")).
		Padding(0, 1)
	return topBarStyle.Render(systemInfo)
}

// Render the steps (left pane)
func renderSteps(steps []string, width int) string {
	stepsView := strings.Join(steps, "\n")
	leftPaneStyle := lipgloss.NewStyle().
		Width(width).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("34")) // Green border
	return leftPaneStyle.Render(stepsView)
}

// Render the console logs (right pane)
func renderLogs(logs []string, width int) string {
	logsView := strings.Join(logs, "\n")
	rightPaneStyle := lipgloss.NewStyle().
		Width(width).
		Padding(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")). // Gray border for logs
		MaxHeight(20)                            // Clip the logs if they exceed 20 lines
	return rightPaneStyle.Render(logsView)
}

// Render the progress bar (bottom bar)
func renderProgressBar(progressBar progress.Model, percent float64, width int) string {
	progressBarView := progressBar.ViewAs(percent)
	progressBarStyle := lipgloss.NewStyle().Width(width).Align(lipgloss.Left) // Align left and use full width
	return progressBarStyle.Render(progressBarView)
}

// Render the entire layout view
func (m LayoutModel) RenderView(steps []string, logs []string, percent float64) string {
	// Render top system info bar
	topBar := renderTopBar(m.SystemInfo)

	// Render steps and logs with available widths
	stepsView := renderSteps(steps, m.StepsPane.Width)
	logsView := renderLogs(logs, m.LogsPane.Width)

	// Render the progress bar to fill the bottom width
	totalWidth := m.StepsPane.Width + m.LogsPane.Width // Full width of both panes
	progressBar := renderProgressBar(m.ProgressBar, percent, totalWidth)

	// Join steps and logs in horizontal layout
	mainView := lipgloss.JoinHorizontal(
		lipgloss.Top,
		stepsView,
		logsView,
	)

	// Render the complete view (top bar, main panels, progress bar)
	return lipgloss.JoinVertical(lipgloss.Left, topBar, mainView, progressBar)
}

func (l *LayoutModel) UpdateLogsPane(logs []string) {
	logsView := strings.Join(logs, "\n")
	l.LogsPane.SetContent(logsView)
	l.LogsPane.GotoBottom()
}
