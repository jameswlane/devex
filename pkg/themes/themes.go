package themes

import (
	"os"

	"gopkg.in/yaml.v3"
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

func LoadThemes(filePath string) ([]Theme, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config ThemeConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config.Themes, nil
}
