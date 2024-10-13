package flatpak

import (
	"github.com/jameswlane/devex/pkg/testutils"
	"os/exec"
	"testing"
)

func TestInstallFlatpak(t *testing.T) {
	// Mock the exec.Command to simulate successful Flatpak installation
	flatpakExecCommand = testutils.MockExecCommand
	defer func() { flatpakExecCommand = exec.Command }() // Reset after test

	// Test InstallFlatpak
	err := Install("com.example.App", "flathub")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestInstallFlatpak_Error(t *testing.T) {
	// Mock the exec.Command to simulate an error during installation
	flatpakExecCommand = testutils.MockCommandWithError
	defer func() { flatpakExecCommand = exec.Command }() // Reset after test

	// Test InstallFlatpak with error
	err := Install("com.example.App", "flathub")
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
}
