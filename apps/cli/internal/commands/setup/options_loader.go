package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// OptionsLoader loads options dynamically from various sources
type OptionsLoader struct {
	settings config.CrossPlatformSettings
	platform platform.DetectionResult
}

// NewOptionsLoader creates a new options loader
func NewOptionsLoader(settings config.CrossPlatformSettings, platform platform.DetectionResult) *OptionsLoader {
	return &OptionsLoader{
		settings: settings,
		platform: platform,
	}
}

// Load loads options from the specified source
func (ol *OptionsLoader) Load(source *types.OptionsSource) ([]types.QuestionOption, error) {
	switch source.Type {
	case types.SourceTypeStatic:
		// Static options should already be in the question definition
		return []types.QuestionOption{}, nil

	case types.SourceTypeConfig:
		return ol.loadFromConfig(source)

	case types.SourceTypeSystem:
		return ol.loadFromSystem(source)

	case types.SourceTypePlugin:
		return ol.loadFromPlugin(source)

	default:
		return nil, fmt.Errorf("unknown options source type: %s", source.Type)
	}
}

// loadFromConfig loads options from configuration files
func (ol *OptionsLoader) loadFromConfig(source *types.OptionsSource) ([]types.QuestionOption, error) {
	// Apply transform based on the key
	switch source.Transform {
	case "get_language_names":
		return ol.getLanguageOptions(), nil

	case "get_theme_names":
		return ol.getThemeOptions(), nil

	case "filter_by_platform":
		// This would filter applications by current platform
		return ol.getDesktopAppOptions(), nil

	case "load_directory":
		// Load options from YAML files in a directory
		return ol.loadFromDirectory(source.Path)

	default:
		return nil, fmt.Errorf("unknown transform: %s", source.Transform)
	}
}

// loadFromSystem loads options from system detection
func (ol *OptionsLoader) loadFromSystem(source *types.OptionsSource) ([]types.QuestionOption, error) {
	switch source.SystemType {
	case "shells":
		return ol.getAvailableShells(), nil

	case "desktop_environments":
		return ol.getDesktopEnvironments(), nil

	default:
		return nil, fmt.Errorf("unknown system type: %s", source.SystemType)
	}
}

// loadFromPlugin loads options from plugins
func (ol *OptionsLoader) loadFromPlugin(source *types.OptionsSource) ([]types.QuestionOption, error) {
	// This would query plugins for available options
	// For now, return empty
	return []types.QuestionOption{}, nil
}

// getLanguageOptions returns programming language options
func (ol *OptionsLoader) getLanguageOptions() []types.QuestionOption {
	// Extract language names from settings
	languages := getProgrammingLanguageNames(ol.settings)

	options := make([]types.QuestionOption, len(languages))
	for i, lang := range languages {
		options[i] = types.QuestionOption{
			Label: lang,
			Value: lang,
		}
	}

	return options
}

// getThemeOptions returns available theme options
func (ol *OptionsLoader) getThemeOptions() []types.QuestionOption {
	// Extract theme names from settings
	themes := getAvailableThemeNames(ol.settings)

	options := make([]types.QuestionOption, len(themes))
	for i, theme := range themes {
		options[i] = types.QuestionOption{
			Label: theme,
			Value: theme,
		}
	}

	return options
}

// getDesktopAppOptions returns desktop application options
func (ol *OptionsLoader) getDesktopAppOptions() []types.QuestionOption {
	// Extract desktop app names from settings
	apps := getDesktopAppNames(ol.settings)

	options := make([]types.QuestionOption, len(apps))
	for i, app := range apps {
		options[i] = types.QuestionOption{
			Label: app,
			Value: app,
		}
	}

	return options
}

// getAvailableShells returns shell options based on system
func (ol *OptionsLoader) getAvailableShells() []types.QuestionOption {
	// For now, return standard shells
	// In the future, this could detect installed shells
	shells := []string{"zsh", "bash", "fish"}

	options := make([]types.QuestionOption, len(shells))
	for i, shell := range shells {
		description := ""
		switch shell {
		case "zsh":
			description = "Modern shell with powerful features"
		case "bash":
			description = "Bourne Again Shell - widely compatible"
		case "fish":
			description = "Friendly interactive shell"
		}

		options[i] = types.QuestionOption{
			Label:       shell,
			Value:       shell,
			Description: description,
		}
	}

	return options
}

// getDesktopEnvironments returns desktop environment options
func (ol *OptionsLoader) getDesktopEnvironments() []types.QuestionOption {
	// Return available desktop environments based on platform
	if ol.platform.OS != "linux" {
		return []types.QuestionOption{}
	}

	desktops := []string{"gnome", "kde", "xfce", "mate"}

	options := make([]types.QuestionOption, len(desktops))
	for i, desktop := range desktops {
		options[i] = types.QuestionOption{
			Label: desktop,
			Value: desktop,
		}
	}

	return options
}

// Helper function to extract base filename without extension
func getBaseName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return base[:len(base)-len(ext)]
}

// loadFromDirectory loads options from YAML files in a directory
func (ol *OptionsLoader) loadFromDirectory(dirPath string) ([]types.QuestionOption, error) {
	// Read directory entries
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	// Pre-allocate with estimated capacity (assume all files could be valid)
	options := make([]types.QuestionOption, 0, len(entries))

	// Iterate through files
	for _, entry := range entries {
		// Skip directories and non-YAML files
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".yaml") && !strings.HasSuffix(strings.ToLower(name), ".yml") {
			continue
		}

		// Read YAML file
		filePath := filepath.Join(dirPath, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			// Skip files we can't read
			continue
		}

		// Parse YAML to extract metadata
		var metadata struct {
			Name        string `yaml:"name"`
			Description string `yaml:"description"`
		}

		if err := yaml.Unmarshal(data, &metadata); err != nil {
			// Skip files we can't parse
			continue
		}

		// Create option from metadata
		// Use the base filename (without extension) as the value
		value := getBaseName(name)

		option := types.QuestionOption{
			Label:       metadata.Name,
			Value:       value,
			Description: metadata.Description,
		}

		options = append(options, option)
	}

	if len(options) == 0 {
		return nil, fmt.Errorf("no valid YAML files found in directory: %s", dirPath)
	}

	return options, nil
}
