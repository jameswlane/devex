package tui

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("GPG Key Command Validation", func() {
	var executor *SecureCommandExecutor

	BeforeEach(func() {
		// Use permissive security level for tests that expect permissive behavior
		executor = NewSecureCommandExecutor(security.SecurityLevelPermissive, []types.CrossPlatformApp{})
	})

	Context("Safe GPG key installation commands", func() {
		It("should allow curl with apt-key add", func() {
			command := "curl -fsSL https://example.com/key.asc | sudo apt-key add -"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow wget with apt-key add", func() {
			command := "wget -qO- https://example.com/key.asc | sudo apt-key add -"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow curl with gpg dearmor", func() {
			command := "curl -fsSL https://example.com/key.asc | gpg --dearmor -o /etc/apt/keyrings/key.gpg"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow wget with gpg dearmor", func() {
			command := "wget -qO- https://example.com/key.asc | gpg --dearmor -o /etc/apt/keyrings/key.gpg"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Safe bash -c system information commands", func() {
		It("should allow bash -c with os-release (single quotes)", func() {
			command := "bash -c '. /etc/os-release && echo $VERSION_CODENAME'"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow bash -c with os-release (double quotes)", func() {
			command := `bash -c ". /etc/os-release && echo $VERSION_CODENAME"`
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow bash -c with UBUNTU_CODENAME", func() {
			command := "bash -c '. /etc/os-release && echo $UBUNTU_CODENAME'"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Permissive validation approach", func() {
		It("should allow curl piping to shell under permissive approach", func() {
			command := "curl -fsSL https://example.com/script.sh | bash"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred(), "Curl piping should be allowed under permissive approach")
		})

		It("should allow wget piping to shell under permissive approach", func() {
			command := "wget -qO- https://example.com/script.sh | sh"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred(), "Wget piping should be allowed under permissive approach")
		})

		It("should allow pipes with safe commands", func() {
			command := "echo data | bash -c 'rm -rf /tmp/test'"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred(), "Safe pipes should be allowed under permissive approach")
		})

		It("should allow pipes writing to user filesystem", func() {
			command := "echo content | bash -c 'cat > /tmp/output'"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred(), "Pipes to user filesystem should be allowed")
		})

		It("should allow bash -c commands under permissive approach", func() {
			command := "bash -c 'curl -fsSL https://example.com/script.sh | sh'"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred(), "Bash -c should be allowed under permissive approach")
		})

		It("should only block truly dangerous operations", func() {
			// This should still be blocked because it targets system root
			command := "bash -c 'export PATH=$PATH && rm -rf /'"
			err := executor.ValidateCommand(command)
			Expect(err).To(HaveOccurred(), "rm -rf / should still be blocked")
		})
	})

	Context("Mise language installation commands", func() {
		It("should allow mise PATH setup and use command (single quotes)", func() {
			command := `bash -c 'export PATH="$HOME/.local/bin:$PATH" && if command -v mise >/dev/null 2>&1; then mise use --global go@latest; else echo "mise not found in PATH"; exit 1; fi'`
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow mise PATH setup and install command (single quotes)", func() {
			command := `bash -c 'export PATH="$HOME/.local/bin:$PATH" && mise install python@latest'`
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow mise uninstall command (single quotes)", func() {
			command := `bash -c 'export PATH="$HOME/.local/bin:$PATH" && mise uninstall node@18'`
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow mise conditional installation (double quotes)", func() {
			command := `bash -c "export PATH=\"$HOME/.local/bin:$PATH\" && if command -v mise >/dev/null 2>&1; then mise use --global ruby@latest; fi"`
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow mise PATH with conditional check", func() {
			command := `bash -c 'export PATH="$HOME/.local/bin:$PATH" && if command -v mise >/dev/null 2>&1; then mise install java@latest; else echo "mise not found"; exit 1; fi'`
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Specific failing apps from log", func() {
		It("should allow Eza GPG key command", func() {
			// This simulates the command that was failing for Eza
			command := "curl -fsSL https://raw.githubusercontent.com/eza-community/eza/main/deb.asc | sudo apt-key add -"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow GitHub CLI GPG key command", func() {
			// This simulates the command that was failing for GitHub CLI
			command := "curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo apt-key add -"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow Mise GPG key command", func() {
			// This simulates the command that was failing for Mise
			command := "curl -fsSL https://mise.jdx.dev/gpg-key.pub | sudo apt-key add -"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should allow Ulauncher GPG key command", func() {
			// This simulates the command that was failing for Ulauncher
			command := "curl -fsSL http://keyserver.ubuntu.com/pks/lookup?op=get&search=0xfaf1020699503176 | sudo apt-key add -"
			err := executor.ValidateCommand(command)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
