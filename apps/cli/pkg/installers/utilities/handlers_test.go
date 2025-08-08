package utilities_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/installers/utilities"
)

func TestHandlerRegistry(t *testing.T) {
	t.Run("creates new registry with default handlers", func(t *testing.T) {
		registry := utilities.NewHandlerRegistry()

		// Check that default Docker handlers are registered
		if !registry.HasHandler("docker") {
			t.Error("Expected docker handler to be registered")
		}

		if !registry.HasHandler("docker-ce") {
			t.Error("Expected docker-ce handler to be registered")
		}

		if !registry.HasHandler("nginx") {
			t.Error("Expected nginx handler to be registered")
		}
	})

	t.Run("registers and executes custom handlers", func(t *testing.T) {
		registry := utilities.NewHandlerRegistry()

		executed := false
		testHandler := func() error {
			executed = true
			return nil
		}

		registry.Register("test-package", testHandler)

		if !registry.HasHandler("test-package") {
			t.Error("Expected test-package handler to be registered")
		}

		err := registry.Execute("test-package")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !executed {
			t.Error("Expected handler to be executed")
		}
	})

	t.Run("returns nil for packages without handlers", func(t *testing.T) {
		registry := utilities.NewHandlerRegistry()

		err := registry.Execute("nonexistent-package")
		if err != nil {
			t.Errorf("Expected no error for nonexistent package, got %v", err)
		}
	})
}

func TestExecutePostInstallHandler(t *testing.T) {
	t.Run("executes handler for registered package", func(t *testing.T) {
		executed := false
		testHandler := func() error {
			executed = true
			return nil
		}

		utilities.RegisterHandler("test-registry-package", testHandler)

		err := utilities.ExecutePostInstallHandler("test-registry-package")
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		if !executed {
			t.Error("Expected handler to be executed")
		}
	})

	t.Run("handles package variations", func(t *testing.T) {
		// Test that docker variations work
		if !utilities.DefaultRegistry.HasHandler("docker") {
			t.Error("Expected docker handler to be available")
		}

		// Test execution doesn't fail (we can't easily test the actual system commands)
		err := utilities.ExecutePostInstallHandler("docker")
		// We expect this might fail in test environment, but shouldn't panic
		if err != nil {
			t.Logf("docker handler execution returned error (expected in test env): %v", err)
		}
	})
}
