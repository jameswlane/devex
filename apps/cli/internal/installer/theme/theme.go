// Package theme handles theme selection and application for installed applications
package theme

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Manager handles theme operations for applications
type Manager struct {
	repo            types.Repository
	commandExecutor CommandExecutor
}

// CommandExecutor interface for executing system commands
type CommandExecutor interface {
	Execute(ctx context.Context, command string) error
}

// New creates a new theme manager
func New(repo types.Repository, executor CommandExecutor) *Manager {
	return &Manager{
		repo:            repo,
		commandExecutor: executor,
	}
}

// SelectTheme stores the theme selection for an application
func (m *Manager) SelectTheme(appName, themeName string) error {
	if m.repo == nil {
		return fmt.Errorf("repository not available")
	}

	themeKey := fmt.Sprintf("app_theme_%s", appName)
	return m.repo.Set(themeKey, themeName)
}

// GetSelectedTheme retrieves the selected theme for an application
func (m *Manager) GetSelectedTheme(appName string) (string, error) {
	if m.repo == nil {
		return "", fmt.Errorf("repository not available")
	}

	themeKey := fmt.Sprintf("app_theme_%s", appName)
	return m.repo.Get(themeKey)
}

// SetGlobalTheme sets the global theme preference
func (m *Manager) SetGlobalTheme(themeName string) error {
	if m.repo == nil {
		return fmt.Errorf("repository not available")
	}

	return m.repo.Set("global_theme", themeName)
}

// GetGlobalTheme retrieves the global theme preference
func (m *Manager) GetGlobalTheme() (string, error) {
	if m.repo == nil {
		return "", fmt.Errorf("repository not available")
	}

	return m.repo.Get("global_theme")
}

// UseGlobalTheme applies the global theme preference to an application
func (m *Manager) UseGlobalTheme(appName string, availableThemes []types.Theme) error {
	globalTheme, err := m.GetGlobalTheme()
	if err != nil || globalTheme == "" {
		return fmt.Errorf("no global theme preference found")
	}

	// Find the global theme in available themes
	for _, theme := range availableThemes {
		if theme.Name == globalTheme {
			return m.SelectTheme(appName, theme.Name)
		}
	}

	return fmt.Errorf("global theme '%s' not available for %s", globalTheme, appName)
}

// ApplyTheme applies the selected theme files for an application
func (m *Manager) ApplyTheme(ctx context.Context, appName string, availableThemes []types.Theme) error {
	selectedThemeName, err := m.GetSelectedTheme(appName)
	if err != nil {
		return fmt.Errorf("no theme selected for %s: %w", appName, err)
	}

	// Find the selected theme
	var selectedTheme *types.Theme
	for _, theme := range availableThemes {
		if theme.Name == selectedThemeName {
			selectedTheme = &theme
			break
		}
	}

	if selectedTheme == nil {
		return fmt.Errorf("selected theme '%s' not found for %s", selectedThemeName, appName)
	}

	// Apply theme files
	for _, configFile := range selectedTheme.Files {
		if err := m.applyThemeFile(ctx, configFile); err != nil {
			// Log error but continue with other files
			fmt.Printf("Warning: Failed to apply theme file %s: %v\n", configFile.Source, err)
		}
	}

	return nil
}

// applyThemeFile copies a single theme configuration file
func (m *Manager) applyThemeFile(ctx context.Context, file types.ConfigFile) error {
	source := expandPath(file.Source)
	destination := expandPath(file.Destination)

	// Create destination directory if needed
	destDir := filepath.Dir(destination)
	if err := m.ensureDirectory(ctx, destDir); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	// Copy the file
	copyCmd := fmt.Sprintf("cp '%s' '%s'", source, destination)
	if err := m.commandExecutor.Execute(ctx, copyCmd); err != nil {
		return fmt.Errorf("failed to copy theme file: %w", err)
	}

	return nil
}

// ensureDirectory creates a directory if it doesn't exist
func (m *Manager) ensureDirectory(ctx context.Context, dir string) error {
	if dir == "." || dir == "/" {
		return nil
	}

	// Check if directory exists
	if _, err := os.Stat(dir); err == nil {
		return nil
	}

	// Create directory
	cmd := fmt.Sprintf("mkdir -p '%s'", dir)
	return m.commandExecutor.Execute(ctx, cmd)
}

// expandPath expands tilde (~) in file paths to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = os.Getenv("USERPROFILE") // Windows fallback
		}
		return strings.Replace(path, "~", homeDir, 1)
	}
	return path
}

// ValidateTheme checks if a theme is valid and has all required files
func (m *Manager) ValidateTheme(theme types.Theme) error {
	if theme.Name == "" {
		return fmt.Errorf("theme name cannot be empty")
	}

	if len(theme.Files) == 0 {
		return fmt.Errorf("theme must have at least one configuration file")
	}

	for i, file := range theme.Files {
		if file.Source == "" {
			return fmt.Errorf("theme file %d has empty source path", i)
		}
		if file.Destination == "" {
			return fmt.Errorf("theme file %d has empty destination path", i)
		}
	}

	return nil
}

// ListAvailableThemes returns a list of theme names from available themes
func (m *Manager) ListAvailableThemes(themes []types.Theme) []string {
	names := make([]string, len(themes))
	for i, theme := range themes {
		names[i] = theme.Name
	}
	return names
}
