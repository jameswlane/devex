package rpm_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/installers/rpm"
	"github.com/jameswlane/devex/pkg/mocks"
)

func TestNewRpmInstaller(t *testing.T) {
	installer := rpm.NewRpmInstaller()
	if installer == nil {
		t.Errorf("NewRpmInstaller() returned nil")
	}
}

func TestRpmInstaller_Methods(t *testing.T) {
	installer := rpm.NewRpmInstaller()
	mockRepo := &mocks.MockRepository{}

	// Test Install - should return not implemented error
	err := installer.Install("test-package", mockRepo)
	if err == nil {
		t.Errorf("Install() should return an error for unimplemented installer")
	}

	// Test Uninstall - should return not implemented error
	err = installer.Uninstall("test-package", mockRepo)
	if err == nil {
		t.Errorf("Uninstall() should return an error for unimplemented installer")
	}

	// Test IsInstalled - should work and return false when package not found
	// Note: IsInstalled is implemented to check RPM packages, so it doesn't return an error
	// when the system validation fails, it returns (false, nil) for not found packages
	installed, err := installer.IsInstalled("test-package")
	if err != nil {
		// This is expected if RPM system is not available in test environment
		t.Logf("IsInstalled() returned error (expected in test env without RPM): %v", err)
	} else {
		// If no error, package should not be installed
		if installed {
			t.Errorf("IsInstalled() should return false for non-existent package")
		}
	}
}
