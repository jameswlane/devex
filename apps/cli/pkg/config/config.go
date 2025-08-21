package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/security"
	"github.com/jameswlane/devex/pkg/types"
)

// Settings Legacy Settings struct for backward compatibility
type Settings struct {
	DebugMode       bool                   `mapstructure:"debug_mode"`
	HomeDir         string                 `mapstructure:"home_dir"`
	Verbose         bool                   `mapstructure:"verbose"`
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
	DebugMode            bool                       `mapstructure:"debug_mode"`
	HomeDir              string                     `mapstructure:"home_dir"`
	Verbose              bool                       `mapstructure:"verbose"`
	Config               map[string]any             `mapstructure:"config"`
	Terminal             TerminalApplicationsConfig `mapstructure:"terminal_applications"`
	TerminalOptional     TerminalOptionalConfig     `mapstructure:"terminal_optional_applications"`
	Desktop              DesktopApplicationsConfig  `mapstructure:"desktop_applications"`
	DesktopOptional      DesktopOptionalConfig      `mapstructure:"desktop_optional_applications"`
	Databases            DatabasesConfig            `mapstructure:"database_applications"`
	ProgrammingLanguages ProgrammingLanguagesConfig `mapstructure:"programming_languages"`
	Fonts                FontsConfig                `mapstructure:"fonts"`
	Shell                ShellConfig                `mapstructure:"shell"`
	Dotfiles             DotfilesConfig             `mapstructure:",inline"`
	DesktopEnvironments  DesktopEnvironmentsConfig  `mapstructure:",inline"`
	Security             SecurityConfigField        `mapstructure:"security"`
}

// ApplicationsConfig represents the application configuration
type ApplicationsConfig struct {
	Development []types.CrossPlatformApp `mapstructure:"development"`
	Databases   []types.CrossPlatformApp `mapstructure:"databases"`
	SystemTools []types.CrossPlatformApp `mapstructure:"system_tools"`
	Optional    []types.CrossPlatformApp `mapstructure:"optional"`
}

// DesktopApplicationsConfig represents desktop application configuration
type DesktopApplicationsConfig struct {
	Productivity []types.CrossPlatformApp `mapstructure:"productivity"`
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

// TerminalApplicationsConfig represents terminal application configuration
type TerminalApplicationsConfig struct {
	Development  []types.CrossPlatformApp `mapstructure:"development"`
	Utilities    []types.CrossPlatformApp `mapstructure:"utilities"`
	Dependencies []types.CrossPlatformApp `mapstructure:"dependencies"`
}

// TerminalOptionalConfig represents optional terminal application configuration
type TerminalOptionalConfig struct {
	Development       []types.CrossPlatformApp `mapstructure:"development"`
	Utilities         []types.CrossPlatformApp `mapstructure:"utilities"`
	SystemMonitoring  []types.CrossPlatformApp `mapstructure:"system_monitoring"`
	TerminalEmulators []types.CrossPlatformApp `mapstructure:"terminal_emulators"`
}

// DesktopOptionalConfig represents optional desktop application configuration
type DesktopOptionalConfig struct {
	Communication []types.CrossPlatformApp `mapstructure:"communication"`
	Productivity  []types.CrossPlatformApp `mapstructure:"productivity"`
	System        []types.CrossPlatformApp `mapstructure:"system"`
	Browsers      []types.CrossPlatformApp `mapstructure:"browsers"`
	Utilities     []types.CrossPlatformApp `mapstructure:"utilities"`
}

// DatabasesConfig represents database configuration
type DatabasesConfig struct {
	Servers   []types.CrossPlatformApp `mapstructure:"servers"`
	Tools     []types.CrossPlatformApp `mapstructure:"tools"`
	Libraries []types.CrossPlatformApp `mapstructure:"libraries"`
}

// ProgrammingLanguagesConfig represents programming language configuration
type ProgrammingLanguagesConfig []types.CrossPlatformApp

// FontsConfig represents font configuration
type FontsConfig []types.Font

// ShellConfig represents shell configuration
type ShellConfig []types.CrossPlatformApp

// DotfilesConfig represents dotfiles and system configuration
type DotfilesConfig struct {
	Git         []types.GitConfig `mapstructure:"git"`
	SSH         map[string]any    `mapstructure:"ssh"`
	Terminal    map[string]any    `mapstructure:"terminal"`
	GlobalTheme string            `mapstructure:"global_theme"`
}

// DesktopEnvironmentsConfig represents desktop environment configurations
type DesktopEnvironmentsConfig struct {
	GNOME   DesktopEnvConfig `mapstructure:",inline"`
	KDE     DesktopEnvConfig `mapstructure:",inline"`
	MacOS   DesktopEnvConfig `mapstructure:",inline"`
	Windows DesktopEnvConfig `mapstructure:",inline"`
}

// SecurityConfigField represents security configuration within the main config
type SecurityConfigField struct {
	Level                int                                `mapstructure:"level"`
	EnterpriseMode       bool                               `mapstructure:"enterprise_mode"`
	WarnOnOverrides      bool                               `mapstructure:"warn_on_overrides"`
	GlobalOverrides      []SecurityOverrideField            `mapstructure:"global_overrides"`
	AppSpecificOverrides map[string][]SecurityOverrideField `mapstructure:"app_overrides"`
}

// SecurityOverrideField represents a security override within the main config
type SecurityOverrideField struct {
	RuleType string `mapstructure:"rule_type"`
	Pattern  string `mapstructure:"pattern"`
	Reason   string `mapstructure:"reason"`
	AppName  string `mapstructure:"app_name,omitempty"`
	WarnUser bool   `mapstructure:"warn_user"`
}

// GetAllApps returns all applications from the cross-platform configuration
func (s *CrossPlatformSettings) GetAllApps() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp

	// Terminal applications
	apps = append(apps, s.Terminal.Development...)
	apps = append(apps, s.Terminal.Utilities...)
	apps = append(apps, s.Terminal.Dependencies...)

	// Terminal optional applications
	apps = append(apps, s.TerminalOptional.Development...)
	apps = append(apps, s.TerminalOptional.Utilities...)
	apps = append(apps, s.TerminalOptional.SystemMonitoring...)
	apps = append(apps, s.TerminalOptional.TerminalEmulators...)

	// Desktop applications
	apps = append(apps, s.Desktop.Productivity...)

	// Desktop optional applications
	apps = append(apps, s.DesktopOptional.Communication...)
	apps = append(apps, s.DesktopOptional.Productivity...)
	apps = append(apps, s.DesktopOptional.System...)
	apps = append(apps, s.DesktopOptional.Browsers...)
	apps = append(apps, s.DesktopOptional.Utilities...)

	// Database applications
	apps = append(apps, s.Databases.Servers...)
	apps = append(apps, s.Databases.Tools...)
	apps = append(apps, s.Databases.Libraries...)

	// Programming languages
	apps = append(apps, s.ProgrammingLanguages...)

	// Shell applications
	apps = append(apps, s.Shell...)

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
	overrideConfigPath := filepath.Join(homeDir, ".devex/config")

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
	viper.SetDefault("home_dir", homeDir)
	viper.AutomaticEnv() // Enable environment variable overrides
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Unmarshal into the Settings struct
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

	// Validate configuration files before loading
	if err := ValidateConfigFiles(homeDir); err != nil {
		log.Warn("Configuration validation failed, continuing with loading", "error", err)
		// Don't fail completely, just warn and continue
	}

	v := viper.New()
	v.SetConfigType("yaml")

	// Paths for environment-aware configuration inheritance
	tempSettings := CrossPlatformSettings{HomeDir: homeDir}
	defaultConfigPath, teamConfigPath, userConfigPath, envDirs := tempSettings.GetConfigDirsWithEnvironment()

	// Load configurations in priority order (lowest to highest):
	// 1. Default configs
	// 2. Default environment configs
	// 3. Team configs
	// 4. Team environment configs
	// 5. User configs
	// 6. User environment configs

	// 1. Load default configs first (lowest priority)
	for _, file := range CrossPlatformFiles {
		defaultPath := filepath.Join(defaultConfigPath, file)
		if exists, _ := fs.Exists(defaultPath); exists {
			if err := mergeConfigFileIntoViper(v, defaultPath); err != nil {
				log.Warn("Failed to load default config; skipping", "file", file, "error", err)
			}
		}
	}

	// 2. Load default environment configs
	for _, file := range CrossPlatformFiles {
		envPath := filepath.Join(envDirs["default"], file)
		if exists, _ := fs.Exists(envPath); exists {
			log.Info("Applying default environment config", "file", envPath, "env", tempSettings.GetEnvironment())
			if err := mergeConfigFileIntoViper(v, envPath); err != nil {
				log.Warn("Failed to apply default environment config; skipping", "file", envPath, "error", err)
			}
		}
	}

	// 3. Apply team configs
	for _, file := range CrossPlatformFiles {
		teamPath := filepath.Join(teamConfigPath, file)
		if exists, _ := fs.Exists(teamPath); exists {
			log.Info("Applying team config", "file", teamPath)
			if err := mergeConfigFileIntoViper(v, teamPath); err != nil {
				log.Warn("Failed to apply team config; skipping", "file", teamPath, "error", err)
			}
		}
	}

	// 4. Apply team environment configs
	for _, file := range CrossPlatformFiles {
		envPath := filepath.Join(envDirs["team"], file)
		if exists, _ := fs.Exists(envPath); exists {
			log.Info("Applying team environment config", "file", envPath, "env", tempSettings.GetEnvironment())
			if err := mergeConfigFileIntoViper(v, envPath); err != nil {
				log.Warn("Failed to apply team environment config; skipping", "file", envPath, "error", err)
			}
		}
	}

	// 5. Apply user configs
	for _, file := range CrossPlatformFiles {
		userPath := filepath.Join(userConfigPath, file)
		if exists, _ := fs.Exists(userPath); exists {
			log.Info("Applying user config", "file", userPath)
			if err := mergeConfigFileIntoViper(v, userPath); err != nil {
				log.Warn("Failed to apply user config; skipping", "file", userPath, "error", err)
			}
		}
	}

	// 6. Apply user environment configs (highest priority)
	for _, file := range CrossPlatformFiles {
		envPath := filepath.Join(envDirs["user"], file)
		if exists, _ := fs.Exists(envPath); exists {
			log.Info("Applying user environment config", "file", envPath, "env", tempSettings.GetEnvironment())
			if err := mergeConfigFileIntoViper(v, envPath); err != nil {
				log.Warn("Failed to apply user environment config; skipping", "file", envPath, "error", err)
			}
		}
	}

	// Bind global settings
	v.SetDefault("debug_mode", false)
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

// ToLegacySettings converts CrossPlatformSettings to the legacy Settings format
func (s *CrossPlatformSettings) ToLegacySettings() Settings {
	legacy := Settings{
		DebugMode: s.DebugMode,
		HomeDir:   s.HomeDir,
		Config:    s.Config,
	}

	// Convert all cross-platform apps to legacy AppConfig
	for _, app := range s.GetAllApps() {
		legacyApp := app.ToLegacyAppConfig()
		legacy.Apps = append(legacy.Apps, legacyApp)
	}

	// Convert fonts config
	legacy.Fonts = append(legacy.Fonts, s.Fonts...)

	// Convert dotfiles configs
	legacy.GitConfig = append(legacy.GitConfig, s.Dotfiles.Git...)

	// Convert desktop environment configs (placeholder for now)
	// TODO: Implement proper desktop environment config conversion
	// when GNOME/KDE/etc configs are properly structured

	return legacy
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

// GetConfigDir returns the default configuration directory path
func (s *CrossPlatformSettings) GetConfigDir() string {
	if s.HomeDir == "" {
		s.HomeDir = defaultHomeDir()
	}
	return filepath.Join(s.HomeDir, ".local/share/devex/config")
}

// GetUserConfigDir returns the user override configuration directory path
func (s *CrossPlatformSettings) GetUserConfigDir() string {
	if s.HomeDir == "" {
		s.HomeDir = defaultHomeDir()
	}
	return filepath.Join(s.HomeDir, ".devex/config")
}

// GetTeamConfigDir returns the team/organization configuration directory path
func (s *CrossPlatformSettings) GetTeamConfigDir() string {
	if s.HomeDir == "" {
		s.HomeDir = defaultHomeDir()
	}
	// Look for team config in environment variable or default location
	if teamDir := os.Getenv("DEVEX_TEAM_CONFIG_DIR"); teamDir != "" {
		return teamDir
	}
	return filepath.Join(s.HomeDir, ".devex/team")
}

// GetConfigDirs returns both default and user config directories for inheritance
func (s *CrossPlatformSettings) GetConfigDirs() (defaultDir, userDir string) {
	return s.GetConfigDir(), s.GetUserConfigDir()
}

// GetAllConfigDirs returns all config directories in inheritance order: default -> team -> user
func (s *CrossPlatformSettings) GetAllConfigDirs() (defaultDir, teamDir, userDir string) {
	return s.GetConfigDir(), s.GetTeamConfigDir(), s.GetUserConfigDir()
}

// GetEnvironment returns the current environment (dev, staging, prod, etc.)
func (s *CrossPlatformSettings) GetEnvironment() string {
	// Check environment variable first
	if env := os.Getenv("DEVEX_ENV"); env != "" {
		return env
	}
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("NODE_ENV"); env != "" {
		return env
	}
	// Default to development
	return "dev"
}

// GetConfigDirsWithEnvironment returns config directories with environment-specific paths
func (s *CrossPlatformSettings) GetConfigDirsWithEnvironment() (defaultDir, teamDir, userDir string, envDirs map[string]string) {
	env := s.GetEnvironment()
	defaultDir = s.GetConfigDir()
	teamDir = s.GetTeamConfigDir()
	userDir = s.GetUserConfigDir()

	envDirs = map[string]string{
		"default": filepath.Join(defaultDir, "environments", env),
		"team":    filepath.Join(teamDir, "environments", env),
		"user":    filepath.Join(userDir, "environments", env),
	}

	return defaultDir, teamDir, userDir, envDirs
}

// GetApplicationByName returns an application configuration by name
func (s *CrossPlatformSettings) GetApplicationByName(name string) (*types.AppConfig, error) {
	// Search through all application categories
	allApps := s.GetAllApps()
	for _, app := range allApps {
		legacyApp := app.ToLegacyAppConfig()
		if legacyApp.Name == name {
			return &legacyApp, nil
		}
	}
	return nil, fmt.Errorf("application '%s' not found", name)
}

// GetApplications returns all applications as legacy AppConfig slice
func (s *CrossPlatformSettings) GetApplications() []types.AppConfig {
	allApps := s.GetAllApps()
	apps := make([]types.AppConfig, 0, len(allApps))
	for _, app := range allApps {
		apps = append(apps, app.ToLegacyAppConfig())
	}
	return apps
}

// GetEnvironmentSettings returns environment configuration
func (s *CrossPlatformSettings) GetEnvironmentSettings() interface{} {
	return map[string]interface{}{
		"programming_languages": s.ProgrammingLanguages,
		"fonts":                 s.Fonts,
		"shell":                 s.Shell,
	}
}

// GetSystemSettings returns system configuration
func (s *CrossPlatformSettings) GetSystemSettings() interface{} {
	return map[string]interface{}{
		"git":      s.Dotfiles.Git,
		"ssh":      s.Dotfiles.SSH,
		"terminal": s.Dotfiles.Terminal,
	}
}

// GetDesktopSettings returns desktop configuration
func (s *CrossPlatformSettings) GetDesktopSettings() interface{} {
	if len(s.DesktopEnvironments.GNOME.Themes) == 0 &&
		len(s.DesktopEnvironments.GNOME.Settings) == 0 &&
		len(s.DesktopEnvironments.GNOME.Extensions) == 0 {
		return nil
	}

	return map[string]interface{}{
		"gnome": s.DesktopEnvironments.GNOME,
		"kde":   s.DesktopEnvironments.KDE,
	}
}

// defaultHomeDir returns the default home directory
func defaultHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp" // Fallback
	}
	return homeDir
}

// ToSecurityConfig converts SecurityConfigField to security.SecurityConfig
func (s *SecurityConfigField) ToSecurityConfig() *security.SecurityConfig {
	securityConfig := &security.SecurityConfig{
		Level:                security.SecurityLevel(s.Level),
		EnterpriseMode:       s.EnterpriseMode,
		WarnOnOverrides:      s.WarnOnOverrides,
		GlobalOverrides:      make([]security.SecurityOverride, len(s.GlobalOverrides)),
		AppSpecificOverrides: make(map[string][]security.SecurityOverride),
	}

	// Convert global overrides
	for i, override := range s.GlobalOverrides {
		securityConfig.GlobalOverrides[i] = security.SecurityOverride{
			RuleType: security.SecurityRuleType(override.RuleType),
			Pattern:  override.Pattern,
			Reason:   override.Reason,
			AppName:  override.AppName,
			WarnUser: override.WarnUser,
		}
	}

	// Convert app-specific overrides
	for appName, overrides := range s.AppSpecificOverrides {
		securityOverrides := make([]security.SecurityOverride, len(overrides))
		for i, override := range overrides {
			securityOverrides[i] = security.SecurityOverride{
				RuleType: security.SecurityRuleType(override.RuleType),
				Pattern:  override.Pattern,
				Reason:   override.Reason,
				AppName:  override.AppName,
				WarnUser: override.WarnUser,
			}
		}
		securityConfig.AppSpecificOverrides[appName] = securityOverrides
	}

	return securityConfig
}

// GetSecurityConfig returns the security configuration from CrossPlatformSettings
func (s *CrossPlatformSettings) GetSecurityConfig() *security.SecurityConfig {
	return s.Security.ToSecurityConfig()
}

// LoadSecurityConfigForSettings loads security configuration and applies it to settings
func (s *CrossPlatformSettings) LoadSecurityConfigForSettings() (*security.SecurityConfig, error) {
	// Try to get from embedded config first
	securityConfig := s.GetSecurityConfig()

	// If no embedded config or empty, try loading from separate file
	if securityConfig.Level == 0 && len(securityConfig.GlobalOverrides) == 0 {
		return security.LoadSecurityConfigFromDefaults(s.HomeDir)
	}

	return securityConfig, nil
}

// GetCommandValidator returns a configured command validator
func (s *CrossPlatformSettings) GetCommandValidator() (*security.CommandValidator, error) {
	securityConfig, err := s.LoadSecurityConfigForSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to load security config: %w", err)
	}

	return security.NewCommandValidatorWithConfig(securityConfig.Level, securityConfig), nil
}
