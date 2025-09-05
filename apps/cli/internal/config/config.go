package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// Removed legacy Settings struct - no longer needed without backward compatibility

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

// loadDirectoryConfigs loads configuration files from directories in the specified order
// Directories are processed in the order defined by ConfigDirectories
// Within each directory, YAML files are processed alphabetically
func loadDirectoryConfigs(configPath string) error {
	log.Info("Loading directory-based configuration", "path", configPath)

	// Process directories in the specified order
	for _, dirName := range ConfigDirectories {
		dirPath := filepath.Join(configPath, dirName)

		// Check if directory exists
		if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
			log.Debug("Skipping directory (not found or not a directory)", "dir", dirName)
			continue
		}

		log.Info("Processing config directory", "directory", dirName)

		if err := loadDirectoryAlphabetically(dirPath, dirName); err != nil {
			log.Warn("Error processing directory", "directory", dirName, "error", err)
		}
	}

	return nil
}

// loadDirectoryAlphabetically loads YAML files from a directory in alphabetical order
// This ensures consistent loading order and enables prefix-based ordering
// Uses parallel loading for improved performance with many files
func loadDirectoryAlphabetically(dirPath, dirName string) error {
	// Get all YAML files and sort them alphabetically
	files, err := getDirectoryFiles(dirPath)
	if err != nil {
		return fmt.Errorf("failed to get directory files: %w", err)
	}

	if len(files) == 0 {
		log.Debug("No YAML files found in directory", "directory", dirName)
		return nil
	}

	// For small numbers of files, use sequential loading to maintain order
	if len(files) <= 5 {
		return loadFilesSequentially(files, dirPath, dirName)
	}

	// For larger numbers of files, use parallel loading with ordered merge
	return loadFilesInParallel(files, dirPath, dirName)
}

// loadFilesSequentially loads files one by one in order
func loadFilesSequentially(files []string, dirPath, dirName string) error {
	for _, filename := range files {
		if !isValidFilename(filename) {
			log.Warn("Skipping file with invalid name", "file", filename, "directory", dirName)
			continue
		}

		filePath := filepath.Join(dirPath, filename)
		log.Debug("Loading config file", "file", filename, "directory", dirName)

		if err := loadYamlFileIntoViperWithPrefix(filePath, dirName); err != nil {
			log.Warn("Failed to load config file", "file", filename, "error", err)
			// Continue loading other files even if one fails
		}
	}
	return nil
}

// fileLoadResult represents the result of loading a single file
type fileLoadResult struct {
	filename string
	settings map[string]interface{}
	err      error
}

// loadFilesInParallel loads files concurrently with goroutine limiting but merges results in alphabetical order
func loadFilesInParallel(files []string, dirPath, dirName string) error {
	resultsChan := make(chan fileLoadResult, len(files))
	var wg sync.WaitGroup

	// Semaphore channel to limit concurrent goroutines for security
	semaphore := make(chan struct{}, maxConcurrentLoaders)

	// Start goroutines for parallel file loading with concurrency limit
	for _, filename := range files {
		if !isValidFilename(filename) {
			log.Warn("Skipping file with invalid name", "file", filename, "directory", dirName)
			continue
		}

		wg.Add(1)
		go func(fname string) {
			defer wg.Done()

			// Acquire semaphore (blocks if at limit)
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release semaphore

			loadSingleFileForParallel(fname, dirPath, dirName, resultsChan)
		}(filename)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results in a map for ordered processing
	resultsMap := make(map[string]fileLoadResult)
	for result := range resultsChan {
		resultsMap[result.filename] = result
	}

	// Process results in alphabetical order to maintain consistency
	for _, filename := range files {
		result, exists := resultsMap[filename]
		if !exists {
			continue // File was skipped due to invalid name
		}

		if result.err != nil {
			log.Warn("Failed to load config file", "file", filename, "error", result.err)
			continue
		}

		// Merge settings into main viper in order
		for key, value := range result.settings {
			fullKey := fmt.Sprintf("%s.%s.%s", dirName, sanitizeFilenameForKey(filename), key)
			viper.Set(fullKey, value)
		}
	}

	return nil
}

// loadSingleFileForParallel loads a single file and sends the result to a channel
func loadSingleFileForParallel(filename, dirPath, dirName string, resultsChan chan<- fileLoadResult) {
	filePath := filepath.Join(dirPath, filename)
	log.Debug("Loading config file (parallel)", "file", filename, "directory", dirName)

	fileViper, err := loadYamlFileToViper(filePath)
	if err != nil {
		resultsChan <- fileLoadResult{filename: filename, err: err}
		return
	}

	resultsChan <- fileLoadResult{
		filename: filename,
		settings: fileViper.AllSettings(),
		err:      nil,
	}
}

// loadYamlFileIntoViperWithPrefix loads a YAML file into Viper with a directory-based prefix
// Uses a utility function for consistent file loading and proper resource management
func loadYamlFileIntoViperWithPrefix(filePath, dirPrefix string) error {
	// Validate file path
	if !isValidConfigPath(filePath) {
		return fmt.Errorf("invalid file path: %s", filePath)
	}

	// Check cache to avoid unnecessary reloading
	if shouldReload, err := globalConfigCache.shouldReloadFile(filePath); err != nil {
		return fmt.Errorf("failed to check file status: %w", err)
	} else if !shouldReload {
		log.Debug("Skipping file load, not modified", "path", filePath)
		return nil
	}

	// Use utility function for consistent file loading
	fileViper, err := loadYamlFileToViper(filePath)
	if err != nil {
		return err
	}

	// Get the filename without extension for use as key
	filename := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	// Sanitize filename for use in keys
	sanitizedFilename := sanitizeFilenameForKey(filename)

	// Merge into main viper with directory-based structure
	allSettings := fileViper.AllSettings()
	for key, value := range allSettings {
		// Create hierarchical key: directory.filename.key
		fullKey := fmt.Sprintf("%s.%s.%s", dirPrefix, sanitizedFilename, key)
		viper.Set(fullKey, value)
	}

	return nil
}

// getDirectoryFiles returns all YAML files in a directory sorted alphabetically
// This enables prefix-based ordering (e.g., "00-priority-app.yaml" loads first)
func getDirectoryFiles(dirPath string) ([]string, error) {
	// Validate directory path to prevent traversal attacks
	if !isValidConfigPath(dirPath) {
		return nil, fmt.Errorf("invalid directory path: %s", dirPath)
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	yamlFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !isYamlFile(filename) {
			continue
		}

		if !isValidFilename(filename) {
			log.Warn("Skipping file with invalid name", "file", filename)
			continue
		}

		yamlFiles = append(yamlFiles, filename)
	}

	// Sort alphabetically - this enables prefix-based ordering
	sort.Strings(yamlFiles)

	return yamlFiles, nil
}

// getDirectoryFilesRecursive returns all YAML files in a directory tree with depth limiting
// Uses filepath.WalkDir (Go 1.16+) for better performance with large directory structures
func getDirectoryFilesRecursive(rootPath string, maxDepth int) ([]string, error) {
	// Validate directory path to prevent traversal attacks
	if !isValidConfigPath(rootPath) {
		return nil, fmt.Errorf("invalid directory path: %s", rootPath)
	}

	if maxDepth < 0 {
		return nil, fmt.Errorf("maxDepth must be non-negative, got %d", maxDepth)
	}

	var yamlFiles []string
	rootPath = filepath.Clean(rootPath)

	// Check if root directory exists first
	if _, err := os.Stat(rootPath); err != nil {
		return nil, fmt.Errorf("failed to access directory %s: %w", rootPath, err)
	}

	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Log but continue walking for non-critical errors
			log.Debug("Error walking directory", "path", path, "error", err)
			return nil
		}

		// Calculate depth relative to root
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			log.Debug("Failed to calculate relative path", "path", path, "error", err)
			return nil
		}

		// Count directory separators in relative path to determine depth
		depth := 0
		if relPath != "." {
			depth = strings.Count(relPath, string(filepath.Separator))
			// Both files and directories use separator count for depth calculation
			// Files are at the same depth as their parent directory
		}

		// Check depth limit for directories
		if d.IsDir() && depth > maxDepth {
			return filepath.SkipDir
		}

		// Skip directories (we only want files)
		if d.IsDir() {
			return nil
		}

		// Check depth limit for files (file depth = parent directory depth)
		if depth > maxDepth {
			return nil
		}

		filename := d.Name()
		if !isYamlFile(filename) {
			return nil
		}

		if !isValidFilename(filename) {
			log.Warn("Skipping file with invalid name", "file", filename)
			return nil
		}

		// Store relative path for consistency with getDirectoryFiles
		yamlFiles = append(yamlFiles, relPath)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", rootPath, err)
	}

	// Sort alphabetically for consistent ordering
	sort.Strings(yamlFiles)

	return yamlFiles, nil
}

// Removed LoadSettings function - replaced by LoadCrossPlatformSettings

func loadYamlFileIntoViper(path string) error {
	log.Info("Loading YAML file into Viper", "path", path)

	// Validate path
	if !isValidConfigPath(path) {
		return fmt.Errorf("invalid config path: %s", path)
	}

	// Use utility function for consistent loading
	subViper, err := loadYamlFileToViper(path)
	if err != nil {
		// Check if file exists for better error reporting
		if exists, checkErr := fs.Exists(path); checkErr == nil && !exists {
			log.Warn("Config file not found", "path", path)
			return nil // Don't fail for missing optional files
		}
		return err
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
	if err := ValidateDirectoryBasedConfig(homeDir, false); err != nil {
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

// Removed ToLegacySettings method - no longer needed without backward compatibility

// mergeConfigFileIntoViper reads a YAML file and merges it into the specified Viper instance
// Uses the utility function for consistent loading behavior
func mergeConfigFileIntoViper(v *viper.Viper, path string) error {
	log.Info("Loading YAML file into Viper", "path", path)

	// Validate path before loading
	if !isValidConfigPath(path) {
		return fmt.Errorf("invalid config path: %s", path)
	}

	// Check if file needs reloading (performance optimization)
	if shouldReload, err := globalConfigCache.shouldReloadFile(path); err != nil {
		return fmt.Errorf("failed to check file status: %w", err)
	} else if !shouldReload {
		log.Debug("Skipping file load, not modified", "path", path)
		return nil
	}

	_, err := fs.ReadFile(path)
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
	subViper, err := loadYamlFileToViper(path)
	if err != nil {
		return err
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

// ValidateApplicationsConfig validates a legacy applications configuration map
// This function is maintained for backward compatibility with existing tests
func ValidateApplicationsConfig(configMap map[string]interface{}) error {
	if configMap == nil {
		return fmt.Errorf("configuration map is nil")
	}

	// Basic validation for legacy format
	if len(configMap) == 0 {
		return fmt.Errorf("configuration map is empty")
	}

	// Check for applications section
	applications, exists := configMap["applications"]
	if !exists {
		return fmt.Errorf("missing required section: applications")
	}

	// Verify applications is a map (handle both string and interface{} keys from YAML)
	var appsMap map[string]interface{}
	switch v := applications.(type) {
	case map[string]interface{}:
		appsMap = v
	case map[interface{}]interface{}:
		// Convert interface{} keys to string keys
		appsMap = make(map[string]interface{})
		for k, val := range v {
			if keyStr, ok := k.(string); ok {
				appsMap[keyStr] = val
			}
		}
	default:
		return fmt.Errorf("applications section must be a map")
	}

	// Check for required subsections
	requiredSections := []string{"databases"}
	for _, section := range requiredSections {
		if _, exists := appsMap[section]; !exists {
			return fmt.Errorf("missing required section: applications.%s", section)
		}
	}

	return nil
}

// Security and validation utility functions

// filenameValidationRegex matches valid configuration filenames
// Allows alphanumeric, hyphens, underscores, and dots, but prevents path traversal
var filenameValidationRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*\.(yaml|yml)$`)

// pathTraversalRegex detects potential path traversal attempts
var pathTraversalRegex = regexp.MustCompile(`\.\.[\\/]`)

// isValidFilename validates that a filename is safe to use
// Prevents path traversal and ensures reasonable naming conventions
func isValidFilename(filename string) bool {
	if filename == "" || len(filename) > 255 {
		return false
	}

	// Check for path traversal attempts
	if pathTraversalRegex.MatchString(filename) {
		return false
	}

	// Validate against allowed pattern
	return filenameValidationRegex.MatchString(filename)
}

// isValidConfigPath validates that a file/directory path is within allowed config directories
func isValidConfigPath(path string) bool {
	if path == "" {
		return false
	}

	// Check for path traversal attempts
	if pathTraversalRegex.MatchString(path) {
		return false
	}

	// Ensure path is absolute or within reasonable bounds
	cleanPath := filepath.Clean(path)
	return !strings.Contains(cleanPath, "..")
}

// isYamlFile checks if a filename is a YAML file
func isYamlFile(filename string) bool {
	return strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml")
}

// sanitizeFilenameForKey converts a filename to a safe key format
// Removes potentially problematic characters and ensures consistent naming
func sanitizeFilenameForKey(filename string) string {
	// Remove file extension
	key := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Replace any remaining problematic characters with underscores
	key = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(key, "_")

	// Ensure key doesn't start with non-letter
	if len(key) > 0 && !regexp.MustCompile(`^[a-zA-Z]`).MatchString(key) {
		key = "config_" + key
	}

	return key
}

// File loading utility functions

// loadYamlFileToViper loads a YAML file into a new Viper instance with size limits
// Centralizes file loading logic and provides consistent error handling
func loadYamlFileToViper(filePath string) (*viper.Viper, error) {
	// Check file size before reading for DoS prevention
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if fileInfo.Size() > maxFileSize {
		return nil, fmt.Errorf("file %s exceeds maximum size limit (%d bytes)", filePath, maxFileSize)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Validate YAML content isn't empty or malformed
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, fmt.Errorf("config file %s is empty", filePath)
	}

	// Create a new Viper instance for this file
	fileViper := viper.New()
	fileViper.SetConfigType("yaml")

	if err := fileViper.ReadConfig(bytes.NewReader(content)); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", filePath, err)
	}

	return fileViper, nil
}

// Config file caching for performance optimization

// Security and performance constants for cache management
const (
	// Maximum number of files to cache to prevent unbounded memory growth
	maxCachedFiles = 10000
	// Cache TTL - entries older than this are considered stale
	cacheTTL = 1 * time.Hour
	// Maximum file size to process (10MB)
	maxFileSize = 10 * 1024 * 1024
	// Maximum concurrent goroutines for parallel loading
	maxConcurrentLoaders = 10
)

type configFileInfo struct {
	path     string
	modTime  time.Time
	size     int64
	cachedAt time.Time // When this entry was cached
}

type configCache struct {
	mu    sync.RWMutex
	files map[string]configFileInfo
}

var globalConfigCache = &configCache{
	files: make(map[string]configFileInfo),
}

// shouldReloadFile checks if a config file needs to be reloaded based on modification time and TTL
func (c *configCache) shouldReloadFile(filePath string) (bool, error) {
	c.mu.RLock()
	cachedInfo, exists := c.files[filePath]
	c.mu.RUnlock()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		// File doesn't exist or can't be accessed
		// Remove from cache if it was previously cached
		if exists {
			c.mu.Lock()
			delete(c.files, filePath)
			c.mu.Unlock()
		}
		return false, err
	}

	// Check file size limit for security
	if fileInfo.Size() > maxFileSize {
		return false, fmt.Errorf("file %s exceeds maximum size limit (%d bytes)", filePath, maxFileSize)
	}

	if !exists {
		// File not in cache, needs loading
		c.updateFileInfo(filePath, fileInfo)
		return true, nil
	}

	// Check TTL - if cache entry is too old, force reload
	now := time.Now()
	if now.Sub(cachedInfo.cachedAt) > cacheTTL {
		c.updateFileInfo(filePath, fileInfo)
		return true, nil
	}

	// Check if file has been modified or size changed
	// Use >= for time comparison to handle low-resolution timestamps
	if fileInfo.ModTime().After(cachedInfo.modTime) ||
		fileInfo.Size() != cachedInfo.size ||
		fileInfo.ModTime().Equal(cachedInfo.modTime) && fileInfo.Size() != cachedInfo.size {
		c.updateFileInfo(filePath, fileInfo)
		return true, nil
	}

	return false, nil
}

// updateFileInfo updates the cached file information with cache size limits
func (c *configCache) updateFileInfo(filePath string, fileInfo os.FileInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce cache size limit for security
	if len(c.files) >= maxCachedFiles {
		c.evictOldestEntry()
	}

	c.files[filePath] = configFileInfo{
		path:     filePath,
		modTime:  fileInfo.ModTime(),
		size:     fileInfo.Size(),
		cachedAt: time.Now(),
	}
}

// evictOldestEntry removes the oldest cache entry when cache is full
func (c *configCache) evictOldestEntry() {
	if len(c.files) == 0 {
		return
	}

	var oldestPath string
	var oldestTime time.Time
	first := true

	for path, info := range c.files {
		if first || info.cachedAt.Before(oldestTime) {
			oldestPath = path
			oldestTime = info.cachedAt
			first = false
		}
	}

	if oldestPath != "" {
		delete(c.files, oldestPath)
	}
}

// clearCache clears the config file cache (useful for testing)
func (c *configCache) clearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files = make(map[string]configFileInfo)
}
