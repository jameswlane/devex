package deb

import (
	"github.com/jameswlane/devex/pkg/testutils"
	"os/exec"
	"testing"
)

func TestInstallDeb(t *testing.T) {
	// Mock the exec.Command to simulate successful installation
	execCommand = testutils.MockExecCommand
	defer func() { execCommand = exec.Command }() // Reset after test

	// Test InstallDeb
	err := Install("/path/to/test.deb")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestInstallDeb_Error(t *testing.T) {
	// Mock the exec.Command to simulate an error during installation
	execCommand = testutils.MockCommandWithError
	defer func() { execCommand = exec.Command }() // Reset after test

	// Test InstallDeb with error
	err := Install("/path/to/test.deb")
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
}
