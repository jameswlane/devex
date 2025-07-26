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

// Legacy Settings struct for backward compatibility
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

// CrossPlatformSettings represents the new configuration structure
type CrossPlatformSettings struct {
	DebugMode    bool               `mapstructure:"debug_mode"`
	HomeDir      string             `mapstructure:"home_dir"`
	DryRun       bool               `mapstructure:"dry_run"`
	Config       map[string]any     `mapstructure:"config"`
	Applications ApplicationsConfig `mapstructure:"applications"`
	Environment  EnvironmentConfig  `mapstructure:"environment"`
	Desktop      DesktopConfig      `mapstructure:"desktop"`
	System       SystemConfig       `mapstructure:"system"`
}

// ApplicationsConfig represents the applications configuration
type ApplicationsConfig struct {
	Development []types.CrossPlatformApp `mapstructure:"development"`
	Databases   []types.CrossPlatformApp `mapstructure:"databases"`
	SystemTools []types.CrossPlatformApp `mapstructure:"system_tools"`
	Optional    []types.CrossPlatformApp `mapstructure:"optional"`
}

// EnvironmentConfig represents development environment configuration
type EnvironmentConfig struct {
	ProgrammingLanguages []types.CrossPlatformApp `mapstructure:"programming_languages"`
	Fonts                []types.Font             `mapstructure:"fonts"`
	Shell                []types.CrossPlatformApp `mapstructure:"shell"`
}

// DesktopConfig represents desktop environment configuration
type DesktopConfig struct {
	DesktopEnvironments map[string]DesktopEnvConfig `mapstructure:"desktop_environments"`
}

// DesktopEnvConfig represents configuration for a specific desktop environment
type DesktopEnvConfig struct {
	Platforms  []string               `mapstructure:"platforms"`
	Themes     []types.Theme          `mapstructure:"themes"`
	Settings   []types.GnomeSetting   `mapstructure:"settings"`
	Extensions []types.GnomeExtension `mapstructure:"extensions"`
	Dock       []types.DockItem       `mapstructure:"dock"`
}

// SystemConfig represents system-level configuration
type SystemConfig struct {
	Git      []types.GitConfig `mapstructure:"git"`
	SSH      map[string]any    `mapstructure:"ssh"`
	Terminal map[string]any    `mapstructure:"terminal"`
}

// GetAllApps returns all applications from the cross-platform configuration
func (s *CrossPlatformSettings) GetAllApps() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp
	apps = append(apps, s.Applications.Development...)
	apps = append(apps, s.Applications.Databases...)
	apps = append(apps, s.Applications.SystemTools...)
	apps = append(apps, s.Applications.Optional...)
	apps = append(apps, s.Environment.ProgrammingLanguages...)
	apps = append(apps, s.Environment.Shell...)
	return apps
}

// GetDefaultApps returns only apps marked as default
func (s *CrossPlatformSettings) GetDefaultApps() []types.CrossPlatformApp {
	var defaultApps []types.CrossPlatformApp
	for _, app := range s.GetAllApps() {
		if app.Default {
			defaultApps = append(defaultApps, app)
		}
	}
	return defaultApps
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

// LoadCrossPlatformSettings loads the new consolidated configuration structure
func LoadCrossPlatformSettings(homeDir string) (CrossPlatformSettings, error) {
	log.Info("Loading cross-platform settings", "homeDir", homeDir)

	v := viper.New()
	v.SetConfigType("yaml")

	// Paths for default configurations
	defaultConfigPath := filepath.Join(homeDir, ".local/share/devex/config")
	overrideConfigPath := filepath.Join(homeDir, ".devex")

	// Load default configs
	for _, file := range CrossPlatformFiles {
		defaultPath := filepath.Join(defaultConfigPath, file)
		if err := mergeConfigFileIntoViper(v, defaultPath); err != nil {
			log.Warn("Failed to load default config; skipping", "file", file, "error", err)
		}
	}

	// Apply overrides from ~/.devex directory
	for _, file := range CrossPlatformFiles {
		overridePath := filepath.Join(overrideConfigPath, file)
		if exists, err := fs.Stat(overridePath); err == nil && exists != nil {
			log.Info("Applying override", "file", overridePath)
			if err := mergeConfigFileIntoViper(v, overridePath); err != nil {
				log.Warn("Failed to apply override; skipping", "file", overridePath, "error", err)
			}
		}
	}

	// Bind global settings
	v.SetDefault("debug_mode", false)
	v.SetDefault("dry_run", false)
	v.SetDefault("home_dir", homeDir)
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Unmarshal into CrossPlatformSettings struct
	var settings CrossPlatformSettings
	if err := v.Unmarshal(&settings); err != nil {
		log.Error("Failed to unmarshal cross-platform settings", err)
		return CrossPlatformSettings{}, fmt.Errorf("failed to unmarshal cross-platform settings: %w", err)
	}

	log.Info("Cross-platform settings loaded successfully")
	return settings, nil
}

// mergeConfigFileIntoViper reads a YAML file and merges it into the specified Viper instance
func mergeConfigFileIntoViper(v *viper.Viper, path string) error {
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

	// Parse and merge into the specified Viper instance
	subViper := viper.New()
	subViper.SetConfigType("yaml")
	if err := subViper.ReadConfig(bytes.NewReader(data)); err != nil {
		log.Error("Failed to parse YAML file", err, "path", path)
		return fmt.Errorf("failed to parse YAML file %s: %w", path, err)
	}

	// Merge settings into the specified Viper instance
	for k, value := range subViper.AllSettings() {
		v.Set(k, value)
	}

	log.Info("YAML file loaded successfully", "path", path)
	return nil
}
