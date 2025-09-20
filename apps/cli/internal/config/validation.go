package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/viper"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	File    string
	Field   string
	Message string
	Err     error
}

func (e ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s in %s.%s: %v", e.Message, e.File, e.Field, e.Err)
	}
	return fmt.Sprintf("%s in %s.%s", e.Message, e.File, e.Field)
}

// ConfigValidator handles validation of directory-based configuration files
type ConfigValidator struct {
	homeDir  string
	strict   bool
	errors   []ValidationError
	warnings []ValidationError
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator(homeDir string, strict bool) *ConfigValidator {
	return &ConfigValidator{
		homeDir:  homeDir,
		strict:   strict,
		errors:   make([]ValidationError, 0),
		warnings: make([]ValidationError, 0),
	}
}

// ValidateDirectoryStructure validates the entire directory-based configuration
func (v *ConfigValidator) ValidateDirectoryStructure(configPath string) error {
	log.Info("Validating directory-based configuration", "path", configPath)

	// Validate each directory in the processing order
	for _, dirName := range ConfigDirectories {
		dirPath := filepath.Join(configPath, dirName)

		if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
			if v.strict && dirName == "system" {
				// System directory is required in strict mode
				v.addError(dirName, "", "Required directory missing", err)
			} else {
				v.addWarning(dirName, "", "Optional directory missing", err)
			}
			continue
		}

		if err := v.validateDirectory(dirPath, dirName); err != nil {
			log.Warn("Directory validation failed", "directory", dirName, "error", err)
		}
	}

	// Report results
	if len(v.errors) > 0 {
		return fmt.Errorf("configuration validation failed with %d error(s)", len(v.errors))
	}

	if len(v.warnings) > 0 {
		log.Warn("Configuration validation completed with warnings", "warnings", len(v.warnings))
	} else {
		log.Info("Configuration validation passed")
	}

	return nil
}

// validateDirectory validates all YAML files in a specific directory
func (v *ConfigValidator) validateDirectory(dirPath, dirName string) error {
	files, err := getDirectoryFiles(dirPath)
	if err != nil {
		return err
	}

	for _, filename := range files {
		filePath := filepath.Join(dirPath, filename)
		if err := v.validateConfigFile(filePath, dirName, filename); err != nil {
			log.Debug("File validation failed", "file", filename, "error", err)
		}
	}

	return nil
}

// validateConfigFile validates a single configuration file
func (v *ConfigValidator) validateConfigFile(filePath, dirName, filename string) error {
	// Load file into temporary Viper instance
	fileViper, err := loadYamlFileToViper(filePath)
	if err != nil {
		v.addError(filename, "", "Failed to load YAML file", err)
		return err
	}

	// Validate based on directory type
	switch dirName {
	case "applications":
		return v.validateApplicationConfig(fileViper, filename)
	case "environments":
		return v.validateEnvironmentConfig(fileViper, filename)
	case "system":
		return v.validateSystemConfig(fileViper, filename)
	case "desktop":
		return v.validateDesktopConfig(fileViper, filename)
	default:
		v.addWarning(filename, "", "Unknown directory type, skipping validation", nil)
	}

	return nil
}

// validateApplicationConfig validates application configuration files
func (v *ConfigValidator) validateApplicationConfig(fileViper *viper.Viper, filename string) error {
	settings := fileViper.AllSettings()

	// Check for required fields for application configs
	requiredFields := []string{"name", "description"}
	for _, field := range requiredFields {
		if !fileViper.IsSet(field) {
			v.addError(filename, field, "Required field missing", nil)
		}
	}

	// Validate application structure if it looks like a cross-platform app
	if name, ok := settings["name"].(string); ok {
		app := types.CrossPlatformApp{}
		if err := fileViper.Unmarshal(&app); err != nil {
			v.addError(filename, "structure", "Invalid application structure", err)
		} else {
			v.validateCrossPlatformApp(app, filename)
		}

		// Validate name format
		if strings.TrimSpace(name) == "" {
			v.addError(filename, "name", "Application name cannot be empty", nil)
		}
	}

	return nil
}

// validateEnvironmentConfig validates environment configuration files
func (v *ConfigValidator) validateEnvironmentConfig(fileViper *viper.Viper, filename string) error {
	// For programming languages, fonts, etc.
	if fileViper.IsSet("install_method") {
		installMethod := fileViper.GetString("install_method")
		if installMethod == "" {
			v.addError(filename, "install_method", "Install method cannot be empty", nil)
		}

		// Validate install method is supported
		validMethods := []string{"apt", "dnf", "pacman", "brew", "winget", "mise", "pip", "cargo", "npm"}
		if !contains(validMethods, installMethod) {
			v.addWarning(filename, "install_method", fmt.Sprintf("Unknown install method: %s", installMethod), nil)
		}
	}

	return nil
}

// validateSystemConfig validates system configuration files
func (v *ConfigValidator) validateSystemConfig(fileViper *viper.Viper, filename string) error {
	// Basic validation for system configs (git, ssh, terminal, etc.)
	settings := fileViper.AllSettings()

	// Ensure settings aren't empty
	if len(settings) == 0 {
		v.addWarning(filename, "", "System configuration file is empty", nil)
	}

	return nil
}

// validateDesktopConfig validates desktop environment configuration files
func (v *ConfigValidator) validateDesktopConfig(fileViper *viper.Viper, filename string) error {
	// Validate desktop environment configs (GNOME, KDE, etc.)
	if fileViper.IsSet("desktop_environment") {
		de := fileViper.GetString("desktop_environment")
		validDEs := []string{"gnome", "kde", "xfce", "i3", "macos", "windows"}
		if !contains(validDEs, strings.ToLower(de)) {
			v.addWarning(filename, "desktop_environment", fmt.Sprintf("Unknown desktop environment: %s", de), nil)
		}
	}

	return nil
}

// validateCrossPlatformApp validates a cross-platform application structure
func (v *ConfigValidator) validateCrossPlatformApp(app types.CrossPlatformApp, filename string) {
	// Validate platform configurations exist
	hasAnyPlatform := false

	if app.Linux.InstallMethod != "" {
		hasAnyPlatform = true
		v.validateOSConfig(app.Linux, "linux", filename)
	}

	if app.MacOS.InstallMethod != "" {
		hasAnyPlatform = true
		v.validateOSConfig(app.MacOS, "macos", filename)
	}

	if app.Windows.InstallMethod != "" {
		hasAnyPlatform = true
		v.validateOSConfig(app.Windows, "windows", filename)
	}

	if !hasAnyPlatform {
		v.addError(filename, "platforms", "No platform configurations found", nil)
	}

	// Validate category if present
	if app.Category != "" {
		validCategories := []string{
			"Development Tools", "Databases", "System Monitoring", "Utilities",
			"Programming Languages", "Text Editors", "Terminal Emulators",
			"File Sharing", "Optional Apps", "System Utilities",
		}
		if !contains(validCategories, app.Category) {
			v.addWarning(filename, "category", fmt.Sprintf("Unknown category: %s", app.Category), nil)
		}
	}
}

// validateOSConfig validates an OS-specific configuration
func (v *ConfigValidator) validateOSConfig(osConfig types.OSConfig, platform, filename string) {
	if osConfig.InstallMethod == "" {
		v.addError(filename, platform+".install_method", "Install method is required", nil)
	}

	if osConfig.InstallCommand == "" {
		v.addError(filename, platform+".install_command", "Install command is required", nil)
	}

	// Validate platform-specific install methods
	switch platform {
	case "linux":
		validMethods := []string{"apt", "dnf", "pacman", "zypper", "emerge", "snap", "flatpak", "appimage"}
		if !contains(validMethods, osConfig.InstallMethod) {
			v.addWarning(filename, platform+".install_method",
				fmt.Sprintf("Uncommon Linux install method: %s", osConfig.InstallMethod), nil)
		}
	case "macos":
		validMethods := []string{"brew", "mas", "dmg", "pkg"}
		if !contains(validMethods, osConfig.InstallMethod) {
			v.addWarning(filename, platform+".install_method",
				fmt.Sprintf("Uncommon macOS install method: %s", osConfig.InstallMethod), nil)
		}
	case "windows":
		validMethods := []string{"winget", "chocolatey", "scoop", "msi", "exe"}
		if !contains(validMethods, osConfig.InstallMethod) {
			v.addWarning(filename, platform+".install_method",
				fmt.Sprintf("Uncommon Windows install method: %s", osConfig.InstallMethod), nil)
		}
	}
}

// addError adds a validation error
func (v *ConfigValidator) addError(file, field, message string, err error) {
	v.errors = append(v.errors, ValidationError{
		File:    file,
		Field:   field,
		Message: message,
		Err:     err,
	})
}

// addWarning adds a validation warning
func (v *ConfigValidator) addWarning(file, field, message string, err error) {
	v.warnings = append(v.warnings, ValidationError{
		File:    file,
		Field:   field,
		Message: message,
		Err:     err,
	})
}

// GetErrors returns all validation errors
func (v *ConfigValidator) GetErrors() []ValidationError {
	return v.errors
}

// GetWarnings returns all validation warnings
func (v *ConfigValidator) GetWarnings() []ValidationError {
	return v.warnings
}

// GetErrorCount returns the number of validation errors
func (v *ConfigValidator) GetErrorCount() int {
	return len(v.errors)
}

// GetWarningCount returns the number of validation warnings
func (v *ConfigValidator) GetWarningCount() int {
	return len(v.warnings)
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

// ValidateDirectoryBasedConfig is a convenience function for validating directory-based configs
func ValidateDirectoryBasedConfig(homeDir string, strict bool) error {
	configPath := filepath.Join(homeDir, ".local/share/devex/config")

	validator := NewConfigValidator(homeDir, strict)
	return validator.ValidateDirectoryStructure(configPath)
}
