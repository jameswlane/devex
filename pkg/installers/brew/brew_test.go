package brew

import (
	"github.com/jameswlane/devex/pkg/testutils"
	"testing"
)

func TestInstallViaBrew(t *testing.T) {
	brewExecCommand = testutils.MockExecCommand

	err := Install("git")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestUninstallViaBrew(t *testing.T) {
	brewExecCommand = testutils.MockExecCommand

	err := Uninstall("git")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}

func TestInstallViaBrew_Error(t *testing.T) {
	brewExecCommand = testutils.MockCommandWithError

	err := Install("invalid-package")
	if err == nil {
		t.Errorf("Expected error when installing invalid package, but got none")
	}
}

func TestUninstallViaBrew_Error(t *testing.T) {
	brewExecCommand = testutils.MockCommandWithError

	err := Uninstall("invalid-package")
	if err == nil {
		t.Errorf("Expected error when uninstalling invalid package, but got none")
	}
}
