package config

import (
	"fmt"
	"reflect"
	"strings"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
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

	_ = v.validateEffectiveDirectory(configPath, map[string]string{})
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

// validateCrossPlatformApp validates a cross-platform application structure
func (v *ConfigValidator) validateCrossPlatformApp(app types.CrossPlatformApp, filename string) {
	hasAnyPlatform := false
	if !reflect.DeepEqual(app.AllPlatforms, types.OSConfig{}) {
		hasAnyPlatform = true
		v.validateOSConfig(app.AllPlatforms, "all_platforms", filename)
	}
	if !reflect.DeepEqual(app.Linux, types.OSConfig{}) {
		hasAnyPlatform = true
		v.validateOSConfig(app.Linux, "linux", filename)
	}
	if !reflect.DeepEqual(app.MacOS, types.OSConfig{}) {
		hasAnyPlatform = true
		v.validateOSConfig(app.MacOS, "macos", filename)
	}
	if !reflect.DeepEqual(app.Windows, types.OSConfig{}) {
		hasAnyPlatform = true
		v.validateOSConfig(app.Windows, "windows", filename)
	}
	if !hasAnyPlatform {
		v.addError(filename, "platforms", "No platform configurations found", nil)
	}
}

// validateOSConfig validates an OS-specific configuration
func (v *ConfigValidator) validateOSConfig(osConfig types.OSConfig, platform, filename string) {
	if err := osConfig.ValidateShellActions(); err != nil {
		v.addError(filename, platform+".post_install", err.Error(), nil)
	}
	for i, alternative := range osConfig.Alternatives {
		v.validateOSConfig(alternative, fmt.Sprintf("%s.alternatives[%d]", platform, i), filename)
	}
	v.validateOSChannels(osConfig, platform, filename)
	if len(osConfig.Packages) > 0 || len(osConfig.Options) > 0 {
		if _, err := types.ResolvePackageInstall(osConfig.InstallMethod, osConfig.InstallCommand, osConfig.Packages, osConfig.Options); err != nil {
			v.addError(filename, platform+".packages", err.Error(), err)
		}
	}
	if osConfig.InstallMethod == "" {
		v.addError(filename, platform+".install_method", "Install method is required", nil)
	}

	if osConfig.InstallScope != "" && osConfig.InstallScope != "user" {
		v.addError(filename, platform+".install_scope", fmt.Sprintf("Unsupported install_scope %q (allowed: user)", osConfig.InstallScope), nil)
	}

	if osConfig.InstallCommand == "" && len(osConfig.Packages) == 0 && !osConfig.IsHookDrivenCurlpipe() && ((osConfig.InstallMethod != "curlpipe" && osConfig.InstallMethod != "appimage") || osConfig.DownloadURL == "") {
		v.addError(filename, platform+".install_command", "Install command is required", nil)
	}
}

// validateOSChannels enforces unique channel names within one platform block
// and ensures a non-official primary is never paired with an official alternative.
func (v *ConfigValidator) validateOSChannels(osConfig types.OSConfig, platform, filename string) {
	if strings.EqualFold(strings.TrimSpace(osConfig.Channel), types.ChannelOfficial) {
		v.addError(filename, platform+".channel", fmt.Sprintf("Channel name %q is reserved", types.ChannelOfficial), nil)
	}
	if len(osConfig.Alternatives) == 0 {
		return
	}

	seen := map[string]string{}
	if primaryName := strings.ToLower(osConfig.ChannelName()); primaryName != "" {
		seen[primaryName] = platform
	}

	for i, alt := range osConfig.Alternatives {
		altField := fmt.Sprintf("%s.alternatives[%d]", platform, i)
		if altName := strings.ToLower(alt.ChannelName()); altName != "" {
			if prev, duplicate := seen[altName]; duplicate {
				v.addError(filename, altField+".channel", fmt.Sprintf("Duplicate channel name %q (already used by %s); add an explicit channel key to distinguish channels sharing an install_method", alt.ChannelName(), prev), nil)
			} else {
				seen[altName] = altField
			}
		}
		if !osConfig.OfficialSupport && alt.OfficialSupport {
			v.addError(filename, platform+".official_support", fmt.Sprintf("Primary channel %q has official_support: false while alternative %q (%s) has official_support: true; make the official channel primary", osConfig.ChannelName(), alt.ChannelName(), altField), nil)
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

// ValidateDirectoryBasedConfig is a convenience function for validating directory-based configs
func ValidateDirectoryBasedConfig(homeDir string, strict bool) error {
	configPath := devexpaths.ForHome(homeDir).Defaults()

	validator := NewConfigValidator(homeDir, strict)
	return validator.ValidateDirectoryStructure(configPath)
}
