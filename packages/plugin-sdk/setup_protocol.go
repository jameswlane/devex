package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// PlatformInfo represents platform detection results passed to plugins
type PlatformInfo struct {
	OS           string `json:"os"`
	Distribution string `json:"distribution"`
	Desktop      string `json:"desktop"`
	Architecture string `json:"arch"`
	HasDesktop   bool   `json:"has_desktop"`
}

// PluginSetupInput represents standardized input for setup plugins
// This is passed via stdin as JSON when a plugin is executed during setup
type PluginSetupInput struct {
	// Command to execute (e.g., "install", "configure", "setup")
	Command string `json:"command"`

	// Config contains the parsed YAML/JSON configuration for this plugin
	// For example, for a language plugin, this might contain version, tools, etc.
	Config map[string]interface{} `json:"config,omitempty"`

	// Parameters contains user answers from the setup workflow
	// Keys are variable names from the setup config, values are user selections
	Parameters map[string]interface{} `json:"parameters"`

	// Environment contains detected platform information
	Environment PlatformInfo `json:"environment"`

	// ConfigPath is the path to the original config file (if applicable)
	// This allows plugins to read structured config directly
	ConfigPath string `json:"config_path,omitempty"`
}

// PluginSetupOutput represents standardized output from setup plugins
// Plugins should write this to stdout as JSON to report progress and results
type PluginSetupOutput struct {
	// Status indicates the current state: "in_progress", "success", "error"
	Status string `json:"status"`

	// Progress is a percentage (0-100) for long-running operations
	Progress int `json:"progress,omitempty"`

	// Message is a human-readable status message
	Message string `json:"message"`

	// Data contains arbitrary result data from the plugin
	Data map[string]interface{} `json:"data,omitempty"`

	// Error contains error details if Status is "error"
	Error string `json:"error,omitempty"`
}

// SetupStatus represents the possible status values for setup operations
type SetupStatus string

const (
	SetupStatusInProgress SetupStatus = "in_progress"
	SetupStatusSuccess    SetupStatus = "success"
	SetupStatusError      SetupStatus = "error"
)

// ReadSetupInput reads PluginSetupInput from stdin
// Plugins should call this at the start of their setup command handler
func ReadSetupInput() (*PluginSetupInput, error) {
	return ReadSetupInputFromReader(os.Stdin)
}

// ReadSetupInputFromReader reads PluginSetupInput from an io.Reader
// This is useful for testing or reading from alternative sources
func ReadSetupInputFromReader(r io.Reader) (*PluginSetupInput, error) {
	var input PluginSetupInput
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("failed to decode setup input: %w", err)
	}
	return &input, nil
}

// WriteSetupOutput writes PluginSetupOutput to stdout
// Plugins should call this to send their final result
func WriteSetupOutput(output *PluginSetupOutput) error {
	return WriteSetupOutputToWriter(os.Stdout, output)
}

// WriteSetupOutputToWriter writes PluginSetupOutput to an io.Writer
// This is useful for testing or writing to alternative destinations
func WriteSetupOutputToWriter(w io.Writer, output *PluginSetupOutput) error {
	encoder := json.NewEncoder(w)
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode setup output: %w", err)
	}
	return nil
}

// SendProgress sends a progress update to stdout
// Plugins should call this during long-running operations to report progress
func SendProgress(progress int, message string) error {
	output := &PluginSetupOutput{
		Status:   string(SetupStatusInProgress),
		Progress: progress,
		Message:  message,
	}
	return WriteSetupOutput(output)
}

// SendSuccess sends a success message to stdout
// Plugins should call this when the operation completes successfully
func SendSuccess(message string, data map[string]interface{}) error {
	output := &PluginSetupOutput{
		Status:   string(SetupStatusSuccess),
		Progress: 100,
		Message:  message,
		Data:     data,
	}
	return WriteSetupOutput(output)
}

// SendError sends an error message to stdout
// Plugins should call this when an error occurs
func SendError(message string, err error) error {
	errorMsg := message
	if err != nil {
		errorMsg = fmt.Sprintf("%s: %v", message, err)
	}

	output := &PluginSetupOutput{
		Status:  string(SetupStatusError),
		Message: message,
		Error:   errorMsg,
	}
	return WriteSetupOutput(output)
}

// GetParameterString safely extracts a string parameter from setup input
func (input *PluginSetupInput) GetParameterString(key string) (string, bool) {
	val, exists := input.Parameters[key]
	if !exists {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetParameterStringSlice safely extracts a string slice parameter from setup input
func (input *PluginSetupInput) GetParameterStringSlice(key string) ([]string, bool) {
	val, exists := input.Parameters[key]
	if !exists {
		return nil, false
	}

	// Handle []interface{} from JSON unmarshaling
	if slice, ok := val.([]interface{}); ok {
		result := make([]string, 0, len(slice))
		for _, item := range slice {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, len(result) > 0
	}

	// Handle direct []string (from custom marshaling)
	if slice, ok := val.([]string); ok {
		return slice, len(slice) > 0
	}

	return nil, false
}

// GetParameterBool safely extracts a boolean parameter from setup input
func (input *PluginSetupInput) GetParameterBool(key string) (bool, bool) {
	val, exists := input.Parameters[key]
	if !exists {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// GetParameterInt safely extracts an integer parameter from setup input
func (input *PluginSetupInput) GetParameterInt(key string) (int, bool) {
	val, exists := input.Parameters[key]
	if !exists {
		return 0, false
	}

	// Handle float64 from JSON unmarshaling
	if f, ok := val.(float64); ok {
		return int(f), true
	}

	// Handle direct int
	if i, ok := val.(int); ok {
		return i, true
	}

	return 0, false
}

// GetConfigString safely extracts a string value from config
func (input *PluginSetupInput) GetConfigString(key string) (string, bool) {
	val, exists := input.Config[key]
	if !exists {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetConfigStringSlice safely extracts a string slice from config
func (input *PluginSetupInput) GetConfigStringSlice(key string) ([]string, bool) {
	val, exists := input.Config[key]
	if !exists {
		return nil, false
	}

	// Handle []interface{} from JSON unmarshaling
	if slice, ok := val.([]interface{}); ok {
		result := make([]string, 0, len(slice))
		for _, item := range slice {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, len(result) > 0
	}

	// Handle direct []string
	if slice, ok := val.([]string); ok {
		return slice, len(slice) > 0
	}

	return nil, false
}
