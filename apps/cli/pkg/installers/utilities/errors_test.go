package utilities_test

import (
	"errors"
	"testing"

	"github.com/jameswlane/devex/pkg/installers/utilities"
)

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestInstallerError(t *testing.T) {
	t.Run("creates system error with suggestions", func(t *testing.T) {
		underlying := errors.New("command not found")
		err := utilities.NewSystemError("apt", "APT not available", underlying)

		if err.Error() == "" {
			t.Error("Expected non-empty error message")
		}

		if len(err.Suggestions) == 0 {
			t.Error("Expected suggestions for system error")
		}

		if err.Recoverable {
			t.Error("System errors should not be recoverable")
		}

		// Test error wrapping
		if !errors.Is(err, underlying) {
			t.Error("Should wrap underlying error")
		}
	})

	t.Run("creates package error with operation-specific suggestions", func(t *testing.T) {
		underlying := errors.New("package not found")
		err := utilities.NewPackageError("install", "nginx", "apt", underlying)

		if err.Package != "nginx" {
			t.Errorf("Expected package 'nginx', got '%s'", err.Package)
		}

		if err.Installer != "apt" {
			t.Errorf("Expected installer 'apt', got '%s'", err.Installer)
		}

		if err.Operation != "install" {
			t.Errorf("Expected operation 'install', got '%s'", err.Operation)
		}

		// Should have install-specific suggestions
		found := false
		for _, suggestion := range err.Suggestions {
			if indexOf(suggestion, "package name") >= 0 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected install-specific suggestions")
		}
	})

	t.Run("creates repository error with appropriate recoverability", func(t *testing.T) {
		underlying := errors.New("database connection failed")
		err := utilities.NewRepositoryError("add", "nginx", "apt", underlying)

		if !err.Recoverable {
			t.Error("Repository errors should be recoverable")
		}

		if len(err.Suggestions) == 0 {
			t.Error("Expected suggestions for repository error")
		}
	})

	t.Run("creates network error with retry suggestions", func(t *testing.T) {
		underlying := errors.New("connection timeout")
		err := utilities.NewNetworkError("download", "package", "apt", underlying)

		if !err.Recoverable {
			t.Error("Network errors should be recoverable")
		}

		// Should have network-specific suggestions
		found := false
		for _, suggestion := range err.Suggestions {
			if indexOf(suggestion, "internet connectivity") >= 0 {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected network-specific suggestions")
		}
	})

	t.Run("wraps existing errors with context", func(t *testing.T) {
		original := errors.New("original error")
		wrapped := utilities.WrapError(original, utilities.ErrorTypePackage, "install", "test", "apt")

		if wrapped == nil {
			t.Error("Expected wrapped error, got nil")
		}

		if !errors.Is(wrapped, original) {
			t.Error("Should maintain error chain")
		}

		installerErr := &utilities.InstallerError{}
		ok := errors.As(wrapped, &installerErr)
		if !ok {
			t.Error("Expected InstallerError type")
		}

		if installerErr.Package != "test" {
			t.Errorf("Expected package 'test', got '%s'", installerErr.Package)
		}
	})

	t.Run("handles nil error wrapping", func(t *testing.T) {
		wrapped := utilities.WrapError(nil, utilities.ErrorTypePackage, "install", "test", "apt")
		if wrapped != nil {
			t.Error("Wrapping nil error should return nil")
		}
	})
}

func TestErrorTypeComparison(t *testing.T) {
	t.Run("compares error types correctly", func(t *testing.T) {
		systemErr := utilities.NewSystemError("apt", "system error", nil)
		packageErr := utilities.NewPackageError("install", "nginx", "apt", nil)

		// Test Is() method with base errors
		if !errors.Is(systemErr, utilities.ErrSystemValidation) {
			t.Error("System error should match ErrSystemValidation")
		}

		if !errors.Is(packageErr, utilities.ErrInstallationFailed) {
			t.Error("Package error should match ErrInstallationFailed")
		}

		if errors.Is(systemErr, utilities.ErrInstallationFailed) {
			t.Error("System error should not match package error types")
		}
	})

	t.Run("compares InstallerError types", func(t *testing.T) {
		err1 := utilities.NewSystemError("apt", "error1", nil)
		err2 := utilities.NewSystemError("dnf", "error2", nil)
		packageErr := utilities.NewPackageError("install", "nginx", "apt", nil)

		if !errors.Is(err1, err2) {
			t.Error("Same type errors should match")
		}

		if errors.Is(err1, packageErr) {
			t.Error("Different type errors should not match")
		}
	})
}

func TestRecoverabilityDetermination(t *testing.T) {
	t.Run("identifies non-recoverable errors", func(t *testing.T) {
		nonRecoverable := []error{
			errors.New("permission denied"),
			errors.New("disk full"),
			errors.New("package does not exist"),
		}

		for _, err := range nonRecoverable {
			packageErr := utilities.NewPackageError("install", "test", "apt", err)
			if packageErr.Recoverable {
				t.Errorf("Error '%v' should not be recoverable", err)
			}
		}
	})

	t.Run("identifies recoverable errors", func(t *testing.T) {
		recoverable := []error{
			errors.New("network timeout"),
			errors.New("temporary failure"),
			errors.New("server unavailable"),
		}

		for _, err := range recoverable {
			packageErr := utilities.NewPackageError("install", "test", "apt", err)
			if !packageErr.Recoverable {
				t.Errorf("Error '%v' should be recoverable", err)
			}
		}
	})
}
