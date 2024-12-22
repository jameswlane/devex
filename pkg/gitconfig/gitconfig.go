package gitconfig

import (
	"fmt"
	"os"
	"os/exec"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/pkg/logger"
)

type GitConfig struct {
	Aliases  map[string]string `yaml:"aliases"`
	Settings map[string]string `yaml:"settings"`
}

var log = logger.InitLogger()

// LoadGitConfig loads the Git configuration from a YAML file
func LoadGitConfig(filename string) (*GitConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read git config YAML file: %w", err)
	}

	var gitConfig GitConfig
	if err := yaml.Unmarshal(data, &gitConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal git config YAML: %w", err)
	}

	return &gitConfig, nil
}

// ApplyGitConfig applies the Git configuration (aliases and settings)
func ApplyGitConfig(gitConfig *GitConfig) error {
	if err := applyAliases(gitConfig.Aliases); err != nil {
		return fmt.Errorf("failed to apply aliases: %w", err)
	}

	if err := applySettings(gitConfig.Settings); err != nil {
		return fmt.Errorf("failed to apply settings: %w", err)
	}

	return nil
}

func applyAliases(aliases map[string]string) error {
	for alias, command := range aliases {
		log.LogInfo(fmt.Sprintf("Setting Git alias: alias=%s, command=%s", alias, command))
		if err := runGitCommand([]string{"config", "--global", fmt.Sprintf("alias.%s", alias), command}); err != nil {
			return fmt.Errorf("failed to set git alias %s: %w", alias, err)
		}
	}
	return nil
}

func applySettings(settings map[string]string) error {
	for key, value := range settings {
		log.LogInfo(fmt.Sprintf("Setting Git configuration: key=%s, value=%s", key, value))
		if err := runGitCommand([]string{"config", "--global", key, value}); err != nil {
			return fmt.Errorf("failed to set git configuration %s: %w", key, err)
		}
	}
	return nil
}

func runGitCommand(args []string) error {
	log.LogInfo(fmt.Sprintf("Running git command: git %s", args))
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
