package gitconfig

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
)

type GitConfig struct {
	Aliases  map[string]string `yaml:"aliases"`
	Settings map[string]string `yaml:"settings"`
}

// LoadGitConfig loads the Git configuration from a YAML file
func LoadGitConfig(filename string) (*GitConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read git config YAML file: %v", err)
	}

	var gitConfig GitConfig
	err = yaml.Unmarshal(data, &gitConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal git config YAML: %v", err)
	}

	return &gitConfig, nil
}

// ApplyGitConfig applies the Git configuration (aliases and settings)
func ApplyGitConfig(gitConfig *GitConfig) error {
	// Apply aliases
	for alias, command := range gitConfig.Aliases {
		logger.LogInfo("Setting Git alias", "alias", alias, "command", command)
		cmd := exec.Command("git", "config", "--global", fmt.Sprintf("alias.%s", alias), command)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set git alias %s: %v", alias, err)
		}
	}

	// Apply settings
	for key, value := range gitConfig.Settings {
		logger.LogInfo("Setting Git configuration", "key", key, "value", value)
		cmd := exec.Command("git", "config", "--global", key, value)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to set git configuration %s: %v", key, err)
		}
	}

	return nil
}
