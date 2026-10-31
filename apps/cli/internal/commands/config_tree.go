package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func initializeConfigTree(dir string, team bool) error {
	for _, name := range config.ConfigDirectories {
		if err := os.MkdirAll(filepath.Join(dir, name), 0750); err != nil {
			return fmt.Errorf("create %s configuration: %w", name, err)
		}
	}
	if team {
		path, err := applicationConfigPath(dir, "applications", "development", "git")
		if err != nil {
			return err
		}
		app := types.CrossPlatformApp{Name: "git", Description: "Version control system", Category: "development", Default: true,
			Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"},
			MacOS: types.OSConfig{InstallMethod: "brew", InstallCommand: "git"}}
		if err := writeYAMLConfig(app, path, "# Shared team application\n"); err != nil {
			return err
		}
	}
	return writeYAMLConfig([]types.GitConfig{}, filepath.Join(dir, "system", "git.yaml"), "# Shared Git settings\n")
}

func copyConfigurationTree(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("read source configuration %s: %w", src, err)
	}
	files, err := config.ConfigurationFiles(src)
	if err != nil {
		return err
	}
	for _, relative := range files {
		data, err := os.ReadFile(filepath.Join(src, relative))
		if err != nil {
			return fmt.Errorf("read configuration %s: %w", relative, err)
		}
		if relative == "environment.yaml" || relative == "system.yaml" || relative == "desktop.yaml" {
			var section map[string]interface{}
			if err := yaml.Unmarshal(data, &section); err != nil {
				return fmt.Errorf("parse %s: %w", relative, err)
			}
			if err := writeImportedConfiguration(dst, strings.TrimSuffix(relative, ".yaml"), section); err != nil {
				return err
			}
			if err := os.Remove(filepath.Join(dst, relative)); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}

		target := filepath.Join(dst, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0600); err != nil { // #nosec G703 -- relative paths come from ConfigurationFiles, which enumerates fixed configuration roots
			return fmt.Errorf("copy configuration %s: %w", relative, err)
		}
	}
	return migrateApplicationConfig(dst)
}

// configurationFilePath validates paths carried in exported data before writing them.
func configurationFilePath(dir, key string) (string, error) {
	relative := filepath.FromSlash(key + ".yaml")
	if !filepath.IsLocal(relative) || strings.Contains(key, "\\") {
		return "", fmt.Errorf("invalid configuration path %q: use a relative configuration file path", key)
	}
	root := strings.Split(filepath.ToSlash(relative), "/")[0]
	if strings.Contains(filepath.ToSlash(relative), "/") {
		valid := false
		for _, name := range config.ConfigDirectories {
			if root == name {
				valid = true
			}
		}
		if !valid {
			return "", fmt.Errorf("unknown configuration directory %q", root)
		}
	} else {
		valid := root == "devex.yaml"
		for _, name := range config.CrossPlatformFiles {
			if root == name {
				valid = true
			}
		}
		if !valid {
			return "", fmt.Errorf("unknown configuration file %q", root)
		}
	}
	return filepath.Join(dir, relative), nil
}

func configurationSection(relative string) string {
	root := strings.Split(filepath.ToSlash(relative), "/")[0]
	root = strings.TrimSuffix(strings.TrimSuffix(root, ".yaml"), ".yml")
	if root == "environments" {
		return "environment"
	}
	return root
}

func includeConfiguration(relative string, include, exclude []string) bool {
	section := configurationSection(relative)
	for _, name := range exclude {
		if name == section {
			return false
		}
	}
	if len(include) == 0 {
		return true
	}
	for _, name := range include {
		if name == section {
			return true
		}
	}
	return false
}

// writeImportedConfiguration accepts legacy sections and relative tree-file keys.
func writeImportedConfiguration(dir, key string, data interface{}) error {
	path, err := configurationFilePath(dir, key)
	if err != nil {
		return err
	}
	switch key {
	case "applications":
		return importApplications(data, dir)
	case "environment":
		section, ok := data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("environment must be a mapping")
		}
		settings := make(map[string]interface{}, len(section))
		for key, value := range section {
			settings[key] = value
		}
		preferences := make(map[string]interface{})
		for _, key := range []string{"shell", "editor", "languages", "variables"} {
			if value, exists := settings[key]; exists {
				if key == "shell" {
					if _, scalar := value.(string); !scalar {
						continue
					}
				}
				preferences[key] = value
				delete(settings, key)
			}
		}
		if len(preferences) > 0 {
			metadata := make(map[string]interface{})
			metadataPath := filepath.Join(dir, "devex.yaml")
			if existing, err := os.ReadFile(metadataPath); err == nil {
				if err := yaml.Unmarshal(existing, &metadata); err != nil {
					return err
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			metadata["environment"] = preferences
			if err := writeYAMLConfig(metadata, metadataPath, ""); err != nil {
				return err
			}
		}
		return importEnvironment(settings, dir)
	case "system":
		return importSystem(data, dir)
	case "desktop":
		section, ok := data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("desktop must be a mapping")
		}
		if environments, ok := section["desktop_environments"].(map[string]interface{}); ok {
			section = environments
		}
		return importDesktop(section, dir)
	default:
		return writeYAMLConfig(data, path, "")
	}
}
