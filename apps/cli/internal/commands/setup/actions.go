package setup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"gopkg.in/yaml.v3"
)

// ActionExecutor executes actions defined in setup steps
type ActionExecutor struct {
	pluginBootstrap *bootstrap.PluginBootstrap
	settings        config.CrossPlatformSettings
	platform        platform.DetectionResult
}

// NewActionExecutor creates a new action executor
func NewActionExecutor(
	pluginBootstrap *bootstrap.PluginBootstrap,
	settings config.CrossPlatformSettings,
	detectedPlatform platform.DetectionResult,
) *ActionExecutor {
	return &ActionExecutor{
		pluginBootstrap: pluginBootstrap,
		settings:        settings,
		platform:        detectedPlatform,
	}
}

// Execute executes an action with the current state
func (ae *ActionExecutor) Execute(ctx context.Context, action *types.StepAction, state *types.SetupState) error {
	switch action.Type {
	case types.ActionTypeInstall:
		return ae.executeInstall(ctx, action, state)
	case types.ActionTypeConfigure:
		return ae.executeConfigure(ctx, action, state)
	case types.ActionTypePlugin:
		return ae.executePlugin(ctx, action, state)
	case types.ActionTypeExecute:
		return ae.executeCommand(ctx, action, state)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// executeInstall executes an installation action
func (ae *ActionExecutor) executeInstall(ctx context.Context, action *types.StepAction, state *types.SetupState) error {
	// Extract parameters from action
	params := action.Params
	if params == nil {
		return fmt.Errorf("install action requires params")
	}

	// Get items to install from parameters
	// This could be from user selections or static config
	items, ok := params["install_languages"]
	if ok {
		return ae.installLanguages(ctx, items, state)
	}

	items, ok = params["install_databases"]
	if ok {
		return ae.installDatabases(ctx, items, state)
	}

	items, ok = params["install_desktop_apps"]
	if ok {
		return ae.installDesktopApps(ctx, items, state)
	}

	return fmt.Errorf("install action requires install_languages, install_databases, or install_desktop_apps parameter")
}

// executeConfigure executes a configuration action
func (ae *ActionExecutor) executeConfigure(ctx context.Context, action *types.StepAction, state *types.SetupState) error {
	params := action.Params
	if params == nil {
		return fmt.Errorf("configure action requires params")
	}

	// Handle shell configuration
	if shell, ok := params["configure_shell"]; ok {
		return ae.configureShell(ctx, shell, state)
	}

	// Handle git configuration
	if gitConfig, ok := params["configure_git"]; ok {
		return ae.configureGit(ctx, gitConfig, state)
	}

	return fmt.Errorf("configure action requires configure_shell or configure_git parameter")
}

// executePlugin executes a plugin action directly
func (ae *ActionExecutor) executePlugin(ctx context.Context, action *types.StepAction, state *types.SetupState) error {
	params := action.Params
	if params == nil {
		return fmt.Errorf("plugin action requires params")
	}

	// Get plugin name
	pluginName, ok := params["plugin_name"].(string)
	if !ok {
		return fmt.Errorf("plugin action requires plugin_name parameter")
	}

	// Get command (defaults to "setup")
	command := "setup"
	if cmd, ok := params["command"].(string); ok {
		command = cmd
	}

	// Get config mapping
	var configData map[string]interface{}
	if configMap, ok := params["config_mapping"]; ok {
		configData = ae.resolveConfigMapping(configMap, state)
	}

	// Execute plugin with setup protocol
	return ae.executePluginWithSetup(ctx, pluginName, command, configData, state)
}

// executeCommand executes a custom shell command
func (ae *ActionExecutor) executeCommand(ctx context.Context, action *types.StepAction, state *types.SetupState) error {
	params := action.Params
	if params == nil {
		return fmt.Errorf("execute action requires params")
	}

	cmdStr, ok := params["command"].(string)
	if !ok {
		return fmt.Errorf("execute action requires command parameter")
	}

	// Execute shell command
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// installLanguages installs programming languages via mise plugin
func (ae *ActionExecutor) installLanguages(ctx context.Context, items interface{}, state *types.SetupState) error {
	// Convert items to string slice
	languages, err := ae.toStringSlice(items)
	if err != nil {
		return fmt.Errorf("invalid install_languages format: %w", err)
	}

	// For each language, load its config and install via mise
	for _, lang := range languages {
		configPath := filepath.Join("config/environments/programming-languages", lang+".yaml")

		// Read config file
		configData, err := ae.loadConfigFile(configPath)
		if err != nil {
			log.Warn("Failed to load config for language", "language", lang, "error", err)
			continue
		}

		// Execute mise plugin with config
		if err := ae.executePluginWithSetup(ctx, "mise", "install", configData, state); err != nil {
			return fmt.Errorf("failed to install %s: %w", lang, err)
		}
	}

	return nil
}

// installDatabases installs databases via docker plugin
func (ae *ActionExecutor) installDatabases(ctx context.Context, items interface{}, state *types.SetupState) error {
	databases, err := ae.toStringSlice(items)
	if err != nil {
		return fmt.Errorf("invalid install_databases format: %w", err)
	}

	// For each database, load its config and install via docker
	for _, db := range databases {
		configPath := filepath.Join("config/applications/databases", db+".yaml")

		configData, err := ae.loadConfigFile(configPath)
		if err != nil {
			log.Warn("Failed to load config for database", "database", db, "error", err)
			continue
		}

		// Execute docker plugin with config
		if err := ae.executePluginWithSetup(ctx, "docker", "install", configData, state); err != nil {
			return fmt.Errorf("failed to install %s: %w", db, err)
		}
	}

	return nil
}

// installDesktopApps installs desktop applications
func (ae *ActionExecutor) installDesktopApps(ctx context.Context, items interface{}, state *types.SetupState) error {
	apps, err := ae.toStringSlice(items)
	if err != nil {
		return fmt.Errorf("invalid install_desktop_apps format: %w", err)
	}

	log.Info("Installing desktop applications", "apps", apps)
	// TODO: Implement desktop app installation
	return nil
}

// configureShell configures the user's shell
func (ae *ActionExecutor) configureShell(ctx context.Context, shell interface{}, state *types.SetupState) error {
	shellName, ok := shell.(string)
	if !ok {
		return fmt.Errorf("invalid shell configuration format")
	}

	// Prepare parameters
	params := map[string]interface{}{
		"shell": shellName,
	}

	// Execute tool-shell plugin
	return ae.executePluginWithSetup(ctx, "tool-shell", "setup", params, state)
}

// configureGit configures Git with user information
func (ae *ActionExecutor) configureGit(ctx context.Context, gitConfig interface{}, state *types.SetupState) error {
	// Convert git config to parameters
	var params map[string]interface{}

	switch v := gitConfig.(type) {
	case map[string]interface{}:
		params = v
	case string:
		// If it's a string, it might be a variable reference
		if name, ok := state.Answers["git_full_name"].(string); ok {
			if email, ok := state.Answers["git_email"].(string); ok {
				params = map[string]interface{}{
					"name":  name,
					"email": email,
				}
			}
		}
	default:
		return fmt.Errorf("invalid git configuration format")
	}

	if params == nil {
		return fmt.Errorf("failed to resolve git configuration parameters")
	}

	// Execute tool-git plugin
	return ae.executePluginWithSetup(ctx, "tool-git", "setup", params, state)
}

// executePluginWithSetup executes a plugin using the SDK setup protocol
func (ae *ActionExecutor) executePluginWithSetup(
	ctx context.Context,
	pluginName string,
	command string,
	config map[string]interface{},
	state *types.SetupState,
) error {
	// Download plugin if not already installed
	log.Info("Checking plugin", "plugin", pluginName)
	if err := ae.pluginBootstrap.GetManager().DiscoverPluginsWithContext(ctx); err != nil {
		log.Warn("Failed to discover plugins", "error", err)
	}

	// Check if plugin is installed
	plugins := ae.pluginBootstrap.GetManager().ListPluginsWithContext(ctx)
	pluginInfo, exists := plugins[pluginName]
	if !exists {
		// Try to download the plugin
		log.Info("Downloading plugin", "plugin", pluginName)
		downloader := ae.pluginBootstrap.GetManager()
		// TODO: Use downloader to download plugin
		_ = downloader
		return fmt.Errorf("plugin %s not installed and auto-download not yet implemented", pluginName)
	}

	// Prepare setup input
	input := &sdk.PluginSetupInput{
		Command:    command,
		Config:     config,
		Parameters: state.Answers,
		Environment: sdk.PlatformInfo{
			OS:           ae.platform.OS,
			Distribution: ae.platform.Distribution,
			Desktop:      ae.platform.DesktopEnv,
			Architecture: ae.platform.Architecture,
			HasDesktop:   ae.platform.DesktopEnv != "" && ae.platform.DesktopEnv != "none",
		},
	}

	// Serialize input to JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal plugin input: %w", err)
	}

	// Execute plugin
	cmd := exec.CommandContext(ctx, pluginInfo.Path, command)
	cmd.Stdin = bytes.NewReader(inputJSON)

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Info("Executing plugin", "plugin", pluginName, "command", command)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("plugin execution failed: %w\nStderr: %s", err, stderr.String())
	}

	// Parse output
	var output sdk.PluginSetupOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		// If JSON parsing fails, treat stdout as plain text
		log.Info("Plugin output", "output", stdout.String())
		return nil
	}

	// Check status
	if output.Status == string(sdk.SetupStatusError) {
		return fmt.Errorf("plugin error: %s", output.Error)
	}

	log.Info("Plugin execution completed", "status", output.Status, "message", output.Message)
	return nil
}

// resolveConfigMapping resolves config mapping from action params
func (ae *ActionExecutor) resolveConfigMapping(mapping interface{}, state *types.SetupState) map[string]interface{} {
	result := make(map[string]interface{})

	switch v := mapping.(type) {
	case string:
		// Simple variable reference
		if val, ok := state.Answers[v]; ok {
			result[v] = val
		}
	case map[string]interface{}:
		// Complex mapping
		for key, value := range v {
			if strVal, ok := value.(string); ok {
				// Check if it's a variable reference
				if strings.HasPrefix(strVal, "{{") && strings.HasSuffix(strVal, "}}") {
					varName := strings.TrimSpace(strVal[2 : len(strVal)-2])
					if val, ok := state.Answers[varName]; ok {
						result[key] = val
					}
				} else {
					result[key] = value
				}
			} else {
				result[key] = value
			}
		}
	}

	return result
}

// loadConfigFile loads a YAML config file
func (ae *ActionExecutor) loadConfigFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// toStringSlice converts various types to []string
func (ae *ActionExecutor) toStringSlice(val interface{}) ([]string, error) {
	switch v := val.(type) {
	case []string:
		return v, nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result, nil
	case string:
		// Split by comma if it's a single string
		return strings.Split(v, ","), nil
	default:
		return nil, fmt.Errorf("cannot convert %T to []string", val)
	}
}
