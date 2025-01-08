package config

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

type Settings struct {
	DebugMode       bool                   `mapstructure:"debug_mode"`
	HomeDir         string                 `mapstructure:"home_dir"`
	DryRun          bool                   `mapstructure:"dry_run"`
	Config          map[string]any         `mapstructure:"config"`
	Apps            []types.AppConfig      `mapstructure:"apps"`
	Database        []types.AppConfig      `mapstructure:"databases"`
	Dock            []types.DockItem       `mapstructure:"dock"`
	Fonts           []types.Font           `mapstructure:"fonts"`
	GitConfig       []types.GitConfig      `mapstructure:"git_config"`
	GnomeExt        []types.GnomeExtension `mapstructure:"gnome_extensions"`
	GnomeSettings   []types.GnomeSetting   `mapstructure:"gnome_settings"`
	OptionalApps    []types.AppConfig      `mapstructure:"optional_apps"`
	ProgrammingLang []types.AppConfig      `mapstructure:"programming_languages"`
	Themes          []types.Theme          `mapstructure:"themes"`
}

func LoadSettings(homeDir string) (Settings, error) {
	log.Info("Loading settings", "homeDir", homeDir)

	viper.SetConfigType("yaml")

	// Paths for default configurations
	defaultConfigPath := filepath.Join(homeDir, ".local/share/devex/config")
	overrideConfigPath := filepath.Join(homeDir, ".devex")

	// Load default configs directly into the main Viper instance
	for _, file := range DefaultFiles {
		defaultPath := filepath.Join(defaultConfigPath, file)
		if err := loadYamlFileIntoViper(defaultPath); err != nil {
			log.Warn("Failed to load default config; skipping", "file", file, "error", err)
		}
	}

	// Apply overrides from ~/.devex directory
	for _, file := range DefaultFiles {
		overridePath := filepath.Join(overrideConfigPath, file)
		if exists, err := fs.Stat(overridePath); err == nil && exists != nil {
			log.Info("Applying override", "file", overridePath)
			if err := loadYamlFileIntoViper(overridePath); err != nil {
				log.Warn("Failed to apply override; skipping", "file", overridePath, "error", err)
			}
		}
	}

	// Bind global settings
	viper.SetDefault("debug_mode", false)
	viper.SetDefault("dry_run", false)
	viper.SetDefault("home_dir", homeDir)
	viper.AutomaticEnv() // Enable environment variable overrides
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Unmarshal into Settings struct
	var settings Settings
	if err := viper.Unmarshal(&settings); err != nil {
		log.Error("Failed to unmarshal settings", err)
		return Settings{}, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	log.Info("Settings loaded successfully")
	return settings, nil
}

func loadYamlFileIntoViper(path string) error {
	log.Info("Loading YAML file into Viper", "path", path)

	data, err := fs.ReadFile(path)
	if err != nil {
		exists, err := fs.Exists(path)
		if err != nil {
			log.Warn("Failed to check if file exists", "path", path, "error", err)
			return err
		}
		if !exists {
			log.Warn("Config file not found", "path", path)
			return nil
		}
		log.Error("Failed to read YAML file", err, "path", path)
		return fmt.Errorf("failed to read YAML file %s: %w", path, err)
	}

	// Parse and merge into the main Viper instance
	subViper := viper.New()
	subViper.SetConfigType("yaml")
	if err := subViper.ReadConfig(bytes.NewReader(data)); err != nil {
		log.Error("Failed to parse YAML file", err, "path", path)
		return fmt.Errorf("failed to parse YAML file %s: %w", path, err)
	}

	// Merge settings into the main Viper instance
	for k, v := range subViper.AllSettings() {
		viper.Set(k, v)
	}

	log.Info("YAML file loaded successfully", "path", path)
	return nil
}
