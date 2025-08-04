package themes

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
)

type Theme struct {
	Name              string `yaml:"name"`
	ThemeColor        string `yaml:"theme_color"`
	ThemeBackground   string `yaml:"theme_background"`
	NeovimColorscheme string `yaml:"neovim_colorscheme"`
}

type ThemeConfig struct {
	Themes []Theme `yaml:"themes"`
}

// LoadThemes loads themes from the specified YAML file.
func LoadThemes(filePath string) ([]Theme, error) {
	log.Info("Loading themes from file", "filePath", filePath)

	// Read the YAML file
	data, err := fs.ReadFile(filePath)
	if err != nil {
		log.Error("Failed to read theme file", err, "filePath", filePath)
		return nil, fmt.Errorf("failed to read theme file %s: %w", filePath, err)
	}

	// Parse the YAML content into ThemeConfig
	var config ThemeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		log.Error("Failed to parse theme YAML", err, "filePath", filePath)
		return nil, fmt.Errorf("failed to parse theme YAML: %w", err)
	}

	log.Info("Themes loaded successfully", "count", len(config.Themes))
	return config.Themes, nil
}

// GetAvailableThemes collects all unique themes from apps with theme support
func GetAvailableThemes(apps []interface{}) []Theme {
	log.Debug("Collecting available themes from apps")

	themeMap := make(map[string]Theme)

	// Extract themes from apps (this function will work with any app structure)
	for _, app := range apps {
		if appMap, ok := app.(map[string]interface{}); ok {
			if themesInterface, exists := appMap["themes"]; exists {
				if themes, ok := themesInterface.([]interface{}); ok {
					for _, themeInterface := range themes {
						if themeData, ok := themeInterface.(map[string]interface{}); ok {
							theme := Theme{
								Name:            getString(themeData, "name"),
								ThemeColor:      getString(themeData, "theme_color"),
								ThemeBackground: getString(themeData, "theme_background"),
							}
							if theme.Name != "" {
								themeMap[theme.Name] = theme
							}
						}
					}
				}
			}
		}
	}

	// Convert map to slice
	themes := make([]Theme, 0, len(themeMap))
	for _, theme := range themeMap {
		themes = append(themes, theme)
	}

	log.Debug("Available themes collected", "count", len(themes))
	return themes
}

// getString safely extracts string values from map
func getString(m map[string]interface{}, key string) string {
	if val, exists := m[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}
