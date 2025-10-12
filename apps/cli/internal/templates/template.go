package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Template represents a development environment template
type Template struct {
	Metadata     TemplateMetadata    `yaml:"metadata"`
	Applications []types.AppConfig   `yaml:"applications"`
	Environment  EnvironmentTemplate `yaml:"environment"`
	System       SystemTemplate      `yaml:"system"`
	Desktop      *DesktopTemplate    `yaml:"desktop,omitempty"`
}

// TemplateMetadata contains template information
type TemplateMetadata struct {
	Name          string   `yaml:"name"`
	Version       string   `yaml:"version"`
	Description   string   `yaml:"description"`
	Category      string   `yaml:"category"`
	Tags          []string `yaml:"tags"`
	Author        string   `yaml:"author,omitempty"`
	Homepage      string   `yaml:"homepage,omitempty"`
	Icon          string   `yaml:"icon,omitempty"`
	Screenshots   []string `yaml:"screenshots,omitempty"`
	Platforms     []string `yaml:"platforms"`                // ["linux", "macos", "windows"]
	Difficulty    string   `yaml:"difficulty"`               // "beginner", "intermediate", "advanced"
	EstimatedTime string   `yaml:"estimated_time,omitempty"` // "30 minutes"
	Additive      bool     `yaml:"additive"`                 // true = add to default setup, false = replace setup
}

// EnvironmentTemplate defines environment configuration for templates
type EnvironmentTemplate struct {
	Shell     string            `yaml:"shell"`
	Editor    string            `yaml:"editor"`
	Languages []string          `yaml:"languages"`
	Variables map[string]string `yaml:"variables,omitempty"`
	DotFiles  []string          `yaml:"dotfiles,omitempty"`
	Terminal  TerminalConfig    `yaml:"terminal,omitempty"`
}

// TerminalConfig defines terminal preferences
type TerminalConfig struct {
	Theme       string            `yaml:"theme,omitempty"`
	FontFamily  string            `yaml:"font_family,omitempty"`
	FontSize    int               `yaml:"font_size,omitempty"`
	Colorscheme string            `yaml:"colorscheme,omitempty"`
	Profiles    map[string]string `yaml:"profiles,omitempty"`
}

// SystemTemplate defines system configuration for templates
type SystemTemplate struct {
	GitConfig   bool              `yaml:"configure_git"`
	SSHConfig   bool              `yaml:"configure_ssh"`
	GlobalTheme string            `yaml:"global_theme,omitempty"`
	Services    []string          `yaml:"services,omitempty"`
	Directories []string          `yaml:"directories,omitempty"`
	SystemFiles map[string]string `yaml:"system_files,omitempty"`
}

// DesktopTemplate defines desktop environment configuration
type DesktopTemplate struct {
	Environment string            `yaml:"environment"`
	Themes      []string          `yaml:"themes,omitempty"`
	Extensions  []string          `yaml:"extensions,omitempty"`
	Shortcuts   map[string]string `yaml:"shortcuts,omitempty"`
	Wallpaper   string            `yaml:"wallpaper,omitempty"`
}

// TemplateManager handles template operations
type TemplateManager struct {
	builtinTemplatesDir string
	userTemplatesDir    string
}

// NewTemplateManager creates a new template manager
func NewTemplateManager(homeDir string) *TemplateManager {
	// Detect if we're running from the CLI binary location or development
	builtinDir := detectBuiltinTemplatesDir()

	return &TemplateManager{
		builtinTemplatesDir: builtinDir,
		userTemplatesDir:    filepath.Join(homeDir, ".devex", "templates"),
	}
}

// detectBuiltinTemplatesDir detects the location of built-in templates
func detectBuiltinTemplatesDir() string {
	// Try different possible locations for built-in templates
	possiblePaths := []string{
		"../../assets/templates",     // Test mode (from internal/templates)
		"assets/templates",           // Development mode (relative to binary)
		"./assets/templates",         // Current directory
		"/usr/share/devex/templates", // System install
		"/opt/devex/templates",       // Alternative system install
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Fallback - try to find relative to the executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		assetsPath := filepath.Join(execDir, "assets", "templates")
		if _, err := os.Stat(assetsPath); err == nil {
			return assetsPath
		}

		// Try going up directories (for development)
		for i := 0; i < 3; i++ {
			execDir = filepath.Dir(execDir)
			assetsPath := filepath.Join(execDir, "assets", "templates")
			if _, err := os.Stat(assetsPath); err == nil {
				return assetsPath
			}
		}
	}

	// Final fallback
	return "assets/templates"
}

// GetAvailableTemplates returns all available templates with user overrides
func (tm *TemplateManager) GetAvailableTemplates() ([]Template, error) {
	templateMap := make(map[string]Template)

	// Load built-in templates first
	builtinTemplates, err := tm.loadTemplatesFromDir(tm.builtinTemplatesDir)
	if err != nil {
		fmt.Printf("Warning: Failed to load built-in templates: %v\n", err)
	} else {
		for _, template := range builtinTemplates {
			templateMap[template.Metadata.Name] = template
		}
	}

	// Load user templates and allow them to override built-in ones
	userTemplates, err := tm.loadTemplatesFromDir(tm.userTemplatesDir)
	if err != nil {
		// User templates directory might not exist, that's okay
		fmt.Printf("Note: No user templates found (%v)\n", err)
	} else {
		for _, template := range userTemplates {
			if _, exists := templateMap[template.Metadata.Name]; exists {
				fmt.Printf("Note: User template '%s' overriding built-in template\n", template.Metadata.Name)
			}
			templateMap[template.Metadata.Name] = template
		}
	}

	// Convert map back to slice
	templates := make([]Template, 0, len(templateMap))
	for _, template := range templateMap {
		templates = append(templates, template)
	}

	return templates, nil
}

// ListTemplatesSources returns templates with their source information
func (tm *TemplateManager) ListTemplatesSources() (map[string]string, error) {
	sources := make(map[string]string)

	// Load built-in templates first
	builtinTemplates, err := tm.loadTemplatesFromDir(tm.builtinTemplatesDir)
	if err == nil {
		for _, template := range builtinTemplates {
			sources[template.Metadata.Name] = "built-in"
		}
	}

	// Load user templates and mark overrides
	userTemplates, err := tm.loadTemplatesFromDir(tm.userTemplatesDir)
	if err == nil {
		for _, template := range userTemplates {
			if _, exists := sources[template.Metadata.Name]; exists {
				sources[template.Metadata.Name] = "user (overriding built-in)"
			} else {
				sources[template.Metadata.Name] = "user"
			}
		}
	}

	return sources, nil
}

// GetTemplate retrieves a specific template by name
func (tm *TemplateManager) GetTemplate(name string) (*Template, error) {
	templates, err := tm.GetAvailableTemplates()
	if err != nil {
		return nil, err
	}

	for _, template := range templates {
		if template.Metadata.Name == name {
			return &template, nil
		}
	}

	return nil, fmt.Errorf("template '%s' not found", name)
}

// SaveTemplate saves a template to the user templates directory
func (tm *TemplateManager) SaveTemplate(template Template) error {
	// Ensure user templates directory exists
	if err := os.MkdirAll(tm.userTemplatesDir, 0750); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	// Create filename from template name
	filename := fmt.Sprintf("%s.yaml", strings.ToLower(strings.ReplaceAll(template.Metadata.Name, " ", "-")))
	path := filepath.Join(tm.userTemplatesDir, filename)

	// Marshal template to YAML
	data, err := yaml.Marshal(template)
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to save template: %w", err)
	}

	return nil
}

// ValidateTemplate validates a template structure
func (tm *TemplateManager) ValidateTemplate(template Template) error {
	// Check required metadata
	if template.Metadata.Name == "" {
		return fmt.Errorf("template name is required")
	}
	if template.Metadata.Description == "" {
		return fmt.Errorf("template description is required")
	}
	if template.Metadata.Category == "" {
		return fmt.Errorf("template category is required")
	}

	// Validate applications
	for _, app := range template.Applications {
		if err := app.Validate(); err != nil {
			return fmt.Errorf("invalid application '%s': %w", app.Name, err)
		}
	}

	// Validate platforms
	validPlatforms := map[string]bool{
		"linux":   true,
		"macos":   true,
		"windows": true,
	}
	for _, platform := range template.Metadata.Platforms {
		if !validPlatforms[platform] {
			return fmt.Errorf("invalid platform '%s'", platform)
		}
	}

	// Validate difficulty
	validDifficulties := map[string]bool{
		"beginner":     true,
		"intermediate": true,
		"advanced":     true,
	}
	if template.Metadata.Difficulty != "" && !validDifficulties[template.Metadata.Difficulty] {
		return fmt.Errorf("invalid difficulty level '%s'", template.Metadata.Difficulty)
	}

	return nil
}

// loadTemplatesFromDir loads templates from a directory
func (tm *TemplateManager) loadTemplatesFromDir(dir string) ([]Template, error) {
	templates := make([]Template, 0)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return templates, nil // Directory doesn't exist, return empty slice
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("Warning: failed to read template %s: %v\n", entry.Name(), err)
			continue
		}

		var template Template
		if err := yaml.Unmarshal(data, &template); err != nil {
			fmt.Printf("Warning: failed to parse template %s: %v\n", entry.Name(), err)
			continue
		}

		// Validate template
		if err := tm.ValidateTemplate(template); err != nil {
			fmt.Printf("Warning: invalid template %s: %v\n", entry.Name(), err)
			continue
		}

		templates = append(templates, template)
	}

	return templates, nil
}
