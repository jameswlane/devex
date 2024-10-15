package check_install

import (
	"github.com/jameswlane/devex/pkg/testutils"
	"os/exec"
	"testing"
)

func TestIsAppInstalled(t *testing.T) {
	// Mock the exec.Command to simulate the app being installed
	checkInstallExecCommand = testutils.MockExecCommand
	defer func() { checkInstallExecCommand = exec.Command }() // Reset after test

	// Test IsAppInstalled (app is installed)
	installed, err := IsAppInstalled("testapp")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if !installed {
		t.Errorf("Expected app to be installed, but got false")
	}
}

func TestIsAppInstalled_NotInstalled(t *testing.T) {
	// Mock the exec.Command to simulate the app not being installed
	checkInstallExecCommand = testutils.MockCommandWithError
	defer func() { checkInstallExecCommand = exec.Command }() // Reset after test

	// Test IsAppInstalled (app is not installed)
	installed, err := IsAppInstalled("missingapp")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
	if installed {
		t.Errorf("Expected app to be not installed, but got true")
	}
}
