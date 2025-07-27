package shell_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/shell"
)

var _ = Describe("SwitchToZsh", func() {
	var mockExecutor *mockCommandExecutor

	BeforeEach(func() {
		mockExecutor = &mockCommandExecutor{}
	})

	It("installs Zsh if not already installed", func() {
		mockExecutor.FailCommand("which zsh") // Simulate zsh not found

		err := shell.SwitchToZsh(mockExecutor)
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("which zsh"))
		Expect(mockExecutor.Commands).To(ContainElement("sudo apt-get install -y zsh"))
		Expect(mockExecutor.Commands).To(ContainElement("chsh -s /bin/zsh"))
	})

	It("does not install Zsh if already installed", func() {
		// Don't fail the which command, simulating zsh is installed

		err := shell.SwitchToZsh(mockExecutor)
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("which zsh"))
		Expect(mockExecutor.Commands).ToNot(ContainElement("sudo apt-get install -y zsh"))
		Expect(mockExecutor.Commands).To(ContainElement("chsh -s /bin/zsh"))
	})

	It("returns an error if Zsh installation fails", func() {
		mockExecutor.FailCommand("which zsh")                   // Zsh not found
		mockExecutor.FailCommand("sudo apt-get install -y zsh") // Installation fails

		err := shell.SwitchToZsh(mockExecutor)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to install Zsh"))
	})

	It("returns an error if switching to Zsh fails", func() {
		// Zsh is installed but switching fails
		mockExecutor.FailCommand("chsh -s /bin/zsh")

		err := shell.SwitchToZsh(mockExecutor)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to switch shell to Zsh"))
	})
})

type mockCommandExecutor struct {
	Commands        []string
	FailingCommands map[string]bool
}

func (m *mockCommandExecutor) RunShellCommand(cmd string) (string, error) {
	m.Commands = append(m.Commands, cmd)
	if m.FailingCommands[cmd] {
		return "", fmt.Errorf("mock command failed: %s", cmd)
	}
	return "", nil
}

func (m *mockCommandExecutor) FailCommand(command string) {
	if m.FailingCommands == nil {
		m.FailingCommands = make(map[string]bool)
	}
	m.FailingCommands[command] = true
}
