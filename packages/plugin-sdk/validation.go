package sdk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ValidateTimeoutConfig validates a timeout configuration
func ValidateTimeoutConfig(config TimeoutConfig) error {
	if config.Default < 0 {
		return fmt.Errorf("default timeout cannot be negative")
	}
	if config.Install < 0 {
		return fmt.Errorf("install timeout cannot be negative")
	}
	if config.Update < 0 {
		return fmt.Errorf("update timeout cannot be negative")
	}
	if config.Upgrade < 0 {
		return fmt.Errorf("upgrade timeout cannot be negative")
	}
	if config.Search < 0 {
		return fmt.Errorf("search timeout cannot be negative")
	}
	if config.Network < 0 {
		return fmt.Errorf("network timeout cannot be negative")
	}
	if config.Build < 0 {
		return fmt.Errorf("build timeout cannot be negative")
	}
	if config.Shell < 0 {
		return fmt.Errorf("shell timeout cannot be negative")
	}
	
	return nil
}

// ValidateTimeout validates a specific timeout duration
func ValidateTimeout(timeout time.Duration, operationType string) error {
	if timeout < 0 {
		return fmt.Errorf("%s timeout cannot be negative", operationType)
	}
	
	// Warn if timeout is extremely short (likely a mistake)
	if timeout > 0 && timeout < 5*time.Second {
		// Note: We don't return an error for short timeouts, just document the concern
		// Some operations might legitimately need very short timeouts
		_ = timeout // Suppress unused variable warning
	}
	
	// Warn if timeout is extremely long (potentially a mistake)  
	if timeout > 2*time.Hour {
		// Note: We don't return an error for long timeouts, just document the concern
		// Some operations like large builds might legitimately need very long timeouts
		_ = timeout // Suppress unused variable warning
	}
	
	return nil
}

// IsTimeoutError checks if an error is a TimeoutError
func IsTimeoutError(err error) bool {
	_, ok := err.(*TimeoutError)
	return ok
}

// GetTimeoutError extracts TimeoutError from error chain
func GetTimeoutError(err error) *TimeoutError {
	if timeoutErr, ok := err.(*TimeoutError); ok {
		return timeoutErr
	}
	return nil
}

// NormalizeTimeout ensures timeout is within reasonable bounds
func NormalizeTimeout(timeout time.Duration, operationType string) time.Duration {
	// If zero or negative, use default
	if timeout <= 0 {
		return DefaultTimeouts().GetTimeout(operationType)
	}
	
	// Set minimum timeout to prevent too-short operations that may fail spuriously
	minTimeout := 5 * time.Second
	if timeout < minTimeout {
		return minTimeout
	}
	
	// Set maximum timeout to prevent runaway operations
	maxTimeout := 2 * time.Hour
	if timeout > maxTimeout {
		return maxTimeout
	}
	
	return timeout
}

// TimeoutConfigFromDefaults creates a timeout config with specific overrides
func TimeoutConfigFromDefaults(overrides map[string]time.Duration) TimeoutConfig {
	config := DefaultTimeouts()
	
	for operation, timeout := range overrides {
		switch operation {
		case "default":
			config.Default = timeout
		case "install":
			config.Install = timeout
		case "update":
			config.Update = timeout
		case "upgrade":
			config.Upgrade = timeout
		case "search":
			config.Search = timeout
		case "network":
			config.Network = timeout
		case "build":
			config.Build = timeout
		case "shell":
			config.Shell = timeout
		}
	}
	
	return config
}

// Environment Variable Security Validation Functions

// EnvVarSeverity represents the security risk level of environment variables
type EnvVarSeverity int

const (
	// EnvVarSafe indicates the variable is generally safe to use
	EnvVarSafe EnvVarSeverity = iota
	// EnvVarWarning indicates the variable may pose security risks
	EnvVarWarning
	// EnvVarDangerous indicates the variable poses significant security risks
	EnvVarDangerous
	// EnvVarBlocked indicates the variable should never be used
	EnvVarBlocked
)

// EnvVarConfig represents validation configuration for an environment variable
type EnvVarConfig struct {
	Name         string
	Severity     EnvVarSeverity
	Description  string
	AllowEmpty   bool
	ValidateFunc func(string) error
}

// Common dangerous environment variables that can be used for attacks
var dangerousEnvVars = map[string]EnvVarConfig{
	// Path manipulation variables
	"LD_PRELOAD": {
		Name:        "LD_PRELOAD",
		Severity:    EnvVarBlocked,
		Description: "Can be used to inject malicious shared libraries",
		AllowEmpty:  true,
	},
	"LD_LIBRARY_PATH": {
		Name:        "LD_LIBRARY_PATH", 
		Severity:    EnvVarDangerous,
		Description: "Can redirect library loading to malicious libraries",
		AllowEmpty:  true,
	},
	"DYLD_LIBRARY_PATH": {
		Name:        "DYLD_LIBRARY_PATH",
		Severity:    EnvVarDangerous,
		Description: "macOS equivalent of LD_LIBRARY_PATH",
		AllowEmpty:  true,
	},
	"DYLD_INSERT_LIBRARIES": {
		Name:        "DYLD_INSERT_LIBRARIES",
		Severity:    EnvVarBlocked,
		Description: "macOS equivalent of LD_PRELOAD",
		AllowEmpty:  true,
	},
	
	// Python-related security risks
	"PYTHONPATH": {
		Name:        "PYTHONPATH",
		Severity:    EnvVarDangerous,
		Description: "Can redirect Python module loading to malicious modules",
		AllowEmpty:  true,
	},
	"PYTHONSTARTUP": {
		Name:        "PYTHONSTARTUP",
		Severity:    EnvVarBlocked,
		Description: "Executes arbitrary Python code on startup",
		AllowEmpty:  true,
	},
	
	// Configuration injection risks
	"APT_CONFIG": {
		Name:        "APT_CONFIG",
		Severity:    EnvVarDangerous,
		Description: "Can override APT configuration with malicious settings",
		AllowEmpty:  true,
	},
	"DNF_CONFIG": {
		Name:        "DNF_CONFIG",
		Severity:    EnvVarDangerous,
		Description: "Can override DNF configuration with malicious settings",
		AllowEmpty:  true,
	},
	"YUM_CONFIG": {
		Name:        "YUM_CONFIG",
		Severity:    EnvVarDangerous,
		Description: "Can override YUM configuration with malicious settings",
		AllowEmpty:  true,
	},
	"PACMAN_CONFIG": {
		Name:        "PACMAN_CONFIG",
		Severity:    EnvVarDangerous,
		Description: "Can override Pacman configuration with malicious settings",
		AllowEmpty:  true,
	},
	"ZYPPER_CONFIG": {
		Name:        "ZYPPER_CONFIG", 
		Severity:    EnvVarDangerous,
		Description: "Can override Zypper configuration with malicious settings",
		AllowEmpty:  true,
	},
	"FLATPAK_USER_DIR": {
		Name:         "FLATPAK_USER_DIR",
		Severity:     EnvVarWarning,
		Description:  "Can redirect Flatpak user directory",
		AllowEmpty:   true,
		ValidateFunc: validateFlatpakUserDir,
	},
	"FLATPAK_SYSTEM_DIR": {
		Name:         "FLATPAK_SYSTEM_DIR", 
		Severity:     EnvVarWarning,
		Description:  "Can redirect Flatpak system directory",
		AllowEmpty:   true,
		ValidateFunc: validateFlatpakSystemDir,
	},
	"SNAP_USER_DATA": {
		Name:         "SNAP_USER_DATA",
		Severity:     EnvVarWarning,
		Description:  "Can redirect Snap user data directory",
		AllowEmpty:   true,
		ValidateFunc: validateSnapUserData,
	},
	"HOMEBREW_PREFIX": {
		Name:         "HOMEBREW_PREFIX",
		Severity:     EnvVarWarning,
		Description:  "Can redirect Homebrew installation directory",
		AllowEmpty:   true,
		ValidateFunc: validateHomebrewPrefix,
	},
	"HOMEBREW_REPOSITORY": {
		Name:         "HOMEBREW_REPOSITORY",
		Severity:     EnvVarWarning,
		Description:  "Can redirect Homebrew repository location",
		AllowEmpty:   true,
		ValidateFunc: validateHomebrewRepository,
	},
	"DOCKER_HOST": {
		Name:         "DOCKER_HOST",
		Severity:     EnvVarWarning,
		Description:  "Can redirect Docker commands to malicious daemon",
		AllowEmpty:   true,
		ValidateFunc: validateDockerHost,
	},
	"PIP_INDEX_URL": {
		Name:         "PIP_INDEX_URL",
		Severity:     EnvVarWarning,
		Description:  "Can redirect pip to malicious package index",
		AllowEmpty:   true,
		ValidateFunc: validatePipIndexURL,
	},
	"PIP_EXTRA_INDEX_URL": {
		Name:         "PIP_EXTRA_INDEX_URL",
		Severity:     EnvVarWarning,
		Description:  "Can add malicious package indexes",
		AllowEmpty:   true,
		ValidateFunc: validatePipIndexURL,
	},
	
	// Shell and execution risks
	"IFS": {
		Name:        "IFS",
		Severity:    EnvVarBlocked,
		Description: "Can be used for shell injection attacks",
		AllowEmpty:  true,
	},
	"PS4": {
		Name:        "PS4",
		Severity:    EnvVarDangerous,
		Description: "Can execute commands during shell tracing",
		AllowEmpty:  true,
	},
}

// Common system environment variables that need validation
var systemEnvVars = map[string]EnvVarConfig{
	"PATH": {
		Name:         "PATH",
		Severity:     EnvVarWarning,
		Description:  "Command search path - validate for malicious directories",
		AllowEmpty:   false,
		ValidateFunc: validatePath,
	},
	"HOME": {
		Name:         "HOME",
		Severity:     EnvVarWarning,
		Description:  "User home directory - validate path traversal",
		AllowEmpty:   false,
		ValidateFunc: validateHomePath,
	},
	"USER": {
		Name:         "USER",
		Severity:     EnvVarSafe,
		Description:  "Current username - validate format",
		AllowEmpty:   false,
		ValidateFunc: validateUsername,
	},
	"LOGNAME": {
		Name:         "LOGNAME", 
		Severity:     EnvVarSafe,
		Description:  "Login name - validate format",
		AllowEmpty:   false,
		ValidateFunc: validateUsername,
	},
	"SHELL": {
		Name:         "SHELL",
		Severity:     EnvVarWarning,
		Description:  "User shell - validate path",
		AllowEmpty:   false,
		ValidateFunc: validateShellPath,
	},
	"TMPDIR": {
		Name:         "TMPDIR",
		Severity:     EnvVarWarning,
		Description:  "Temporary directory - validate path",
		AllowEmpty:   true,
		ValidateFunc: validateTempDir,
	},
}

// DevEx-specific environment variables
var devexEnvVars = map[string]EnvVarConfig{
	"DEVEX_ENV": {
		Name:         "DEVEX_ENV",
		Severity:     EnvVarSafe,
		Description:  "DevEx environment mode",
		AllowEmpty:   true,
		ValidateFunc: validateDevexEnv,
	},
	"DEVEX_CONFIG_DIR": {
		Name:         "DEVEX_CONFIG_DIR",
		Severity:     EnvVarWarning,
		Description:  "DevEx configuration directory",
		AllowEmpty:   true,
		ValidateFunc: validateConfigDir,
	},
	"DEVEX_PLUGIN_DIR": {
		Name:         "DEVEX_PLUGIN_DIR",
		Severity:     EnvVarWarning,
		Description:  "DevEx plugin directory",
		AllowEmpty:   true,
		ValidateFunc: validatePluginDir,
	},
	"DEVEX_CACHE_DIR": {
		Name:         "DEVEX_CACHE_DIR",
		Severity:     EnvVarWarning,
		Description:  "DevEx cache directory",
		AllowEmpty:   true,
		ValidateFunc: validateCacheDir,
	},
}

// ValidateEnvironmentVariable validates a specific environment variable value
func ValidateEnvironmentVariable(name, value string) error {
	// Check dangerous variables first
	if config, exists := dangerousEnvVars[name]; exists {
		switch config.Severity {
		case EnvVarBlocked:
			if value != "" {
				return fmt.Errorf("environment variable %s is blocked for security reasons: %s", name, config.Description)
			}
		case EnvVarDangerous:
			if value != "" && config.ValidateFunc != nil {
				if err := config.ValidateFunc(value); err != nil {
					return fmt.Errorf("dangerous environment variable %s failed validation: %w", name, err)
				}
			}
		case EnvVarWarning:
			if value != "" && config.ValidateFunc != nil {
				if err := config.ValidateFunc(value); err != nil {
					return fmt.Errorf("warning environment variable %s failed validation: %w", name, err)
				}
			}
		}
	}
	
	// Check system variables
	if config, exists := systemEnvVars[name]; exists {
		if !config.AllowEmpty && value == "" {
			return fmt.Errorf("system environment variable %s cannot be empty", name)
		}
		if value != "" && config.ValidateFunc != nil {
			if err := config.ValidateFunc(value); err != nil {
				return fmt.Errorf("system environment variable %s failed validation: %w", name, err)
			}
		}
	}
	
	// Check DevEx variables
	if config, exists := devexEnvVars[name]; exists {
		if !config.AllowEmpty && value == "" {
			return fmt.Errorf("DevEx environment variable %s cannot be empty", name)
		}
		if value != "" && config.ValidateFunc != nil {
			if err := config.ValidateFunc(value); err != nil {
				return fmt.Errorf("DevEx environment variable %s failed validation: %w", name, err)
			}
		}
	}
	
	return nil
}

// SafeGetEnv safely retrieves an environment variable with validation
func SafeGetEnv(name string) (string, error) {
	value := os.Getenv(name)
	if err := ValidateEnvironmentVariable(name, value); err != nil {
		return "", err
	}
	return value, nil
}

// SafeGetEnvWithDefault safely retrieves an environment variable with default and validation
func SafeGetEnvWithDefault(name, defaultValue string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		value = defaultValue
	}
	if err := ValidateEnvironmentVariable(name, value); err != nil {
		return defaultValue, err
	}
	return value, nil
}

// SanitizeEnvironmentForLogging removes sensitive environment variables for logging
func SanitizeEnvironmentForLogging(env []string) []string {
	sanitized := make([]string, 0, len(env))
	
	for _, envVar := range env {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		name := parts[0]
		value := parts[1]
		
		// Check if this is a sensitive variable
		if shouldRedactEnvVar(name) {
			sanitized = append(sanitized, name+"=[REDACTED]")
		} else if shouldPartiallyRedactEnvVar(name) {
			sanitized = append(sanitized, name+"="+partiallyRedactValue(value))
		} else {
			sanitized = append(sanitized, envVar)
		}
	}
	
	return sanitized
}

// SanitizeEnvVarForLogging sanitizes a single environment variable for logging
func SanitizeEnvVarForLogging(name, value string) string {
	if shouldRedactEnvVar(name) {
		return "[REDACTED]"
	} else if shouldPartiallyRedactEnvVar(name) {
		return partiallyRedactValue(value)
	}
	return value
}

// CheckEnvironmentSecurity performs a comprehensive security check of environment variables
func CheckEnvironmentSecurity() []SecurityIssue {
	var issues []SecurityIssue
	
	// Check for dangerous variables
	for name, config := range dangerousEnvVars {
		value := os.Getenv(name)
		if value != "" {
			switch config.Severity {
			case EnvVarBlocked:
				issues = append(issues, SecurityIssue{
					Type:        "blocked_env_var",
					Severity:    "critical",
					Description: fmt.Sprintf("Blocked environment variable %s is set: %s", name, config.Description),
					Variable:    name,
				})
			case EnvVarDangerous:
				issues = append(issues, SecurityIssue{
					Type:        "dangerous_env_var",
					Severity:    "high",
					Description: fmt.Sprintf("Dangerous environment variable %s is set: %s", name, config.Description),
					Variable:    name,
				})
			}
		}
	}
	
	// Validate system variables
	for name, config := range systemEnvVars {
		value := os.Getenv(name)
		if value != "" {
			if config.ValidateFunc != nil {
				if err := config.ValidateFunc(value); err != nil {
					issues = append(issues, SecurityIssue{
						Type:        "invalid_env_var",
						Severity:    "medium", 
						Description: fmt.Sprintf("System environment variable %s has invalid value: %s", name, err.Error()),
						Variable:    name,
					})
				}
			}
		} else if !config.AllowEmpty {
			issues = append(issues, SecurityIssue{
				Type:        "missing_env_var",
				Severity:    "low",
				Description: fmt.Sprintf("Required system environment variable %s is not set", name),
				Variable:    name,
			})
		}
	}
	
	return issues
}

// SecurityIssue represents a security issue found during environment validation
type SecurityIssue struct {
	Type        string
	Severity    string
	Description string
	Variable    string
}

// Helper functions for validation

func validatePath(value string) error {
	if value == "" {
		return fmt.Errorf("PATH cannot be empty")
	}
	
	paths := strings.Split(value, string(os.PathListSeparator))
	for _, path := range paths {
		// Check for suspicious paths
		if strings.Contains(path, "..") {
			return fmt.Errorf("PATH contains suspicious path with '..': %s", path)
		}
		if strings.HasPrefix(path, "/tmp") || strings.HasPrefix(path, "/var/tmp") {
			return fmt.Errorf("PATH contains potentially unsafe temporary directory: %s", path)
		}
		// Check for world-writable directories
		if path != "" {
			if info, err := os.Stat(path); err == nil {
				if info.Mode().Perm()&0002 != 0 {
					return fmt.Errorf("PATH contains world-writable directory: %s", path)
				}
			}
		}
	}
	return nil
}

func validateHomePath(value string) error {
	if value == "" {
		return fmt.Errorf("HOME cannot be empty")
	}
	
	// Check for path traversal
	if strings.Contains(value, "..") {
		return fmt.Errorf("HOME contains path traversal: %s", value)
	}
	
	// Must be an absolute path
	if !filepath.IsAbs(value) {
		return fmt.Errorf("HOME must be an absolute path: %s", value)
	}
	
	// Check if directory exists and is accessible
	if info, err := os.Stat(value); err != nil {
		return fmt.Errorf("HOME directory is not accessible: %s", value)
	} else if !info.IsDir() {
		return fmt.Errorf("HOME is not a directory: %s", value)
	}
	
	return nil
}

func validateUsername(value string) error {
	if value == "" {
		return fmt.Errorf("username cannot be empty")
	}
	
	// Check for suspicious characters
	if strings.ContainsAny(value, "\n\r\t\x00;|&`$()[]{}") {
		return fmt.Errorf("username contains suspicious characters: %s", value)
	}
	
	// Must be reasonable length
	if len(value) > 32 {
		return fmt.Errorf("username is too long: %s", value)
	}
	
	return nil
}

func validateShellPath(value string) error {
	if value == "" {
		return fmt.Errorf("SHELL cannot be empty")
	}
	
	// Must be an absolute path
	if !filepath.IsAbs(value) {
		return fmt.Errorf("SHELL must be an absolute path: %s", value)
	}
	
	// Check for path traversal
	if strings.Contains(value, "..") {
		return fmt.Errorf("SHELL contains path traversal: %s", value)
	}
	
	// Should exist and be executable
	if info, err := os.Stat(value); err != nil {
		return fmt.Errorf("SHELL does not exist: %s", value)
	} else if info.Mode()&0111 == 0 {
		return fmt.Errorf("SHELL is not executable: %s", value)
	}
	
	return nil
}

func validateTempDir(value string) error {
	if value == "" {
		return nil // Empty is allowed
	}
	
	// Must be an absolute path
	if !filepath.IsAbs(value) {
		return fmt.Errorf("TMPDIR must be an absolute path: %s", value)
	}
	
	// Check for path traversal
	if strings.Contains(value, "..") {
		return fmt.Errorf("TMPDIR contains path traversal: %s", value)
	}
	
	return nil
}

func validateDockerHost(value string) error {
	if value == "" {
		return nil
	}
	
	// Check for suspicious protocols or hosts
	if strings.HasPrefix(value, "tcp://") {
		// Extract host part
		hostPart := strings.TrimPrefix(value, "tcp://")
		if idx := strings.Index(hostPart, ":"); idx > 0 {
			host := hostPart[:idx]
			if host != "localhost" && host != "127.0.0.1" && !strings.HasPrefix(host, "192.168.") && !strings.HasPrefix(host, "10.") {
				return fmt.Errorf("DOCKER_HOST points to potentially unsafe host: %s", host)
			}
		}
	} else if !strings.HasPrefix(value, "unix://") && !strings.HasPrefix(value, "fd://") {
		return fmt.Errorf("DOCKER_HOST has unsupported protocol: %s", value)
	}
	
	return nil
}

func validatePipIndexURL(value string) error {
	if value == "" {
		return nil
	}
	
	// Must be HTTPS for security
	if !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("pip index URL must use HTTPS: '%s'", value)
	}
	
	// Check for suspicious domains
	suspiciousDomains := []string{
		"bit.ly", "tinyurl.com", "short.link", "t.co",
		"localhost", "127.0.0.1", "0.0.0.0",
	}
	
	for _, domain := range suspiciousDomains {
		if strings.Contains(value, domain) {
			return fmt.Errorf("PIP index URL contains suspicious domain: %s", value)
		}
	}
	
	return nil
}

func validateDevexEnv(value string) error {
	if value == "" {
		return nil
	}
	
	allowedEnvs := []string{"development", "dev", "staging", "production", "prod", "test", "testing"}
	for _, env := range allowedEnvs {
		if value == env {
			return nil
		}
	}
	
	return fmt.Errorf("invalid DEVEX_ENV value: %s (allowed: %s)", value, strings.Join(allowedEnvs, ", "))
}

func validateConfigDir(value string) error {
	return validateDirectoryPath(value, "DEVEX_CONFIG_DIR")
}

func validatePluginDir(value string) error {
	return validateDirectoryPath(value, "DEVEX_PLUGIN_DIR")
}

func validateCacheDir(value string) error {
	return validateDirectoryPath(value, "DEVEX_CACHE_DIR")
}

func validateDirectoryPath(value, varName string) error {
	if value == "" {
		return nil
	}
	
	// Must be an absolute path
	if !filepath.IsAbs(value) {
		return fmt.Errorf("%s must be an absolute path: %s", varName, value)
	}
	
	// Check for path traversal
	if strings.Contains(value, "..") {
		return fmt.Errorf("%s contains path traversal: %s", varName, value)
	}
	
	return nil
}

func shouldRedactEnvVar(name string) bool {
	redactPatterns := []string{
		"PASSWORD", "PASSWD", "SECRET", "TOKEN", "KEY", "CREDENTIAL",
		"AUTH", "API_KEY", "ACCESS_TOKEN", "PRIVATE_KEY", "CERT",
	}
	
	upperName := strings.ToUpper(name)
	for _, pattern := range redactPatterns {
		if strings.Contains(upperName, pattern) {
			return true
		}
	}
	
	return false
}

func shouldPartiallyRedactEnvVar(name string) bool {
	partialRedactVars := []string{
		"PATH", "HOME", "USER", "LOGNAME", 
		"DOCKER_HOST", "PIP_INDEX_URL",
	}
	
	for _, varName := range partialRedactVars {
		if name == varName {
			return true
		}
	}
	
	return false
}

func partiallyRedactValue(value string) string {
	if len(value) <= 8 {
		return "[REDACTED]"
	}
	
	// Show first and last 2 characters with middle redacted
	return value[:2] + "..." + value[len(value)-2:]
}


// Package manager specific validation functions

func validateFlatpakUserDir(value string) error {
	return validateDirectoryPath(value, "FLATPAK_USER_DIR")
}

func validateFlatpakSystemDir(value string) error {
	return validateDirectoryPath(value, "FLATPAK_SYSTEM_DIR")
}

func validateSnapUserData(value string) error {
	return validateDirectoryPath(value, "SNAP_USER_DATA")
}

func validateHomebrewPrefix(value string) error {
	if err := validateDirectoryPath(value, "HOMEBREW_PREFIX"); err != nil {
		return err
	}
	
	// Additional validation for Homebrew prefix - should be safe locations
	if value != "" {
		safePrefixes := []string{"/usr/local", "/opt/homebrew", "/home/linuxbrew/.linuxbrew"}
		for _, prefix := range safePrefixes {
			if strings.HasPrefix(value, prefix) {
				return nil
			}
		}
		// Allow user home directory installations
		if home := os.Getenv("HOME"); home != "" && strings.HasPrefix(value, home) {
			return nil
		}
		return fmt.Errorf("HOMEBREW_PREFIX points to potentially unsafe location: %s", value)
	}
	
	return nil
}

func validateHomebrewRepository(value string) error {
	if err := validateDirectoryPath(value, "HOMEBREW_REPOSITORY"); err != nil {
		return err
	}
	
	// Additional validation for Homebrew repository
	if value != "" {
		// Should be within Homebrew prefix or user home
		safePatterns := []string{"/usr/local/Homebrew", "/opt/homebrew", "/home/linuxbrew/.linuxbrew"}
		for _, pattern := range safePatterns {
			if strings.HasPrefix(value, pattern) {
				return nil
			}
		}
		// Allow user home directory installations
		if home := os.Getenv("HOME"); home != "" && strings.HasPrefix(value, home) {
			return nil
		}
		return fmt.Errorf("HOMEBREW_REPOSITORY points to potentially unsafe location: %s", value)
	}
	
	return nil
}
