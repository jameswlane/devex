package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// SetupConfigLoader handles loading and validation of setup configurations
type SetupConfigLoader struct {
	configPath string
}

// NewSetupConfigLoader creates a new setup config loader
func NewSetupConfigLoader(configPath string) *SetupConfigLoader {
	return &SetupConfigLoader{
		configPath: configPath,
	}
}

// Load loads the setup configuration from the specified path or defaults
func (l *SetupConfigLoader) Load() (*types.SetupConfig, error) {
	var data []byte
	var err error
	var isJSON bool

	// Load configuration based on path
	if l.configPath != "" {
		// Check if it's a URL
		if isURL(l.configPath) {
			// Load from remote URL
			data, isJSON, err = l.loadFromURL(l.configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load remote config: %w", err)
			}
		} else {
			// Load from custom local path
			data, err = l.loadFromFile(l.configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load custom config: %w", err)
			}
			// Detect JSON from file extension
			isJSON = strings.HasSuffix(strings.ToLower(l.configPath), ".json")
		}
	} else {
		// Load default embedded configuration
		data, err = l.loadDefaultConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load default config: %w", err)
		}
	}

	// Parse configuration based on format
	var config types.SetupConfig
	if isJSON {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse setup config (JSON): %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse setup config (YAML): %w", err)
		}
	}

	// Validate configuration
	if err := l.Validate(&config); err != nil {
		return nil, fmt.Errorf("invalid setup config: %w", err)
	}

	// Set defaults for any missing values
	l.setDefaults(&config)

	return &config, nil
}

// loadFromFile loads configuration from a file path
func (l *SetupConfigLoader) loadFromFile(path string) ([]byte, error) {
	// Expand home directory if needed
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return data, nil
}

// loadDefaultConfig loads the default configuration from standard locations
func (l *SetupConfigLoader) loadDefaultConfig() ([]byte, error) {
	// Try multiple locations for the default config
	searchPaths := []string{
		"config/setup.yaml",                        // Relative to working directory
		"./config/setup.yaml",                      // Explicit relative
		"/usr/local/share/devex/config/setup.yaml", // System installation
		"/usr/share/devex/config/setup.yaml",       // Alternative system location
	}

	// Add user home directory path
	if home, err := os.UserHomeDir(); err == nil {
		searchPaths = append(searchPaths, filepath.Join(home, ".devex", "config", "setup.yaml"))
		searchPaths = append(searchPaths, filepath.Join(home, ".local", "share", "devex", "config", "setup.yaml"))
	}

	// Try each path
	for _, path := range searchPaths {
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}

	// If no default config found, return a minimal fallback
	return []byte(getMinimalFallbackConfig()), nil
}

// getMinimalFallbackConfig returns a minimal working config when no default is found
func getMinimalFallbackConfig() string {
	return `metadata:
  name: "DevEx Fallback Setup"
  description: "Minimal fallback configuration"
  version: "1.0.0"

timeouts:
  plugin_install: 300s
  plugin_verify: 30s
  plugin_download: 120s
  network_operation: 30s

steps:
  - id: welcome
    title: "Welcome to DevEx"
    type: info
    info:
      message: |
        Welcome to DevEx!

        Note: Default configuration not found. Using minimal fallback.
        To customize, use: devex setup --config=/path/to/config.yaml

        Press Enter to continue.
      style: warning
    navigation:
      allow_back: false

  - id: complete
    title: "Setup"
    type: info
    info:
      message: |
        Please provide a custom configuration file using:
        devex setup --config=/path/to/config.yaml

        Or reinstall DevEx to restore default configuration.
      style: info
    navigation:
      allow_back: false
`
}

// Validate validates the setup configuration
func (l *SetupConfigLoader) Validate(config *types.SetupConfig) error {
	// Validate metadata
	if config.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	if config.Metadata.Version == "" {
		return fmt.Errorf("metadata.version is required")
	}

	// Validate steps
	if len(config.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}

	// Validate each step
	stepIDs := make(map[string]bool)
	for i, step := range config.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: id is required", i)
		}
		if stepIDs[step.ID] {
			return fmt.Errorf("step %d: duplicate step ID '%s'", i, step.ID)
		}
		stepIDs[step.ID] = true

		if step.Title == "" {
			return fmt.Errorf("step %s: title is required", step.ID)
		}

		// Validate step type
		if err := l.validateStepType(&step); err != nil {
			return fmt.Errorf("step %s: %w", step.ID, err)
		}
	}

	// Validate step navigation references
	for _, step := range config.Steps {
		if step.Navigation.NextStep != "" && !stepIDs[step.Navigation.NextStep] {
			return fmt.Errorf("step %s: next_step '%s' does not exist", step.ID, step.Navigation.NextStep)
		}
		if step.Navigation.PrevStep != "" && !stepIDs[step.Navigation.PrevStep] {
			return fmt.Errorf("step %s: prev_step '%s' does not exist", step.ID, step.Navigation.PrevStep)
		}
	}

	return nil
}

// validateStepType validates step-specific configuration
func (l *SetupConfigLoader) validateStepType(step *types.SetupStep) error {
	switch step.Type {
	case types.StepTypeQuestion:
		if step.Question == nil {
			return fmt.Errorf("question configuration is required for question type")
		}
		return l.validateQuestion(step.Question)
	case types.StepTypeInfo:
		if step.Info == nil {
			return fmt.Errorf("info configuration is required for info type")
		}
		return l.validateInfo(step.Info)
	case types.StepTypeAction:
		if step.Action == nil {
			return fmt.Errorf("action configuration is required for action type")
		}
		return l.validateAction(step.Action)
	default:
		return fmt.Errorf("invalid step type: %s", step.Type)
	}
}

// validateQuestion validates question configuration
func (l *SetupConfigLoader) validateQuestion(q *types.Question) error {
	if q.Variable == "" {
		return fmt.Errorf("question.variable is required")
	}
	if q.Prompt == "" {
		return fmt.Errorf("question.prompt is required")
	}

	// Validate question type
	switch q.Type {
	case types.QuestionTypeText:
		// Text questions don't need options
	case types.QuestionTypeSelect, types.QuestionTypeMultiSelect:
		// Select questions need either options or options_source
		if len(q.Options) == 0 && q.OptionsSource == nil {
			return fmt.Errorf("select/multiselect questions require options or options_source")
		}
	case types.QuestionTypeBool:
		// Bool questions don't need options
	default:
		return fmt.Errorf("invalid question type: %s", q.Type)
	}

	return nil
}

// validateInfo validates info configuration
func (l *SetupConfigLoader) validateInfo(info *types.InfoContent) error {
	if info.Message == "" {
		return fmt.Errorf("info.message is required")
	}
	return nil
}

// validateAction validates action configuration
func (l *SetupConfigLoader) validateAction(action *types.StepAction) error {
	// Validate action type
	switch action.Type {
	case types.ActionTypeInstall, types.ActionTypeConfigure, types.ActionTypeExecute, types.ActionTypePlugin:
		// Valid action types
	default:
		return fmt.Errorf("invalid action type: %s", action.Type)
	}

	return nil
}

// setDefaults sets default values for missing configuration
func (l *SetupConfigLoader) setDefaults(config *types.SetupConfig) {
	// Set default timeouts if not specified
	if config.Timeouts.PluginInstall == 0 {
		config.Timeouts.PluginInstall = 5 * time.Minute
	}
	if config.Timeouts.PluginVerify == 0 {
		config.Timeouts.PluginVerify = 30 * time.Second
	}
	if config.Timeouts.PluginDownload == 0 {
		config.Timeouts.PluginDownload = 2 * time.Minute
	}
	if config.Timeouts.NetworkOperation == 0 {
		config.Timeouts.NetworkOperation = 30 * time.Second
	}

	// Set default navigation for steps
	// Note: For sequential steps without explicit next/prev, the executor handles flow automatically
	// This section is reserved for future navigation default settings if needed
	_ = config.Steps // Use config.Steps to prevent unused variable warning

	// Set default info style
	for i := range config.Steps {
		step := &config.Steps[i]
		if step.Type == types.StepTypeInfo && step.Info != nil {
			if step.Info.Style == "" {
				step.Info.Style = types.InfoStyleInfo
			}
		}
	}

	// Set default error behavior for actions
	for i := range config.Steps {
		step := &config.Steps[i]
		if step.Type == types.StepTypeAction && step.Action != nil {
			if step.Action.OnError == "" {
				step.Action.OnError = types.ErrorBehaviorStop
			}
		}
	}
}

// LoadSetupConfig is a convenience function to load setup configuration
func LoadSetupConfig(configPath string) (*types.SetupConfig, error) {
	loader := NewSetupConfigLoader(configPath)
	return loader.Load()
}

// ValidateSetupConfig validates a setup configuration without loading
func ValidateSetupConfig(config *types.SetupConfig) error {
	loader := NewSetupConfigLoader("")
	return loader.Validate(config)
}

// isURL checks if a string is a URL
func isURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// loadFromURL loads configuration from a remote URL with caching
func (l *SetupConfigLoader) loadFromURL(url string) ([]byte, bool, error) {
	// Check cache first
	if data, isJSON, valid := l.loadFromCache(url); valid {
		return data, isJSON, nil
	}

	// Download from URL
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, false, fmt.Errorf("failed to download config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("failed to download config: HTTP %d", resp.StatusCode)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read config: %w", err)
	}

	// Detect format from content-type header or URL
	contentType := resp.Header.Get("Content-Type")
	var isJSON bool
	switch {
	case strings.Contains(contentType, "application/json"):
		isJSON = true
	case strings.Contains(contentType, "text/yaml"), strings.Contains(contentType, "application/yaml"):
		isJSON = false
	default:
		// Fallback to URL extension
		isJSON = strings.HasSuffix(strings.ToLower(url), ".json")
	}

	// Save to cache (ignore errors - cache failure shouldn't block usage)
	_ = l.saveToCache(url, data, isJSON)

	return data, isJSON, nil
}

// getCacheDir returns the cache directory for setup configs
func (l *SetupConfigLoader) getCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".devex", "setup-cache"), nil
}

// getCachePath returns the cache file path for a URL
func (l *SetupConfigLoader) getCachePath(url string) (string, error) {
	cacheDir, err := l.getCacheDir()
	if err != nil {
		return "", err
	}

	// Create a hash of the URL for the cache filename
	hash := sha256.Sum256([]byte(url))
	hashStr := hex.EncodeToString(hash[:])

	return filepath.Join(cacheDir, hashStr), nil
}

// loadFromCache loads configuration from cache if valid
func (l *SetupConfigLoader) loadFromCache(url string) ([]byte, bool, bool) {
	cachePath, err := l.getCachePath(url)
	if err != nil {
		return nil, false, false
	}

	// Check if cache file exists
	info, err := os.Stat(cachePath)
	if err != nil {
		return nil, false, false
	}

	// Check if cache is still valid (24 hours)
	cacheDuration := 24 * time.Hour
	if time.Since(info.ModTime()) > cacheDuration {
		return nil, false, false
	}

	// Read cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false, false
	}

	// Read metadata file to determine format
	metaPath := cachePath + ".meta"
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		// Default to YAML if no metadata
		return data, false, true
	}

	var meta struct {
		IsJSON bool `json:"is_json"`
	}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return data, false, true
	}

	return data, meta.IsJSON, true
}

// saveToCache saves configuration to cache
func (l *SetupConfigLoader) saveToCache(url string, data []byte, isJSON bool) error {
	cacheDir, err := l.getCacheDir()
	if err != nil {
		return err
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cachePath, err := l.getCachePath(url)
	if err != nil {
		return err
	}

	// Write config data
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	// Write metadata
	meta := struct {
		IsJSON bool      `json:"is_json"`
		URL    string    `json:"url"`
		Cached time.Time `json:"cached"`
	}{
		IsJSON: isJSON,
		URL:    url,
		Cached: time.Now(),
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metaPath := cachePath + ".meta"
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}
