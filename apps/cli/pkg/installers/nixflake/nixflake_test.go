package nixflake_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/installers/nixflake"
	"github.com/jameswlane/devex/pkg/mocks"
)

func TestNewNixFlakeInstaller(t *testing.T) {
	installer := nixflake.NewNixFlakeInstaller()
	if installer == nil {
		t.Errorf("NewNixFlakeInstaller() returned nil")
	}
}

func TestNixFlakeInstaller_Methods(t *testing.T) {
	installer := nixflake.NewNixFlakeInstaller()
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

	// Test IsInstalled - should return an error for unimplemented installer
	_, err = installer.IsInstalled("test-package")
	if err == nil {
		t.Errorf("IsInstalled() should return an error for unimplemented installer")
	}
}
