package repository

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const (
	globalThemeKey = "global_theme"
	appThemePrefix = "app_theme_"
)

// isNotFoundError checks if the error indicates a key was not found
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for the standard "not found" error message pattern
	return strings.Contains(err.Error(), "not found")
}

type themeRepository struct {
	systemRepo types.SystemRepository
}

func NewThemeRepository(systemRepo types.SystemRepository) types.ThemeRepository {
	return &themeRepository{systemRepo: systemRepo}
}

func (r *themeRepository) GetGlobalTheme() (string, error) {
	log.Debug("Retrieving global theme preference")

	theme, err := r.systemRepo.Get(globalThemeKey)
	if err != nil {
		if isNotFoundError(err) {
			log.Debug("No global theme preference found, using default")
			return "", nil // Return empty string if no preference set
		}
		// Actual database error - propagate it
		log.Error("Database error retrieving global theme preference", err)
		return "", fmt.Errorf("failed to retrieve global theme preference: %w", err)
	}

	log.Debug("Global theme preference retrieved", "theme", theme)
	return theme, nil
}

func (r *themeRepository) SetGlobalTheme(theme string) error {
	log.Info("Setting global theme preference", "theme", theme)

	err := r.systemRepo.Set(globalThemeKey, theme)
	if err != nil {
		return fmt.Errorf("failed to set global theme preference: %w", err)
	}

	log.Debug("Global theme preference saved successfully")
	return nil
}

func (r *themeRepository) GetAppTheme(appName string) (string, error) {
	log.Debug("Retrieving app theme preference", "app", appName)

	key := appThemePrefix + appName
	theme, err := r.systemRepo.Get(key)
	if err != nil {
		if isNotFoundError(err) {
			log.Debug("No app theme preference found, using global theme", "app", appName)
			return r.GetGlobalTheme() // Fall back to global theme
		}
		// Actual database error - propagate it
		log.Error("Database error retrieving app theme preference", err, "app", appName)
		return "", fmt.Errorf("failed to retrieve app theme preference for %s: %w", appName, err)
	}

	log.Debug("App theme preference retrieved", "app", appName, "theme", theme)
	return theme, nil
}

func (r *themeRepository) SetAppTheme(appName, theme string) error {
	log.Info("Setting app theme preference", "app", appName, "theme", theme)

	key := appThemePrefix + appName
	err := r.systemRepo.Set(key, theme)
	if err != nil {
		return fmt.Errorf("failed to set app theme preference for %s: %w", appName, err)
	}

	log.Debug("App theme preference saved successfully", "app", appName)
	return nil
}

func (r *themeRepository) GetAllThemePreferences() (*types.ThemePreferences, error) {
	log.Debug("Retrieving all theme preferences")

	allData, err := r.systemRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve theme preferences: %w", err)
	}

	preferences := &types.ThemePreferences{
		AppThemes: make(map[string]string),
	}

	// Extract global theme and app themes from system data
	for key, value := range allData {
		if key == globalThemeKey {
			preferences.GlobalTheme = value
		} else if len(key) > len(appThemePrefix) && key[:len(appThemePrefix)] == appThemePrefix {
			appName := key[len(appThemePrefix):]
			preferences.AppThemes[appName] = value
		}
	}

	log.Debug("Theme preferences retrieved", "globalTheme", preferences.GlobalTheme, "appCount", len(preferences.AppThemes))
	return preferences, nil
}
