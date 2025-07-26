package shell_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/shell"
	"github.com/jameswlane/devex/pkg/types"
)

var _ = Describe("SwitchToZsh", func() {
	var mockExecutor *mockCommandExecutor
	var mockChecker *mockAppChecker

	BeforeEach(func() {
		mockExecutor = &mockCommandExecutor{}
		mockChecker = &mockAppChecker{}
	})

	It("installs Zsh if not already installed", func() {
		mockChecker.AppNotInstalled = true

		err := shell.SwitchToZsh(mockExecutor, mockChecker)
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("sudo apt-get install -y zsh"))
		Expect(mockExecutor.Commands).To(ContainElement("chsh -s /bin/zsh"))
	})

	It("does not install Zsh if already installed", func() {
		mockChecker.AppNotInstalled = false

		err := shell.SwitchToZsh(mockExecutor, mockChecker)
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).ToNot(ContainElement("sudo apt-get install -y zsh"))
		Expect(mockExecutor.Commands).To(ContainElement("chsh -s /bin/zsh"))
	})

	It("returns an error if Zsh installation fails", func() {
		mockChecker.AppNotInstalled = true
		mockExecutor.FailCommand("sudo apt-get install -y zsh")

		err := shell.SwitchToZsh(mockExecutor, mockChecker)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to install Zsh"))
	})

	It("returns an error if switching to Zsh fails", func() {
		mockChecker.AppNotInstalled = false
		mockExecutor.FailCommand("chsh -s /bin/zsh")

		err := shell.SwitchToZsh(mockExecutor, mockChecker)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to switch shell to Zsh"))
	})
})

type mockCommandExecutor struct {
	Commands       []string
	FailingCommand string
}

func (m *mockCommandExecutor) RunShellCommand(cmd string) (string, error) {
	m.Commands = append(m.Commands, cmd)
	if cmd == m.FailingCommand {
		return "", fmt.Errorf("mock command failed: %s", cmd)
	}
	return "", nil
}

func (m *mockCommandExecutor) FailCommand(command string) {
	m.FailingCommand = command
}

type mockAppChecker struct {
	AppNotInstalled bool
}

func (m *mockAppChecker) IsAppInstalled(appConfig types.AppConfig) (bool, error) {
	return !m.AppNotInstalled, nil
}
