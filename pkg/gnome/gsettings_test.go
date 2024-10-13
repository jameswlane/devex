package gnome

import (
	"github.com/jameswlane/devex/pkg/testutils"
	"os/exec"
	"testing"
)

func TestSetGSetting(t *testing.T) {
	// Mock the exec.Command to simulate successful gsettings call
	gsettingsExecCommand = testutils.MockExecCommand
	defer func() { gsettingsExecCommand = exec.Command }() // Reset after test

	// Test SetGSetting
	err := SetGSetting("org.gnome.desktop.interface", "gtk-theme", "Adwaita")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestSetGSetting_Error(t *testing.T) {
	// Mock the exec.Command to simulate an error during gsettings call
	gsettingsExecCommand = testutils.MockCommandWithError
	defer func() { gsettingsExecCommand = exec.Command }() // Reset after test

	// Test SetGSetting with error
	err := SetGSetting("org.gnome.desktop.interface", "gtk-theme", "InvalidTheme")
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
}
