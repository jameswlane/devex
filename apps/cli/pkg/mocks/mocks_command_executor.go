package mocks

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type MockCommandExecutor struct {
	Commands          []string        // Stores commands executed
	FailingCommand    string          // Command that should fail
	InstallationState map[string]bool // Track package installation state
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		InstallationState: make(map[string]bool),
	}
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

	// Handle apt-get install commands - mark packages as installed
	if strings.Contains(command, "sudo apt-get install -y") {
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			packageName := parts[len(parts)-1] // Last argument is the package name
			m.InstallationState[packageName] = true
		}
		return "Reading package lists...\nBuilding dependency tree...\nPackage installed successfully", nil
	}

	// Handle dpkg-query commands for installation verification
	if strings.Contains(command, "dpkg-query -W -f='${Status}'") {
		// Extract package name from command
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[len(parts)-1]
			if m.InstallationState[packageName] {
				return "install ok installed", nil
			}
		}
		// For other packages, return not installed
		return "", fmt.Errorf("dpkg-query: no packages found matching package")
	}

	// Handle dpkg -l commands (alternative installation check)
	if strings.Contains(command, "dpkg -l") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[len(parts)-1]
			if m.InstallationState[packageName] {
				return fmt.Sprintf("ii  %s    1.0.0    amd64    Test package description", packageName), nil
			}
		}
		// For other packages, return not found
		return "", fmt.Errorf("dpkg-query: no packages found matching package")
	}

	// Handle dpkg --print-architecture (used by APT source functions)
	if strings.Contains(command, "dpkg --print-architecture") {
		return "amd64", nil
	}

	// Handle lsb_release commands (used for codename detection)
	if strings.Contains(command, "lsb_release -cs") {
		return "focal", nil
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

	// For test URLs, simulate creating a mock GPG key file
	if strings.Contains(url, "example.com") || strings.Contains(url, "test") {
		// Create a mock GPG key content to satisfy file size checks
		_ = "-----BEGIN PGP PUBLIC KEY BLOCK-----\nMock GPG key content for testing\n-----END PGP PUBLIC KEY BLOCK-----"
		// We don't actually write to filesystem in tests but this simulates success
	}

	return nil
}
