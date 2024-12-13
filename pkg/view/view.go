package view

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/layout"
	"github.com/jameswlane/devex/pkg/logger"
	"github.com/jameswlane/devex/pkg/steps"
)

// ViewModel holds the layout structure
type ViewModel struct {
	layout     layout.LayoutModel
	logs       []string
	steps      []string
	maxLogSize int
}

// NewViewModel initializes the view model
func NewViewModel(systemInfo string, width int, height int) ViewModel {
	stepsPaneWidth := width / 5
	logsPaneWidth := width - stepsPaneWidth - 5
	return ViewModel{
		layout:     layout.NewLayoutModel(systemInfo, stepsPaneWidth, logsPaneWidth, height),
		maxLogSize: height - 5, // Limit logs to fit within the height of the terminal
	}
}

// ExecuteSteps runs through all steps, updating the layout dynamically
func (v *ViewModel) ExecuteSteps(stepsList []steps.Step, dryRun bool, db *datastore.DB, logger *logger.Logger) {
	for i := range stepsList {
		step := &stepsList[i]
		v.addLog(fmt.Sprintf("Starting step: %s", step.Name))

		// Execute the step (this now includes the install process)
		steps.ExecuteSteps(stepsList, dryRun, db, logger)

		// Update the steps pane in the view after each step
		v.updateStepsPane(stepsList)

		// Log the step's completion status
		v.addLog(fmt.Sprintf("Step %s: %s", step.Name, step.Status))

		// Refresh the view with updated steps and logs
		v.updateLogsPane()
	}
}

// Add log entry and handle overflow
func (v *ViewModel) addLog(logEntry string) {
	v.logs = append(v.logs, logEntry)
	if len(v.logs) > v.maxLogSize {
		v.logs = v.logs[1:] // Remove oldest entry if overflowing
	}
}

// Render the complete layout
func (v ViewModel) Render() string {
	return v.layout.RenderView(v.steps, v.logs, 0.5) // Placeholder for progress
}

// updateStepsPane updates the steps pane in the layout
func (v *ViewModel) updateStepsPane(stepsList []steps.Step) {
	v.steps = []string{}
	for _, step := range stepsList {
		v.steps = append(v.steps, fmt.Sprintf("%s: %s", step.Name, step.Status))
	}
}

// updateLogsPane refreshes the logs pane in the layout
func (v *ViewModel) updateLogsPane() {
	v.layout.UpdateLogsPane(v.logs)
}
