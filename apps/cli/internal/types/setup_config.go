package types

import "time"

// SetupConfig represents the complete dynamic setup workflow configuration
type SetupConfig struct {
	// Metadata about the setup configuration
	Metadata SetupMetadata `yaml:"metadata"`

	// Timeouts for various operations
	Timeouts SetupTimeouts `yaml:"timeouts"`

	// Steps define the workflow screens/questions
	Steps []SetupStep `yaml:"steps"`

	// Actions map action IDs to their implementations
	Actions map[string]SetupAction `yaml:"actions,omitempty"`
}

// SetupMetadata provides information about the setup configuration
type SetupMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	Author      string `yaml:"author,omitempty"`
}

// SetupTimeouts configures timeouts for various operations
type SetupTimeouts struct {
	PluginInstall    time.Duration `yaml:"plugin_install"`
	PluginVerify     time.Duration `yaml:"plugin_verify"`
	PluginDownload   time.Duration `yaml:"plugin_download"`
	NetworkOperation time.Duration `yaml:"network_operation"`
}

// SetupStep represents a single screen/step in the setup workflow
type SetupStep struct {
	// Unique identifier for this step
	ID string `yaml:"id"`

	// Display name shown to users
	Title string `yaml:"title"`

	// Optional description/help text
	Description string `yaml:"description,omitempty"`

	// Type of step (question, info, action)
	Type StepType `yaml:"type"`

	// Question configuration (if type is "question")
	Question *Question `yaml:"question,omitempty"`

	// Information to display (if type is "info")
	Info *InfoContent `yaml:"info,omitempty"`

	// Action to execute (if type is "action")
	Action *StepAction `yaml:"action,omitempty"`

	// Conditions for showing this step
	ShowIf *Condition `yaml:"show_if,omitempty"`

	// Whether this step can be skipped
	Skippable bool `yaml:"skippable"`

	// Navigation configuration
	Navigation StepNavigation `yaml:"navigation,omitempty"`
}

// StepType defines the type of setup step
type StepType string

const (
	StepTypeQuestion StepType = "question"
	StepTypeInfo     StepType = "info"
	StepTypeAction   StepType = "action"
)

// Question represents a user input question
type Question struct {
	// Type of question (text, select, multiselect)
	Type QuestionType `yaml:"type"`

	// Variable name to store the answer
	Variable string `yaml:"variable"`

	// Prompt text shown to user
	Prompt string `yaml:"prompt"`

	// Placeholder text for text inputs
	Placeholder string `yaml:"placeholder,omitempty"`

	// Default value
	Default interface{} `yaml:"default,omitempty"`

	// Options for select/multiselect questions
	Options []QuestionOption `yaml:"options,omitempty"`

	// Options source (static, config, system)
	OptionsSource *OptionsSource `yaml:"options_source,omitempty"`

	// Validation rules
	Validation *Validation `yaml:"validation,omitempty"`

	// Whether multiple selections are allowed (for select type)
	Multiple bool `yaml:"multiple,omitempty"`
}

// QuestionType defines the type of question
type QuestionType string

const (
	QuestionTypeText        QuestionType = "text"
	QuestionTypeSelect      QuestionType = "select"
	QuestionTypeMultiSelect QuestionType = "multiselect"
	QuestionTypeBool        QuestionType = "bool"
)

// QuestionOption represents a selectable option
type QuestionOption struct {
	// Display name shown to user
	Label string `yaml:"label"`

	// Value stored when selected
	Value string `yaml:"value"`

	// Description/help text for this option
	Description string `yaml:"description,omitempty"`

	// Whether this option is selected by default
	Default bool `yaml:"default,omitempty"`

	// Condition for showing this option
	ShowIf *Condition `yaml:"show_if,omitempty"`
}

// OptionsSource defines how to load options dynamically
type OptionsSource struct {
	// Type of source (config, system, plugin)
	Type SourceType `yaml:"type"`

	// Path to config file or key (for config source)
	Path string `yaml:"path,omitempty"`

	// Key within the config (for config source)
	Key string `yaml:"key,omitempty"`

	// System detection type (for system source)
	SystemType string `yaml:"system_type,omitempty"`

	// Transform to apply to loaded options
	Transform string `yaml:"transform,omitempty"`
}

// SourceType defines where options come from
type SourceType string

const (
	SourceTypeStatic SourceType = "static"
	SourceTypeConfig SourceType = "config"
	SourceTypeSystem SourceType = "system"
	SourceTypePlugin SourceType = "plugin"
)

// Validation defines validation rules for answers
type Validation struct {
	// Required field
	Required bool `yaml:"required"`

	// Minimum value/length
	Min *int `yaml:"min,omitempty"`

	// Maximum value/length
	Max *int `yaml:"max,omitempty"`

	// Regex pattern to match
	Pattern string `yaml:"pattern,omitempty"`

	// Custom validation message
	Message string `yaml:"message,omitempty"`

	// Custom validation function name
	Function string `yaml:"function,omitempty"`
}

// InfoContent represents informational content to display
type InfoContent struct {
	// Message to display
	Message string `yaml:"message"`

	// Style/type of info (info, warning, error, success)
	Style InfoStyle `yaml:"style,omitempty"`

	// Variables to interpolate into message
	Variables map[string]string `yaml:"variables,omitempty"`
}

// InfoStyle defines the style of informational message
type InfoStyle string

const (
	InfoStyleInfo    InfoStyle = "info"
	InfoStyleWarning InfoStyle = "warning"
	InfoStyleError   InfoStyle = "error"
	InfoStyleSuccess InfoStyle = "success"
)

// StepAction represents an action to execute
type StepAction struct {
	// Action type (install, configure, execute)
	Type ActionType `yaml:"type"`

	// Action-specific parameters
	Params map[string]interface{} `yaml:"params,omitempty"`

	// Progress message shown during execution
	ProgressMessage string `yaml:"progress_message,omitempty"`

	// Success message after completion
	SuccessMessage string `yaml:"success_message,omitempty"`

	// Error handling behavior
	OnError ErrorBehavior `yaml:"on_error,omitempty"`
}

// ActionType defines the type of action
type ActionType string

const (
	ActionTypeInstall   ActionType = "install"
	ActionTypeConfigure ActionType = "configure"
	ActionTypeExecute   ActionType = "execute"
	ActionTypePlugin    ActionType = "plugin"
)

// ErrorBehavior defines how to handle errors
type ErrorBehavior string

const (
	ErrorBehaviorStop     ErrorBehavior = "stop"
	ErrorBehaviorContinue ErrorBehavior = "continue"
	ErrorBehaviorRetry    ErrorBehavior = "retry"
	ErrorBehaviorSkip     ErrorBehavior = "skip"
)

// Condition represents a conditional expression
type Condition struct {
	// Variable to check
	Variable string `yaml:"variable,omitempty"`

	// Operator (equals, not_equals, contains, exists, etc.)
	Operator ConditionOperator `yaml:"operator"`

	// Value to compare against
	Value interface{} `yaml:"value,omitempty"`

	// System condition (platform, desktop, etc.)
	System *SystemCondition `yaml:"system,omitempty"`

	// Logical operators for complex conditions
	And []*Condition `yaml:"and,omitempty"`
	Or  []*Condition `yaml:"or,omitempty"`
	Not *Condition   `yaml:"not,omitempty"`
}

// ConditionOperator defines comparison operators
type ConditionOperator string

const (
	OperatorEquals      ConditionOperator = "equals"
	OperatorNotEquals   ConditionOperator = "not_equals"
	OperatorContains    ConditionOperator = "contains"
	OperatorNotContains ConditionOperator = "not_contains"
	OperatorExists      ConditionOperator = "exists"
	OperatorNotExists   ConditionOperator = "not_exists"
	OperatorGreaterThan ConditionOperator = "greater_than"
	OperatorLessThan    ConditionOperator = "less_than"
	OperatorMatches     ConditionOperator = "matches" // Regex match
	OperatorNotMatches  ConditionOperator = "not_matches"
)

// SystemCondition represents a system-level condition
type SystemCondition struct {
	// OS condition (linux, darwin, windows)
	OS string `yaml:"os,omitempty"`

	// Distribution condition (ubuntu, fedora, etc.)
	Distribution string `yaml:"distribution,omitempty"`

	// Desktop environment condition (gnome, kde, etc.)
	Desktop string `yaml:"desktop,omitempty"`

	// Architecture condition (amd64, arm64, etc.)
	Architecture string `yaml:"architecture,omitempty"`

	// Has desktop environment
	HasDesktop *bool `yaml:"has_desktop,omitempty"`
}

// StepNavigation controls navigation behavior
type StepNavigation struct {
	// Next step ID (overrides default sequential flow)
	NextStep string `yaml:"next_step,omitempty"`

	// Previous step ID (overrides default sequential flow)
	PrevStep string `yaml:"prev_step,omitempty"`

	// Conditional next steps
	NextStepIf map[string]string `yaml:"next_step_if,omitempty"`

	// Whether user can go back from this step
	AllowBack bool `yaml:"allow_back"`

	// Auto-advance after completion
	AutoAdvance bool `yaml:"auto_advance,omitempty"`
}

// SetupAction represents a reusable action definition
type SetupAction struct {
	// Action name
	Name string `yaml:"name"`

	// Action description
	Description string `yaml:"description,omitempty"`

	// Action type
	Type ActionType `yaml:"type"`

	// Action parameters
	Params map[string]interface{} `yaml:"params,omitempty"`
}

// SetupState holds the runtime state of the setup process
type SetupState struct {
	// Current step index
	CurrentStep int

	// Answers collected from questions
	Answers map[string]interface{}

	// Detected system information
	SystemInfo map[string]interface{}

	// Installation state
	InstallState map[string]interface{}

	// Errors encountered
	Errors []string
}
