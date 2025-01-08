package mocks

import (
	"context"
	"errors"
	"fmt"
)

type MockCommandExecutor struct {
	Commands       []string // Stores commands executed
	FailingCommand string   // Command that should fail
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{}
}

func (m *MockCommandExecutor) RunShellCommand(command string) (string, error) {
	m.Commands = append(m.Commands, command)
	if command == m.FailingCommand {
		return "", fmt.Errorf("mock shell command failed: %s", command)
	}
	return "mock output", nil
}

func (m *MockCommandExecutor) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	command := fmt.Sprintf("%s %s", name, args)
	m.Commands = append(m.Commands, command)
	if command == m.FailingCommand {
		return "", errors.New("mock command failed")
	}
	return "mock output", nil
}

func (m *MockCommandExecutor) DownloadFileWithContext(ctx context.Context, url, filepath string) error {
	// Simulate a successful or failing file download based on the URL
	if url == m.FailingCommand {
		return fmt.Errorf("mock download failed for url: %s", url)
	}
	m.Commands = append(m.Commands, fmt.Sprintf("download %s to %s", url, filepath))
	return nil
}
