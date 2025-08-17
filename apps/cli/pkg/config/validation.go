package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/log"
	"gopkg.in/yaml.v2"
)

// ValidateConfigFiles performs basic validation on the configuration files
func ValidateConfigFiles(homeDir string) error {
	log.Info("Validating configuration files", "homeDir", homeDir)

	configPath := filepath.Join(homeDir, ".local/share/devex/config")

	// Define required configuration files and their basic structure for the new hybrid model
	requiredFiles := map[string]func(map[string]interface{}) error{
		"terminal.yaml":              validateTerminalConfig,
		"terminal-optional.yaml":     validateTerminalOptionalConfig,
		"desktop.yaml":               validateDesktopApplicationsConfig,
		"desktop-optional.yaml":      validateDesktopOptionalConfig,
		"databases.yaml":             validateDatabasesConfig,
		"programming-languages.yaml": validateProgrammingLanguagesConfig,
		"fonts.yaml":                 validateFontsConfig,
		"shell.yaml":                 validateShellConfig,
		"dotfiles.yaml":              validateDotfilesConfig,
		"gnome.yaml":                 validateGnomeConfig,
		"kde.yaml":                   validateKdeConfig,
		"macos.yaml":                 validateMacosConfig,
		"windows.yaml":               validateWindowsConfig,
	}

	var validationErrors []string

	for filename, validator := range requiredFiles {
		filePath := filepath.Join(configPath, filename)

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			validationErrors = append(validationErrors, fmt.Sprintf("required config file missing: %s", filename))
			continue
		}

		// Read and parse YAML
		data, err := os.ReadFile(filePath)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("failed to read %s: %v", filename, err))
			continue
		}

		var config map[string]interface{}
		if err := yaml.Unmarshal(data, &config); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid YAML in %s: %v", filename, err))
			continue
		}

		// Run specific validation
		if err := validator(config); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("validation failed for %s: %v", filename, err))
		}
	}

	if len(validationErrors) > 0 {
		for _, err := range validationErrors {
			log.Warn("Configuration validation issue", "error", err)
		}
		return fmt.Errorf("configuration validation failed: %d issues found", len(validationErrors))
	}

	log.Info("Configuration validation completed successfully")
	return nil
}

// ValidateApplicationsConfig validates the legacy applications.yaml structure for backward compatibility
func ValidateApplicationsConfig(config map[string]interface{}) error {
	// Check if the applications section exists
	applications, exists := config["applications"]
	if !exists {
		return fmt.Errorf("missing required section: applications")
	}

	// Cast to map for further validation
	appsMap, ok := applications.(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("applications section must be a map")
	}

	requiredSections := []string{"development", "databases", "system_tools", "optional"}

	for _, section := range requiredSections {
		if _, exists := appsMap[section]; !exists {
			return fmt.Errorf("missing required section: applications.%s", section)
		}
	}

	return nil
}

// validateTerminalConfig validates the terminal.yaml structure
func validateTerminalConfig(config map[string]interface{}) error {
	if applications, exists := config["applications"]; exists {
		appsMap, ok := applications.(map[interface{}]interface{})
		if !ok {
			return fmt.Errorf("applications section must be a map")
		}
		// Optional validation for expected sections
		expectedSections := []string{"development", "utilities", "dependencies"}
		for _, section := range expectedSections {
			if _, exists := appsMap[section]; !exists {
				log.Info("Optional section missing", "section", section, "file", "terminal.yaml")
			}
		}
	}
	return nil
}

// validateTerminalOptionalConfig validates the terminal-optional.yaml structure
func validateTerminalOptionalConfig(config map[string]interface{}) error {
	if applications, exists := config["applications"]; exists {
		_, ok := applications.(map[interface{}]interface{})
		if !ok {
			return fmt.Errorf("applications section must be a map")
		}
	}
	return nil
}

// validateDesktopApplicationsConfig validates the desktop.yaml structure
func validateDesktopApplicationsConfig(config map[string]interface{}) error {
	if _, exists := config["desktop_applications"]; exists {
		return nil // Valid if desktop_applications section exists
	}
	// Desktop config can be empty, so this is not an error
	return nil
}

// validateDesktopOptionalConfig validates the desktop-optional.yaml structure
func validateDesktopOptionalConfig(config map[string]interface{}) error {
	// Desktop optional can have any structure, just validate it's parseable
	return nil
}

// validateDatabasesConfig validates the databases.yaml structure
func validateDatabasesConfig(config map[string]interface{}) error {
	if applications, exists := config["applications"]; exists {
		_, ok := applications.(map[interface{}]interface{})
		if !ok {
			return fmt.Errorf("applications section must be a map")
		}
	}
	return nil
}

// validateProgrammingLanguagesConfig validates the programming-languages.yaml structure
func validateProgrammingLanguagesConfig(config map[string]interface{}) error {
	if _, exists := config["programming_languages"]; !exists {
		return fmt.Errorf("missing required section: programming_languages")
	}
	return nil
}

// validateFontsConfig validates the fonts.yaml structure
func validateFontsConfig(config map[string]interface{}) error {
	if _, exists := config["fonts"]; !exists {
		return fmt.Errorf("missing required section: fonts")
	}
	return nil
}

// validateShellConfig validates the shell.yaml structure
func validateShellConfig(config map[string]interface{}) error {
	// Shell config can be empty initially
	return nil
}

// validateDotfilesConfig validates the dotfiles.yaml structure
func validateDotfilesConfig(config map[string]interface{}) error {
	// Dotfiles config should have at least one section
	if len(config) == 0 {
		log.Info("Dotfiles config is empty", "file", "dotfiles.yaml")
	}
	return nil
}

// validateGnomeConfig validates the gnome.yaml structure
func validateGnomeConfig(config map[string]interface{}) error {
	// GNOME config can be empty initially
	return nil
}

// validateKdeConfig validates the kde.yaml structure
func validateKdeConfig(config map[string]interface{}) error {
	// KDE config can be empty initially
	return nil
}

// validateMacosConfig validates the macos.yaml structure
func validateMacosConfig(config map[string]interface{}) error {
	// macOS config can be empty initially
	return nil
}

// validateWindowsConfig validates the windows.yaml structure
func validateWindowsConfig(config map[string]interface{}) error {
	// Windows config can be empty initially
	return nil
}

// ValidateAppConfig performs basic validation on an app configuration
func ValidateAppConfig(app map[string]interface{}) error {
	// Check required fields
	if name, exists := app["name"]; !exists || name == "" {
		return fmt.Errorf("app missing required field: name")
	}

	if method, exists := app["install_method"]; !exists || method == "" {
		return fmt.Errorf("app missing required field: install_method")
	}

	if command, exists := app["install_command"]; !exists || command == "" {
		return fmt.Errorf("app missing required field: install_command")
	}

	return nil
}
