package nixpkgs_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/installers/nixpkgs"
	"github.com/jameswlane/devex/pkg/mocks"
)

func TestNewNixpkgsInstaller(t *testing.T) {
	installer := nixpkgs.NewNixpkgsInstaller()
	if installer == nil {
		t.Errorf("NewNixpkgsInstaller() returned nil")
	}
}

func TestNixpkgsInstaller_Methods(t *testing.T) {
	installer := nixpkgs.NewNixpkgsInstaller()
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
