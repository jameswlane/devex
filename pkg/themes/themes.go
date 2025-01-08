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
