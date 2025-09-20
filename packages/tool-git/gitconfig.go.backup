package main

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/utils"
)

type GitConfig struct {
	Aliases  map[string]string `yaml:"aliases"`
	Settings map[string]string `yaml:"settings"`
}

// LoadGitConfig loads the Git configuration from a YAML file.
func LoadGitConfig(filename string) (*GitConfig, error) {
	log.Info("Loading Git configuration from file", "filename", filename)

	data, err := fs.ReadFile(filename)
	if err != nil {
		log.Error("Failed to read Git config YAML file", err, "filename", filename)
		return nil, fmt.Errorf("failed to read Git config YAML file: %w", err)
	}

	var gitConfig GitConfig
	if err := yaml.Unmarshal(data, &gitConfig); err != nil {
		log.Error("Failed to unmarshal Git config YAML", err, "filename", filename)
		return nil, fmt.Errorf("failed to unmarshal Git config YAML: %w", err)
	}

	log.Info("Git configuration loaded successfully", "aliases", len(gitConfig.Aliases), "settings", len(gitConfig.Settings))
	return &gitConfig, nil
}

// ApplyGitConfig applies the Git configuration (aliases and settings).
func ApplyGitConfig(gitConfig *GitConfig) error {
	log.Info("Applying Git configuration")

	if err := applyAliases(gitConfig.Aliases); err != nil {
		log.Error("Failed to apply Git aliases", err)
		return fmt.Errorf("failed to apply aliases: %w", err)
	}

	if err := applySettings(gitConfig.Settings); err != nil {
		log.Error("Failed to apply Git settings", err)
		return fmt.Errorf("failed to apply settings: %w", err)
	}

	log.Info("Git configuration applied successfully")
	return nil
}

// applyAliases applies Git aliases.
func applyAliases(aliases map[string]string) error {
	for alias, command := range aliases {
		log.Info("Setting Git alias", "alias", alias, "command", command)
		_, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("git config --global alias.%s %s", alias, command))
		if err != nil {
			return fmt.Errorf("failed to set git alias %s: %w", alias, err)
		}
	}
	return nil
}

// applySettings applies Git settings.
func applySettings(settings map[string]string) error {
	for key, value := range settings {
		log.Info("Setting Git configuration", "key", key, "value", value)
		_, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("git config --global %s %s", key, value))
		if err != nil {
			return fmt.Errorf("failed to set git configuration %s: %w", key, err)
		}
	}
	return nil
}
