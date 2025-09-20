package mocks

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

type MockUtils struct {
	Commands                      []string
	FailingCommand                string
	CopyFileCalled                bool
	EnsureDirCalled               bool
	DownloadFileCalled            bool
	DownloadFileWithContextCalled bool
	RunShellCommandCalled         bool
	RunCommandCalled              bool
	DependenciesChecked           bool
}

func NewMockUtils() *MockUtils {
	return &MockUtils{}
}

func (m *MockUtils) RunShellCommand(command string) (string, error) {
	m.Commands = append(m.Commands, command)
	if command == m.FailingCommand {
		return "", fmt.Errorf("mock RunShellCommand failed")
	}
	return "mock shell output", nil
}

func (m *MockUtils) FailCommand(command string) {
	m.FailingCommand = command
}

func (m *MockUtils) ExecAsUser(command string, args ...string) (string, error) {
	fullCommand := fmt.Sprintf("%s %s", command, strings.Join(args, " "))
	m.Commands = append(m.Commands, fullCommand)
	if fullCommand == m.FailingCommand {
		return "", errors.New("mock ExecAsUser failed")
	}
	return fmt.Sprintf("Executed as user: %s", fullCommand), nil
}

func (m *MockUtils) ExecAsUserWithContext(ctx context.Context, command string, args ...string) (string, error) {
	fullCommand := fmt.Sprintf("%s %s", command, strings.Join(args, " "))
	m.Commands = append(m.Commands, fullCommand)
	if fullCommand == m.FailingCommand {
		return "", errors.New("mock ExecAsUserWithContext failed")
	}
	return fmt.Sprintf("Executed with context as user: %s", fullCommand), nil
}

func (m *MockUtils) DownloadFile(url, destination string) error {
	m.DownloadFileCalled = true
	if url == m.FailingCommand {
		return errors.New("mock DownloadFile failed")
	}
	return nil
}

func (m *MockUtils) DownloadFileWithContext(ctx context.Context, url, destination string) error {
	m.DownloadFileWithContextCalled = true
	if url == m.FailingCommand {
		return errors.New("mock DownloadFileWithContext failed")
	}
	return nil
}

func (m *MockUtils) IsAppInstalled(appName string) (bool, error) {
	return true, nil
}

func (m *MockUtils) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	m.RunCommandCalled = true
	// Simulate command execution
	return "mock-output", nil
}

func (m *MockUtils) CheckDependencies(dependencies []utils.Dependency) error {
	m.DependenciesChecked = true
	// Simulate all dependencies being available
	return nil
}

func (m *MockUtils) GetHomeDir() (string, error) {
	// Simulate fetching the home directory
	return "/home/mockuser", nil
}

func (m *MockUtils) GetShellRCPath(shellPath, homeDir string) (string, error) {
	// Simulate fetching the shell RC path
	if strings.Contains(shellPath, "bash") {
		return filepath.Join(homeDir, ".bashrc"), nil
	}
	return "", fmt.Errorf("unsupported shell: %s", shellPath)
}

func (m *MockUtils) GetUserShell(username string) (string, error) {
	// Simulate fetching the user's shell
	return "/bin/bash", nil
}

func (m *MockUtils) ProcessInstallCommands(commands []utils.InstallCommand) error {
	// Simulate processing installation commands
	for _, cmd := range commands {
		fmt.Printf("Processed command: %s\n", cmd.Shell)
	}
	return nil
}

func (m *MockUtils) ReplacePlaceholders(input string, placeholders map[string]string) string {
	// Simulate placeholder replacement
	output := input
	for key, value := range placeholders {
		output = strings.ReplaceAll(output, key, value)
	}
	return output
}

func (m *MockUtils) UpdateShellConfig(shellPath, homeDir string, commands []string) error {
	// Simulate updating shell configuration
	for _, command := range commands {
		fmt.Printf("Added command to shell config: %s\n", command)
	}
	return nil
}

func (m *MockUtils) EnsureDir(path string) error {
	m.EnsureDirCalled = true
	if path == m.FailingCommand {
		return fmt.Errorf("mock EnsureDir failed")
	}
	return nil
}

func (m *MockUtils) CopyFile(src, dest string) error {
	m.CopyFileCalled = true
	if src == m.FailingCommand {
		return fmt.Errorf("mock CopyFile failed")
	}
	return nil
}

func (m *MockUtils) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	m.Commands = append(m.Commands, command)
	if command == m.FailingCommand {
		return nil, fmt.Errorf("mock ExecuteCommand failed")
	}
	// Return a mock command that will not actually execute
	cmd := exec.CommandContext(ctx, "echo", "mock-execution")
	return cmd, nil
}

func (m *MockUtils) ValidateCommand(command string) error {
	if command == m.FailingCommand {
		return fmt.Errorf("mock ValidateCommand failed")
	}
	return nil
}
