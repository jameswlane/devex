package setup

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// DynamicSetupModel is a Bubble Tea model for dynamic setup workflows
type DynamicSetupModel struct {
	executor        *SetupExecutor
	actionExecutor  *ActionExecutor
	textInput       textinput.Model
	cursor          int
	selected        map[int]bool
	validationError string
	spinner         spinner.Model
	executing       bool
	err             error
	quitting        bool
}

// ActionCompleteMsg is sent when an action completes
type ActionCompleteMsg struct {
	err error
}

// NewDynamicSetupModel creates a new dynamic setup model
func NewDynamicSetupModel(
	setupConfig *types.SetupConfig,
	repo types.Repository,
	settings config.CrossPlatformSettings,
	detectedPlatform platform.DetectionResult,
	pluginBootstrap *bootstrap.PluginBootstrap,
) *DynamicSetupModel {
	executor := NewSetupExecutor(setupConfig, settings, repo, detectedPlatform)
	actionExecutor := NewActionExecutor(pluginBootstrap, settings, detectedPlatform)

	ti := textinput.New()
	ti.Placeholder = "Enter value..."
	ti.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

	return &DynamicSetupModel{
		executor:       executor,
		actionExecutor: actionExecutor,
		textInput:      ti,
		selected:       make(map[int]bool),
		spinner:        s,
	}
}

// Init initializes the model
func (m *DynamicSetupModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (m *DynamicSetupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case ActionCompleteMsg:
		m.executing = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Action completed successfully, move to next step
		if err := m.executor.NextStep(); err != nil {
			m.err = err
			return m, nil
		}
		// Check if we've reached the end
		if m.executor.IsComplete() {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		if m.executing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	default:
		// Update text input if in text question
		step := m.executor.GetCurrentStep()
		if step != nil && step.Type == types.StepTypeQuestion && step.Question != nil {
			if step.Question.Type == types.QuestionTypeText {
				var cmd tea.Cmd
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}
	}

	return m, nil
}

// handleKeyPress handles keyboard input
func (m *DynamicSetupModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	step := m.executor.GetCurrentStep()
	if step == nil {
		return m, tea.Quit
	}

	switch msg.String() {
	case "ctrl+c", "q":
		// Only allow quitting on info steps or if not in the middle of an action
		if !m.executing && (step.Type == types.StepTypeInfo || step.Type == types.StepTypeQuestion) {
			m.quitting = true
			return m, tea.Quit
		}

	case "b", "backspace":
		// Go back if allowed
		if !m.executing && step.Navigation.AllowBack {
			if err := m.executor.PrevStep(); err != nil {
				m.validationError = err.Error()
			} else {
				m.resetCursor()
			}
		}

	case "enter":
		return m.handleEnter(step)

	case "up", "k":
		if step.Type == types.StepTypeQuestion && !m.executing {
			m.cursor--
			if m.cursor < 0 {
				m.cursor = 0
			}
		}

	case "down", "j":
		if step.Type == types.StepTypeQuestion && !m.executing {
			if step.Question != nil {
				maxOptions := len(step.Question.Options)
				m.cursor++
				if m.cursor >= maxOptions {
					m.cursor = maxOptions - 1
				}
			}
		}

	case " ", "space":
		// Toggle selection for multiselect
		if step.Type == types.StepTypeQuestion && step.Question != nil {
			if step.Question.Type == types.QuestionTypeMultiSelect {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		}
	}

	return m, nil
}

// handleEnter processes the Enter key based on current step
func (m *DynamicSetupModel) handleEnter(step *types.SetupStep) (tea.Model, tea.Cmd) {
	if m.executing {
		return m, nil
	}

	switch step.Type {
	case types.StepTypeInfo:
		// Info steps just advance to next
		if err := m.executor.NextStep(); err != nil {
			m.err = err
			return m, nil
		}
		// Check if we've reached the end
		if m.executor.IsComplete() {
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil

	case types.StepTypeQuestion:
		return m.handleQuestionSubmit(step)

	case types.StepTypeAction:
		// Execute action
		m.executing = true
		return m, tea.Batch(
			m.spinner.Tick,
			m.executeAction(step),
		)

	default:
		return m, nil
	}
}

// handleQuestionSubmit handles submitting a question answer
func (m *DynamicSetupModel) handleQuestionSubmit(step *types.SetupStep) (tea.Model, tea.Cmd) {
	if step.Question == nil {
		return m, nil
	}

	var answer interface{}
	var err error

	switch step.Question.Type {
	case types.QuestionTypeText:
		answer = m.textInput.Value()

	case types.QuestionTypeSelect:
		// Get selected option
		if m.cursor >= 0 && m.cursor < len(step.Question.Options) {
			answer = step.Question.Options[m.cursor].Value
		}

	case types.QuestionTypeMultiSelect:
		// Get all selected options
		var selected []string
		for i, opt := range step.Question.Options {
			if m.selected[i] {
				selected = append(selected, opt.Value)
			}
		}
		answer = selected

	case types.QuestionTypeBool:
		// Use cursor position (0 = Yes, 1 = No)
		answer = m.cursor == 0
	}

	// Validate answer
	if err = m.executor.ValidateAnswer(step.Question, answer); err != nil {
		m.validationError = err.Error()
		return m, nil
	}

	// Clear validation error
	m.validationError = ""

	// Store answer
	m.executor.SetAnswer(step.Question.Variable, answer)

	// Advance to next step
	if err := m.executor.NextStep(); err != nil {
		m.err = err
		return m, nil
	}

	// Reset cursor and selection for next question
	m.resetCursor()

	// Check if we've reached the end
	if m.executor.IsComplete() {
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

// executeAction executes an action asynchronously
func (m *DynamicSetupModel) executeAction(step *types.SetupStep) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.actionExecutor.Execute(ctx, step.Action, m.executor.GetState())
		return ActionCompleteMsg{err: err}
	}
}

// resetCursor resets cursor and selection state
func (m *DynamicSetupModel) resetCursor() {
	m.cursor = 0
	m.selected = make(map[int]bool)
	m.textInput.SetValue("")
}

// View renders the current step
func (m *DynamicSetupModel) View() string {
	if m.quitting {
		return "Setup complete!\n"
	}

	step := m.executor.GetCurrentStep()
	if step == nil {
		return "Setup complete!\n"
	}

	var s strings.Builder

	// Render step title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	s.WriteString(titleStyle.Render(step.Title) + "\n\n")

	// Render description if present
	if step.Description != "" {
		descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		s.WriteString(descStyle.Render(step.Description) + "\n\n")
	}

	// Render step content based on type
	switch step.Type {
	case types.StepTypeInfo:
		s.WriteString(m.renderInfoStep(step))
	case types.StepTypeQuestion:
		s.WriteString(m.renderQuestionStep(step))
	case types.StepTypeAction:
		s.WriteString(m.renderActionStep(step))
	}

	// Render error if present
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		s.WriteString("\n\n" + errorStyle.Render("Error: "+m.err.Error()))
	}

	// Render validation error
	if m.validationError != "" {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
		s.WriteString("\n\n" + errorStyle.Render("⚠ "+m.validationError))
	}

	// Render navigation hints
	s.WriteString("\n\n" + m.renderNavigationHints(step))

	// Render progress
	progress := m.executor.GetProgress()
	progressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	s.WriteString("\n" + progressStyle.Render(fmt.Sprintf("Progress: %.0f%%", progress)))

	return s.String()
}

// renderInfoStep renders an info step
func (m *DynamicSetupModel) renderInfoStep(step *types.SetupStep) string {
	if step.Info == nil {
		return ""
	}

	// Interpolate variables in message
	message, err := m.executor.InterpolateString(step.Info.Message)
	if err != nil {
		message = step.Info.Message
	}

	// Style based on info style
	var style lipgloss.Style
	switch step.Info.Style {
	case types.InfoStyleSuccess:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	case types.InfoStyleWarning:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
	case types.InfoStyleError:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	default:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	}

	return style.Render(message)
}

// renderQuestionStep renders a question step
func (m *DynamicSetupModel) renderQuestionStep(step *types.SetupStep) string {
	if step.Question == nil {
		return ""
	}

	var s strings.Builder

	// Load options if needed
	options := step.Question.Options
	if step.Question.OptionsSource != nil {
		loadedOptions, err := m.executor.LoadOptions(step.Question)
		if err != nil {
			return fmt.Sprintf("Error loading options: %v", err)
		}
		options = loadedOptions
	}

	// Render prompt
	promptStyle := lipgloss.NewStyle().Bold(true)
	s.WriteString(promptStyle.Render(step.Question.Prompt) + "\n\n")

	// Render based on question type
	switch step.Question.Type {
	case types.QuestionTypeText:
		s.WriteString(m.textInput.View())

	case types.QuestionTypeSelect:
		for i, opt := range options {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}
			s.WriteString(fmt.Sprintf("%s %s\n", cursor, opt.Label))
			if opt.Description != "" {
				descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
				s.WriteString("  " + descStyle.Render(opt.Description) + "\n")
			}
		}

	case types.QuestionTypeMultiSelect:
		for i, opt := range options {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
			}
			checkbox := "☐"
			if m.selected[i] {
				checkbox = "☑"
			}
			s.WriteString(fmt.Sprintf("%s %s %s\n", cursor, checkbox, opt.Label))
			if opt.Description != "" {
				descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)
				s.WriteString("  " + descStyle.Render(opt.Description) + "\n")
			}
		}

	case types.QuestionTypeBool:
		yesStyle := lipgloss.NewStyle()
		noStyle := lipgloss.NewStyle()
		if m.cursor == 0 {
			yesStyle = yesStyle.Bold(true).Foreground(lipgloss.Color("46"))
		}
		if m.cursor == 1 {
			noStyle = noStyle.Bold(true).Foreground(lipgloss.Color("196"))
		}
		s.WriteString(yesStyle.Render("[ Yes ]") + "  " + noStyle.Render("[ No ]"))
	}

	return s.String()
}

// renderActionStep renders an action step
func (m *DynamicSetupModel) renderActionStep(step *types.SetupStep) string {
	if step.Action == nil {
		return ""
	}

	if m.executing {
		// Show spinner and progress message
		progressMsg := step.Action.ProgressMessage
		if progressMsg == "" {
			progressMsg = "Executing..."
		}
		return m.spinner.View() + " " + progressMsg
	}

	// Show action description
	message := "Press Enter to execute action"
	if step.Description != "" {
		message = step.Description
	}

	return message
}

// renderNavigationHints renders navigation hints
func (m *DynamicSetupModel) renderNavigationHints(step *types.SetupStep) string {
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	var hints []string

	switch step.Type {
	case types.StepTypeQuestion:
		switch step.Question.Type {
		case types.QuestionTypeText:
			hints = append(hints, "Enter: submit")
		case types.QuestionTypeSelect:
			hints = append(hints, "↑↓: navigate", "Enter: select")
		case types.QuestionTypeMultiSelect:
			hints = append(hints, "↑↓: navigate", "Space: toggle", "Enter: continue")
		case types.QuestionTypeBool:
			hints = append(hints, "←→: choose", "Enter: submit")
		}
	case types.StepTypeInfo:
		hints = append(hints, "Enter: continue")
	case types.StepTypeAction:
		if !m.executing {
			hints = append(hints, "Enter: execute")
		}
	}

	if step.Navigation.AllowBack && !m.executing {
		hints = append(hints, "b: back")
	}

	hints = append(hints, "q: quit")

	return hintStyle.Render(strings.Join(hints, " • "))
}

// HasErrors returns true if there were errors during setup
func (m *DynamicSetupModel) HasErrors() bool {
	return m.err != nil
}
