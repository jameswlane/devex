package setup

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// SetupExecutor executes a dynamic setup workflow
type SetupExecutor struct {
	config        *types.SetupConfig
	state         *types.SetupState
	settings      config.CrossPlatformSettings
	repo          types.Repository
	platform      platform.DetectionResult
	conditionEval *ConditionEvaluator
	optionsLoader *OptionsLoader
}

// NewSetupExecutor creates a new setup executor
func NewSetupExecutor(
	cfg *types.SetupConfig,
	settings config.CrossPlatformSettings,
	repo types.Repository,
	detectedPlatform platform.DetectionResult,
) *SetupExecutor {
	state := &types.SetupState{
		CurrentStep:  0,
		Answers:      make(map[string]interface{}),
		SystemInfo:   buildSystemInfo(detectedPlatform),
		InstallState: make(map[string]interface{}),
		Errors:       []string{},
	}

	executor := &SetupExecutor{
		config:   cfg,
		state:    state,
		settings: settings,
		repo:     repo,
		platform: detectedPlatform,
	}

	// Initialize helper components
	executor.conditionEval = NewConditionEvaluator(state, detectedPlatform)
	executor.optionsLoader = NewOptionsLoader(settings, detectedPlatform)

	return executor
}

// GetCurrentStep returns the current step
func (e *SetupExecutor) GetCurrentStep() *types.SetupStep {
	if e.state.CurrentStep >= len(e.config.Steps) {
		return nil
	}
	return &e.config.Steps[e.state.CurrentStep]
}

// GetState returns the current state
func (e *SetupExecutor) GetState() *types.SetupState {
	return e.state
}

// NextStep advances to the next step, evaluating conditions
func (e *SetupExecutor) NextStep() error {
	currentStep := e.GetCurrentStep()
	if currentStep == nil {
		return fmt.Errorf("already at final step")
	}

	// Check for custom next step
	if currentStep.Navigation.NextStep != "" {
		return e.goToStepByID(currentStep.Navigation.NextStep)
	}

	// Check for conditional next steps
	if len(currentStep.Navigation.NextStepIf) > 0 {
		for condition, stepID := range currentStep.Navigation.NextStepIf {
			if e.evaluateSimpleCondition(condition) {
				return e.goToStepByID(stepID)
			}
		}
	}

	// Default: advance to next step in sequence
	for {
		e.state.CurrentStep++
		if e.state.CurrentStep >= len(e.config.Steps) {
			return nil // Reached end
		}

		nextStep := e.GetCurrentStep()

		// Check if step should be shown
		if nextStep.ShowIf != nil {
			shouldShow, err := e.conditionEval.Evaluate(nextStep.ShowIf)
			if err != nil {
				return fmt.Errorf("failed to evaluate condition: %w", err)
			}
			if !shouldShow {
				continue // Skip this step
			}
		}

		break // Found a valid step
	}

	return nil
}

// PrevStep goes back to the previous step
func (e *SetupExecutor) PrevStep() error {
	currentStep := e.GetCurrentStep()
	if currentStep == nil {
		return fmt.Errorf("no current step")
	}

	// Check if going back is allowed
	if !currentStep.Navigation.AllowBack {
		return fmt.Errorf("cannot go back from this step")
	}

	// Check for custom previous step
	if currentStep.Navigation.PrevStep != "" {
		return e.goToStepByID(currentStep.Navigation.PrevStep)
	}

	// Default: go back to previous step in sequence
	for {
		e.state.CurrentStep--
		if e.state.CurrentStep < 0 {
			e.state.CurrentStep = 0
			return fmt.Errorf("already at first step")
		}

		prevStep := e.GetCurrentStep()

		// Check if step should be shown
		if prevStep.ShowIf != nil {
			shouldShow, err := e.conditionEval.Evaluate(prevStep.ShowIf)
			if err != nil {
				return fmt.Errorf("failed to evaluate condition: %w", err)
			}
			if !shouldShow {
				continue // Skip this step
			}
		}

		break // Found a valid step
	}

	return nil
}

// goToStepByID jumps to a specific step by ID
func (e *SetupExecutor) goToStepByID(stepID string) error {
	for i, step := range e.config.Steps {
		if step.ID == stepID {
			e.state.CurrentStep = i
			return nil
		}
	}
	return fmt.Errorf("step not found: %s", stepID)
}

// SetAnswer sets an answer for a question variable
func (e *SetupExecutor) SetAnswer(variable string, value interface{}) {
	e.state.Answers[variable] = value
}

// GetAnswer gets an answer for a question variable
func (e *SetupExecutor) GetAnswer(variable string) (interface{}, bool) {
	value, ok := e.state.Answers[variable]
	return value, ok
}

// LoadOptions loads options for a question
func (e *SetupExecutor) LoadOptions(question *types.Question) ([]types.QuestionOption, error) {
	// If static options are provided, use them
	if len(question.Options) > 0 {
		return e.filterOptions(question.Options), nil
	}

	// If options source is provided, load dynamically
	if question.OptionsSource != nil {
		return e.optionsLoader.Load(question.OptionsSource)
	}

	return []types.QuestionOption{}, nil
}

// filterOptions filters options based on ShowIf conditions
func (e *SetupExecutor) filterOptions(options []types.QuestionOption) []types.QuestionOption {
	filtered := make([]types.QuestionOption, 0, len(options))
	for _, opt := range options {
		if opt.ShowIf != nil {
			shouldShow, err := e.conditionEval.Evaluate(opt.ShowIf)
			if err != nil || !shouldShow {
				continue
			}
		}
		filtered = append(filtered, opt)
	}
	return filtered
}

// InterpolateString interpolates variables in a string using Go templates
func (e *SetupExecutor) InterpolateString(text string) (string, error) {
	// Create template
	tmpl, err := template.New("interpolate").Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(text)
	if err != nil {
		return text, fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare data for template
	data := make(map[string]interface{})
	for k, v := range e.state.Answers {
		data[k] = v
	}
	for k, v := range e.state.SystemInfo {
		data[k] = v
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return text, fmt.Errorf("failed to interpolate: %w", err)
	}

	return buf.String(), nil
}

// ValidateAnswer validates an answer against the question's validation rules
func (e *SetupExecutor) ValidateAnswer(question *types.Question, answer interface{}) error {
	if question.Validation == nil {
		return nil
	}

	val := question.Validation

	// Check required
	if val.Required && (answer == nil || answer == "") {
		if val.Message != "" {
			return fmt.Errorf("%s", val.Message)
		}
		return fmt.Errorf("this field is required")
	}

	// Skip further validation if answer is empty and not required
	if answer == nil || answer == "" {
		return nil
	}

	// String-based validations
	if str, ok := answer.(string); ok {
		// Check min length
		if val.Min != nil && len(str) < *val.Min {
			if val.Message != "" {
				return fmt.Errorf("%s", val.Message)
			}
			return fmt.Errorf("minimum length is %d characters", *val.Min)
		}

		// Check max length
		if val.Max != nil && len(str) > *val.Max {
			if val.Message != "" {
				return fmt.Errorf("%s", val.Message)
			}
			return fmt.Errorf("maximum length is %d characters", *val.Max)
		}

		// Check pattern
		if val.Pattern != "" {
			matched, err := regexp.MatchString(val.Pattern, str)
			if err != nil {
				return fmt.Errorf("invalid regex pattern: %w", err)
			}
			if !matched {
				if val.Message != "" {
					return fmt.Errorf("%s", val.Message)
				}
				return fmt.Errorf("value does not match required pattern")
			}
		}
	}

	// Slice-based validations (for multi-select)
	if slice, ok := answer.([]interface{}); ok {
		// Check min selections
		if val.Min != nil && len(slice) < *val.Min {
			if val.Message != "" {
				return fmt.Errorf("%s", val.Message)
			}
			return fmt.Errorf("select at least %d option(s)", *val.Min)
		}

		// Check max selections
		if val.Max != nil && len(slice) > *val.Max {
			if val.Message != "" {
				return fmt.Errorf("%s", val.Message)
			}
			return fmt.Errorf("select at most %d option(s)", *val.Max)
		}
	}

	return nil
}

// ExecuteAction executes a step action
func (e *SetupExecutor) ExecuteAction(ctx context.Context, action *types.StepAction) error {
	// This will be implemented based on action type
	// For now, return a placeholder
	return fmt.Errorf("action execution not yet implemented: %s", action.Type)
}

// evaluateSimpleCondition evaluates a simple condition string (for NextStepIf)
func (e *SetupExecutor) evaluateSimpleCondition(conditionStr string) bool {
	// Simple format: "variable=value" or "variable"
	parts := strings.SplitN(conditionStr, "=", 2)
	variable := strings.TrimSpace(parts[0])

	value, exists := e.state.Answers[variable]
	if !exists {
		return false
	}

	// If no value specified, just check existence
	if len(parts) == 1 {
		return value != nil && value != ""
	}

	// Check if value matches
	expectedValue := strings.TrimSpace(parts[1])
	return fmt.Sprintf("%v", value) == expectedValue
}

// buildSystemInfo creates the system info map from platform detection
func buildSystemInfo(platform platform.DetectionResult) map[string]interface{} {
	hasDesktop := platform.DesktopEnv != "none" &&
		platform.DesktopEnv != "unknown" &&
		platform.DesktopEnv != ""

	return map[string]interface{}{
		"os":           platform.OS,
		"distribution": platform.Distribution,
		"desktop":      platform.DesktopEnv,
		"architecture": platform.Architecture,
		"has_desktop":  hasDesktop,
	}
}

// IsComplete returns true if all steps have been completed
func (e *SetupExecutor) IsComplete() bool {
	return e.state.CurrentStep >= len(e.config.Steps)
}

// GetProgress returns the current progress as a percentage
func (e *SetupExecutor) GetProgress() float64 {
	if len(e.config.Steps) == 0 {
		return 100.0
	}
	return float64(e.state.CurrentStep) / float64(len(e.config.Steps)) * 100.0
}
