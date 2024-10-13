package apt

import (
	"github.com/jameswlane/devex/pkg/testutils"
	"testing"
)

func TestInstallViaApt(t *testing.T) {
	// Use the mock from testutils
	aptExecCommand = testutils.MockExecCommand

	err := Install("curl")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestUninstallViaApt(t *testing.T) {
	aptExecCommand = testutils.MockExecCommand

	err := Uninstall("curl")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestInstallViaApt_Error(t *testing.T) {
	aptExecCommand = testutils.MockCommandWithError

	err := Install("invalid-package")
	if err == nil {
		t.Errorf("Expected error when installing invalid package, but got none")
	}
}

func TestUninstallViaApt_Error(t *testing.T) {
	aptExecCommand = testutils.MockCommandWithError

	err := Uninstall("invalid-package")
	if err == nil {
		t.Errorf("Expected error when uninstalling invalid package, but got none")
	}
}
