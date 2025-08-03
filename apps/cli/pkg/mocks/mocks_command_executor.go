package mocks

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

	// Handle specific command patterns for realistic mock responses
	if strings.Contains(command, "apt-cache policy") {
		if strings.Contains(command, "failing-package") {
			// Return output indicating package is not available
			return `N: Unable to locate package failing-package`, nil
		}
		// Return mock apt-cache policy output that indicates package is available
		return `test-package:
  Installed: (none)
  Candidate: 1.0.0
  Version table:
     1.0.0 500
        500 http://archive.ubuntu.com/ubuntu focal/main amd64 Packages`, nil
	}

	if strings.Contains(command, "which") {
		// Most which commands should succeed
		return "/usr/bin/command", nil
	}

	if strings.Contains(command, "dpkg --version") {
		return "Debian dpkg package management program version 1.20.5", nil
	}

	if command == "whoami" {
		return "testuser", nil
	}

	if strings.Contains(command, "systemctl") {
		// Mock systemctl commands for Docker setup
		return "mock systemctl output", nil
	}

	if strings.Contains(command, "docker.io") && strings.Contains(command, "apt-cache policy") {
		// Return mock apt-cache policy output for docker.io package
		return `docker.io:
  Installed: (none)
  Candidate: 20.10.12-0ubuntu2~20.04.1
  Version table:
     20.10.12-0ubuntu2~20.04.1 500
        500 http://archive.ubuntu.com/ubuntu focal-updates/universe amd64 Packages`, nil
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
