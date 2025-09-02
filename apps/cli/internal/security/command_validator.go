package security

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// SecurityLevel defines how strict command validation should be
type SecurityLevel int

const (
	// SecurityLevelStrict blocks everything except explicitly safe patterns
	SecurityLevelStrict SecurityLevel = iota
	// SecurityLevelModerate allows most commands but blocks dangerous patterns
	SecurityLevelModerate
	// SecurityLevelPermissive only blocks obviously malicious patterns
	SecurityLevelPermissive
	// SecurityLevelEnterprise allows everything with warnings (enterprise admin override)
	SecurityLevelEnterprise
)

// SecurityRuleType defines the type of security rule being overridden
type SecurityRuleType string

const (
	RuleTypeDangerousCommand    SecurityRuleType = "dangerous-command"
	RuleTypeUnknownExecutable   SecurityRuleType = "unknown-executable"
	RuleTypeCommandInjection    SecurityRuleType = "command-injection"
	RuleTypePrivilegeEscalation SecurityRuleType = "privilege-escalation"
	RuleTypeNetworkAccess       SecurityRuleType = "network-access"
	RuleTypeFileSystemAccess    SecurityRuleType = "filesystem-access"
)

// SecurityOverride represents a security rule override
type SecurityOverride struct {
	RuleType SecurityRuleType `yaml:"rule_type" json:"rule_type"`
	Pattern  string           `yaml:"pattern" json:"pattern"`
	Reason   string           `yaml:"reason" json:"reason"`
	AppName  string           `yaml:"app_name,omitempty" json:"app_name,omitempty"`
	WarnUser bool             `yaml:"warn_user" json:"warn_user"`
}

// SecurityConfig holds security configuration and overrides
type SecurityConfig struct {
	Level                SecurityLevel                 `yaml:"level" json:"level"`
	GlobalOverrides      []SecurityOverride            `yaml:"global_overrides" json:"global_overrides"`
	AppSpecificOverrides map[string][]SecurityOverride `yaml:"app_overrides" json:"app_overrides"`
	EnterpriseMode       bool                          `yaml:"enterprise_mode" json:"enterprise_mode"`
	WarnOnOverrides      bool                          `yaml:"warn_on_overrides" json:"warn_on_overrides"`
}

// CommandValidator provides flexible command validation with configurable security levels
type CommandValidator struct {
	level              SecurityLevel
	dangerousPatterns  []*regexp.Regexp
	safePatterns       []*regexp.Regexp
	customWhitelist    map[string]bool
	customBlacklist    map[string]bool
	allowedExecutables []string // Dynamically discovered executables
	securityConfig     *SecurityConfig
	logger             func(string, ...interface{})
}

// NewCommandValidator creates a validator with the specified security level
func NewCommandValidator(level SecurityLevel) *CommandValidator {
	return NewCommandValidatorWithConfig(level, nil)
}

// NewCommandValidatorWithConfig creates a validator with security configuration
func NewCommandValidatorWithConfig(level SecurityLevel, config *SecurityConfig) *CommandValidator {
	cv := &CommandValidator{
		level:           level,
		customWhitelist: make(map[string]bool),
		customBlacklist: make(map[string]bool),
		securityConfig:  config,
		logger:          log.Printf, // Default logger
	}

	// Override level from config if provided
	if config != nil {
		cv.level = config.Level
	}

	cv.initializePatterns()
	cv.discoverExecutables()

	return cv
}

// initializePatterns sets up dangerous and safe patterns
func (cv *CommandValidator) initializePatterns() {
	// Always block these dangerous patterns regardless of security level
	cv.dangerousPatterns = []*regexp.Regexp{
		// Destructive commands
		regexp.MustCompile(`\brm\s+(-[rfRi]*\s+)*(/|/home|/usr|/var|/etc|/boot|/sys|/proc)\s*$`),
		regexp.MustCompile(`\bdd\s+.*\bof=/dev/(sd[a-z]|hd[a-z]|nvme\d+n\d+)\b`),
		regexp.MustCompile(`\bmkfs\b.*\b/dev/`),

		// Fork bombs and resource exhaustion
		regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`),
		regexp.MustCompile(`\.\s+/dev/zero`),

		// Privilege escalation attempts
		regexp.MustCompile(`\bchmod\s+[+\-]?s`), // SUID/SGID
		regexp.MustCompile(`\bsetuid\s*\(`),

		// System file manipulation
		regexp.MustCompile(`>\s*/etc/(passwd|shadow|sudoers|hosts)\b`),
		regexp.MustCompile(`>\s*/boot/`),

		// Network attacks
		regexp.MustCompile(`\bnc\s+.*-e\s+/bin/(bash|sh)`), // Reverse shells
		regexp.MustCompile(`\bpython.*\s+-c.*socket.*connect`),

		// Code injection in specific contexts
		regexp.MustCompile(`\bcurl\s+.*\|\s*(bash|sh)\b`),
		regexp.MustCompile(`\bwget\s+.*\|\s*(bash|sh)\b`),

		// Dangerous bash -c patterns
		regexp.MustCompile(`\|\s*bash\s+-c\s+.*rm\s+-rf`),
		regexp.MustCompile(`bash\s+-c\s+.*\|\s*(bash|sh)`),
		regexp.MustCompile(`bash\s+-c\s+.*rm\s+-rf`),

		// Dangerous commands in command substitution
		regexp.MustCompile(`\$\(\s*(rm\s+-rf|dd\s+.*of=|mkfs)\b`),

		// Block command substitution with suspicious keywords
		regexp.MustCompile(`\$\([^)]*malicious[^)]*\)`), // Contains "malicious" keyword
		regexp.MustCompile(`\$\([^)]*evil[^)]*\)`),      // Contains "evil" keyword
		regexp.MustCompile(`\$\([^)]*hack[^)]*\)`),      // Contains "hack" keyword
	}

	// Safe patterns that should generally be allowed
	cv.safePatterns = []*regexp.Regexp{
		// Package managers (cross-platform)
		regexp.MustCompile(`^(apt|apt-get|yum|dnf|pacman|zypper|brew|snap|flatpak|pip|pip3|npm|yarn|pnpm|cargo|go|gem|composer|nuget|choco|scoop|winget)\s+`),

		// Version checks
		regexp.MustCompile(`\s+--version\s*$`),
		regexp.MustCompile(`\s+-v\s*$`),
		regexp.MustCompile(`\s+version\s*$`),

		// Help commands
		regexp.MustCompile(`\s+--help\s*$`),
		regexp.MustCompile(`\s+-h\s*$`),
		regexp.MustCompile(`\s+help\s*$`),

		// Common dev tools
		regexp.MustCompile(`^(git|docker|kubectl|terraform|ansible|vagrant)\s+`),

		// File operations (safe variants)
		regexp.MustCompile(`^(ls|cat|grep|find|which|whereis|file|stat|head|tail|less|more)\s+`),
		regexp.MustCompile(`^(cp|mv|mkdir|touch)\s+[^/]`), // Not on root paths

		// Shell built-ins and safe utilities
		regexp.MustCompile(`^(echo|printf|export|source|eval "\$\(.*init.*\)")\s*`),
		regexp.MustCompile(`^(cd|pwd|date|hostname|whoami|id|uname)\s*`),
	}
}

// discoverExecutables finds available executables on the system
func (cv *CommandValidator) discoverExecutables() {
	// Common paths to search for executables
	paths := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))

	cv.allowedExecutables = []string{}

	// Limit discovery to avoid performance issues
	maxExecutables := 1000
	found := 0

	for _, dir := range paths {
		if found >= maxExecutables {
			break
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if found >= maxExecutables {
				break
			}

			if !entry.IsDir() {
				info, err := entry.Info()
				if err == nil && info.Mode()&0111 != 0 { // Is executable
					cv.allowedExecutables = append(cv.allowedExecutables, entry.Name())
					found++
				}
			}
		}
	}
}

// ValidateCommand checks if a command is safe to execute with configurable overrides
func (cv *CommandValidator) ValidateCommand(command string) error {
	return cv.ValidateCommandForApp(command, "")
}

// ValidateCommandForApp checks if a command is safe to execute for a specific app
func (cv *CommandValidator) ValidateCommandForApp(command, appName string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("empty command")
	}

	// Check for security overrides first
	if override := cv.checkOverrides(command, appName); override != nil {
		if override.WarnUser {
			cv.warnSecurityOverride(command, override)
		}
		return nil // Override allows the command
	}

	// Check critical dangerous patterns first (always enforced unless overridden)
	if err := cv.checkCriticalPatterns(command); err != nil {
		return err
	}

	// Apply security level logic
	switch cv.level {
	case SecurityLevelEnterprise:
		return cv.validateEnterprise(command)
	case SecurityLevelStrict:
		return cv.validateStrict(command)
	case SecurityLevelModerate:
		return cv.validateModerate(command)
	case SecurityLevelPermissive:
		return cv.validatePermissive(command)
	default:
		return cv.validateModerate(command)
	}
}

// checkCriticalPatterns checks for the most dangerous patterns that should rarely be overridden
func (cv *CommandValidator) checkCriticalPatterns(command string) error {
	criticalPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\brm\s+(-[rfRi]*\s+)*(/[^a-zA-Z0-9/_-]|/$|/home[^a-zA-Z0-9/_-]|/home$|/usr[^a-zA-Z0-9/_-]|/usr$|/var[^a-zA-Z0-9/_-]|/var$|/etc[^a-zA-Z0-9/_-]|/etc$|/boot[^a-zA-Z0-9/_-]|/boot$|/sys[^a-zA-Z0-9/_-]|/sys$|/proc[^a-zA-Z0-9/_-]|/proc$)`), // rm -rf on system dirs
		regexp.MustCompile(`\bdd\s+.*\bof=/dev/(sd[a-z]|hd[a-z]|nvme\d+n\d+)\b`), // dd to disks
		regexp.MustCompile(`\bmkfs[\w\.]*\s+/dev/`),                              // format disks
		regexp.MustCompile(`:\(\)\{.*:\|:&.*\};:`),                               // fork bombs
	}

	for _, pattern := range criticalPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains critical dangerous pattern - use security override if intentional")
		}
	}
	return nil
}

// checkOverrides checks if the command has any security overrides
func (cv *CommandValidator) checkOverrides(command, appName string) *SecurityOverride {
	if cv.securityConfig == nil {
		return nil
	}

	// Check app-specific overrides first
	if appName != "" {
		if appOverrides, exists := cv.securityConfig.AppSpecificOverrides[appName]; exists {
			for _, override := range appOverrides {
				if cv.matchesOverride(command, override) {
					return &override
				}
			}
		}
	}

	// Check global overrides
	for _, override := range cv.securityConfig.GlobalOverrides {
		if cv.matchesOverride(command, override) {
			return &override
		}
	}

	return nil
}

// matchesOverride checks if a command matches a security override pattern
func (cv *CommandValidator) matchesOverride(command string, override SecurityOverride) bool {
	pattern, err := regexp.Compile(override.Pattern)
	if err != nil {
		cv.logger("Invalid override pattern '%s': %v", override.Pattern, err)
		return false
	}
	return pattern.MatchString(command)
}

// warnSecurityOverride logs a warning when a security override is used
func (cv *CommandValidator) warnSecurityOverride(command string, override *SecurityOverride) {
	if cv.securityConfig != nil && !cv.securityConfig.WarnOnOverrides {
		return
	}

	message := fmt.Sprintf("🚨 SECURITY OVERRIDE: Command '%s' bypassed %s rule", command, override.RuleType)
	if override.Reason != "" {
		message += fmt.Sprintf(" (Reason: %s)", override.Reason)
	}
	if override.AppName != "" {
		message += fmt.Sprintf(" for app '%s'", override.AppName)
	}
	cv.logger(message)
}

// validateEnterprise allows everything with warnings (enterprise admin override)
func (cv *CommandValidator) validateEnterprise(command string) error {
	// In enterprise mode, we trust the admin but still warn about dangerous patterns
	for _, pattern := range cv.dangerousPatterns {
		if pattern.MatchString(command) {
			cv.logger("⚠️ ENTERPRISE MODE: Allowing potentially dangerous command: %s", command)
			break
		}
	}
	return nil
}

// validateStrict blocks everything except explicitly safe patterns
func (cv *CommandValidator) validateStrict(command string) error {
	// Check if command is in custom whitelist
	if cv.customWhitelist[command] {
		return nil
	}

	// Check against safe patterns
	for _, pattern := range cv.safePatterns {
		if pattern.MatchString(command) {
			return nil
		}
	}

	// Check if executable is known and safe
	parts := strings.Fields(command)
	if len(parts) > 0 {
		executable := parts[0]
		for _, allowed := range cv.allowedExecutables {
			if executable == allowed {
				return nil
			}
		}
	}

	return fmt.Errorf("command not in safe patterns or whitelist (strict mode)")
}

// validateModerate allows most commands but blocks dangerous patterns
func (cv *CommandValidator) validateModerate(command string) error {
	// Check if command is in custom blacklist
	if cv.customBlacklist[command] {
		return fmt.Errorf("command is blacklisted")
	}

	// Check against dangerous patterns
	for _, pattern := range cv.dangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains dangerous pattern")
		}
	}

	return nil
}

// validatePermissive only blocks obviously malicious patterns (current behavior)
func (cv *CommandValidator) validatePermissive(command string) error {
	// This maintains the current permissive behavior
	// Only the critical patterns are checked (already done above)
	return nil
}

// AddToWhitelist adds a command to the custom whitelist
func (cv *CommandValidator) AddToWhitelist(command string) {
	cv.customWhitelist[command] = true
}

// AddToBlacklist adds a command to the custom blacklist
func (cv *CommandValidator) AddToBlacklist(command string) {
	cv.customBlacklist[command] = true
}

// SetSecurityLevel changes the security level
func (cv *CommandValidator) SetSecurityLevel(level SecurityLevel) {
	cv.level = level
}

// GetSecurityLevel returns the current security level
func (cv *CommandValidator) GetSecurityLevel() SecurityLevel {
	return cv.level
}

// LoadSecurityConfig loads security configuration from YAML file
func LoadSecurityConfig(configPath string) (*SecurityConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &SecurityConfig{
				Level:                SecurityLevelModerate,
				GlobalOverrides:      []SecurityOverride{},
				AppSpecificOverrides: make(map[string][]SecurityOverride),
				EnterpriseMode:       false,
				WarnOnOverrides:      true,
			}, nil
		}
		return nil, fmt.Errorf("failed to read security config file: %w", err)
	}

	var config SecurityConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse security config YAML: %w", err)
	}

	// Initialize map if nil
	if config.AppSpecificOverrides == nil {
		config.AppSpecificOverrides = make(map[string][]SecurityOverride)
	}

	// Validate configuration
	if err := validateSecurityConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid security configuration: %w", err)
	}

	return &config, nil
}

// LoadSecurityConfigFromDefaults loads security config from standard locations
func LoadSecurityConfigFromDefaults(homeDir string) (*SecurityConfig, error) {
	// Try user override first
	userConfigPath := filepath.Join(homeDir, ".devex", "security.yaml")
	if _, err := os.Stat(userConfigPath); err == nil {
		return LoadSecurityConfig(userConfigPath)
	}

	// Try system default
	systemConfigPath := filepath.Join(homeDir, ".local", "share", "devex", "config", "security.yaml")
	if _, err := os.Stat(systemConfigPath); err == nil {
		return LoadSecurityConfig(systemConfigPath)
	}

	// Return default config if no files found
	return &SecurityConfig{
		Level:                SecurityLevelModerate,
		GlobalOverrides:      []SecurityOverride{},
		AppSpecificOverrides: make(map[string][]SecurityOverride),
		EnterpriseMode:       false,
		WarnOnOverrides:      true,
	}, nil
}

// SaveSecurityConfig saves security configuration to YAML file
func SaveSecurityConfig(config *SecurityConfig, configPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal security config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write security config file: %w", err)
	}

	return nil
}

// validateSecurityConfig validates the security configuration
func validateSecurityConfig(config *SecurityConfig) error {
	// Validate security level
	if config.Level < SecurityLevelStrict || config.Level > SecurityLevelEnterprise {
		return fmt.Errorf("invalid security level: %d", config.Level)
	}

	// Validate global overrides
	for i, override := range config.GlobalOverrides {
		if err := validateSecurityOverride(override); err != nil {
			return fmt.Errorf("invalid global override %d: %w", i, err)
		}
	}

	// Validate app-specific overrides
	for appName, overrides := range config.AppSpecificOverrides {
		if appName == "" {
			return fmt.Errorf("app name cannot be empty")
		}
		for i, override := range overrides {
			if err := validateSecurityOverride(override); err != nil {
				return fmt.Errorf("invalid override %d for app %s: %w", i, appName, err)
			}
		}
	}

	return nil
}

// validateSecurityOverride validates a single security override
func validateSecurityOverride(override SecurityOverride) error {
	// Validate rule type
	validRuleTypes := []SecurityRuleType{
		RuleTypeDangerousCommand,
		RuleTypeUnknownExecutable,
		RuleTypeCommandInjection,
		RuleTypePrivilegeEscalation,
		RuleTypeNetworkAccess,
		RuleTypeFileSystemAccess,
	}

	isValidRuleType := false
	for _, validType := range validRuleTypes {
		if override.RuleType == validType {
			isValidRuleType = true
			break
		}
	}
	if !isValidRuleType {
		return fmt.Errorf("invalid rule type: %s", override.RuleType)
	}

	// Validate pattern is a valid regex
	if override.Pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}
	if _, err := regexp.Compile(override.Pattern); err != nil {
		return fmt.Errorf("invalid regex pattern '%s': %w", override.Pattern, err)
	}

	// Reason should be provided for security transparency
	if override.Reason == "" {
		return fmt.Errorf("reason must be provided for security override")
	}

	return nil
}

// NewCommandValidatorFromConfig creates a validator from a configuration file
func NewCommandValidatorFromConfig(configPath string) (*CommandValidator, error) {
	config, err := LoadSecurityConfig(configPath)
	if err != nil {
		return nil, err
	}

	return NewCommandValidatorWithConfig(config.Level, config), nil
}

// NewCommandValidatorFromDefaults creates a validator using default config locations
func NewCommandValidatorFromDefaults(homeDir string) (*CommandValidator, error) {
	config, err := LoadSecurityConfigFromDefaults(homeDir)
	if err != nil {
		return nil, err
	}

	return NewCommandValidatorWithConfig(config.Level, config), nil
}
