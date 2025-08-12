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

	// Define required configuration files and their basic structure
	requiredFiles := map[string]func(map[string]interface{}) error{
		"applications.yaml": ValidateApplicationsConfig,
		"environment.yaml":  validateEnvironmentConfig,
		"desktop.yaml":      validateDesktopConfig,
		"system.yaml":       validateSystemConfig,
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

// ValidateApplicationsConfig validates the applications.yaml structure
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

// validateEnvironmentConfig validates the environment.yaml structure
func validateEnvironmentConfig(config map[string]interface{}) error {
	requiredSections := []string{"programming_languages", "fonts", "shell"}

	for _, section := range requiredSections {
		if _, exists := config[section]; !exists {
			return fmt.Errorf("missing required section: %s", section)
		}
	}

	return nil
}

// validateDesktopConfig validates the desktop.yaml structure
func validateDesktopConfig(config map[string]interface{}) error {
	if _, exists := config["desktop_environments"]; !exists {
		return fmt.Errorf("missing required section: desktop_environments")
	}

	return nil
}

// validateSystemConfig validates the system.yaml structure
func validateSystemConfig(config map[string]interface{}) error {
	requiredSections := []string{"git", "ssh", "terminal"}

	for _, section := range requiredSections {
		if _, exists := config[section]; !exists {
			return fmt.Errorf("missing required section: %s", section)
		}
	}

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
