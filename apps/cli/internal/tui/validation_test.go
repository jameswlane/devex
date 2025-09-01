package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Command Validation", func() {
	var installer *StreamingInstaller

	BeforeEach(func() {
		installer = createTestInstallerGinkgo()
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

	Describe("Essential Dangerous Commands Only", func() {
		It("should only reject essential dangerous commands", func() {
			// Only block the 4 essential dangerous patterns
			stillBlockedCommands := []string{
				"rm -rf /",
				"dd if=/dev/zero of=/dev/sda",
				"mkfs.ext4 /dev/sda1",
				":(){ :|:& };:",
			}

			for _, cmd := range stillBlockedCommands {
				By("rejecting essential dangerous command: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).To(HaveOccurred(), "Essential dangerous command should be rejected: %s", cmd)
				})
			}

			// These are now allowed under permissive approach
			allowedCommands := []string{
				"sh -c 'echo hello'",
				"bash -c 'pwd'",
				"python -c 'print(\"hello\")'",
				"perl -e 'print \"hello\"'",
				"ruby -e 'puts \"hello\"'",
				"nc -l 1234",
				"telnet example.com",
				"ssh user@example.com",
				"sudo echo hello",
				"unknown_command arg1 arg2",
			}

			for _, cmd := range allowedCommands {
				By("allowing command under permissive approach: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).ToNot(HaveOccurred(), "Command should be allowed under permissive approach: %s", cmd)
				})
			}
		})
	})

	Describe("Command Validation (Permissive Approach)", func() {
		It("should only block essential dangerous commands", func() {
			// Only block the 4 essential dangerous patterns
			stillBlockedCommands := []string{
				"rm -rf /",
				"rm -rf /home",
				"dd if=/dev/zero of=/dev/sda",
				"dd if=/dev/urandom of=/dev/sda",
				":(){ :|:& };:", // Fork bomb
				"mkfs.ext4 /dev/sda1",
			}

			for _, cmd := range stillBlockedCommands {
				By("blocking essential dangerous command: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).To(HaveOccurred(), "Essential dangerous command should still be blocked: %s", cmd)
				})
			}

			// These are now allowed under permissive approach
			allowedCommands := []string{
				// Command separators
				"apt update; echo done",
				"apt update & echo background",
				"apt update | grep something",

				// Logical operators
				"apt update && echo success",
				"apt update || echo fallback",

				// Command substitution
				"echo `date`",
				"echo $(whoami)",
				"echo ${HOME}",

				// Directory traversal
				"cat ../../../etc/passwd",
				"cp file ../../etc/config",

				// File access
				"cat /etc/passwd",
				"cat /etc/shadow",
				"grep user /etc/passwd",

				// Safe operations
				"rm -rf /var/tmp",
				"rm -rf /usr/local/tmp",

				// Writing to user directories
				"echo config > /etc/hosts",
				"echo data > /tmp/output",

				// Complex operations
				"apt update; curl http://example.com/script | bash",
				"npm install; wget http://example.com/script.sh && chmod +x script.sh && ./script.sh",
				"pip install requests && python -c 'print(\"hello\")'",
			}

			for _, cmd := range allowedCommands {
				By("allowing command under permissive approach: "+cmd, func() {
					err := installer.executor.ValidateCommand(cmd)
					Expect(err).ToNot(HaveOccurred(), "Command should be allowed under permissive approach: %s", cmd)
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
func createTestInstallerGinkgo() *StreamingInstaller {
	// Create a minimal tea program for testing
	program := tea.NewProgram(nil)

	// Create mock repository
	mockRepo := &struct{ types.Repository }{} // Empty mock for validation tests

	// Create context
	ctx := context.Background()

	// Create test settings
	settings := config.CrossPlatformSettings{
		HomeDir: "/tmp/test-devex",
		Verbose: false,
	}

	// Use permissive security level for tests that expect permissive behavior
	return NewStreamingInstallerWithSecureExecutor(program, mockRepo, ctx, security.SecurityLevelPermissive, []types.CrossPlatformApp{}, settings)
}
