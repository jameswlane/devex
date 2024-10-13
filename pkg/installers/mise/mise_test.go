package mise

import (
	"os/exec"
	"testing"
)

// Mock function to simulate exec.Command for Mise installation
func mockMiseExecCommand(command string, args ...string) *exec.Cmd {
	return exec.Command("echo", "mocked command") // Simulate successful execution
}

func TestInstallMiseLanguage(t *testing.T) {
	// Mock miseExecCommand to avoid actually running shell commands
	miseExecCommand = mockMiseExecCommand
	defer func() { miseExecCommand = exec.Command }() // Reset after test

	// Test installing a language with Mise
	err := InstallMiseLanguage("python@latest")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestRunPostInstallCommand(t *testing.T) {
	// Mock miseExecCommand to avoid actually running shell commands
	miseExecCommand = mockMiseExecCommand
	defer func() { miseExecCommand = exec.Command }() // Reset after test

	// Test running a post-install command
	err := RunPostInstallCommand("mise x ruby -- gem install rails --no-document")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}
