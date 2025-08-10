package utilities_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
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

	t.Run("handles package variations with mocked commands", func(t *testing.T) {
		// Store original CommandExec
		originalExec := utils.CommandExec
		defer func() {
			utils.CommandExec = originalExec
		}()

		// Create mock executor
		mockExec := mocks.NewMockCommandExecutor()
		utils.CommandExec = mockExec

		// Test that docker variations work
		if !utilities.DefaultRegistry.HasHandler("docker") {
			t.Error("Expected docker handler to be available")
		}

		// Test execution with mocked commands
		err := utilities.ExecutePostInstallHandler("docker")
		// Should not fail with mocked commands
		if err != nil {
			t.Errorf("Expected no error with mocked commands, got %v", err)
		}
	})
}
