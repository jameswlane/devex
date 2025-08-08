package utilities

import (
	"errors"
	"fmt"
)

// Common installer error types for standardized error handling
var (
	// System validation errors
	ErrSystemValidation  = errors.New("system validation failed")
	ErrPackageManager    = errors.New("package manager not available")
	ErrDependencyMissing = errors.New("required dependency missing")

	// Package operation errors
	ErrPackageNotFound     = errors.New("package not found")
	ErrPackageInstalled    = errors.New("package already installed")
	ErrPackageNotInstalled = errors.New("package not installed")
	ErrInstallationFailed  = errors.New("installation failed")
	ErrUninstallFailed     = errors.New("uninstallation failed")
	ErrVerificationFailed  = errors.New("installation verification failed")

	// Repository errors
	ErrRepositoryAccess = errors.New("repository access failed")
	ErrRepositoryUpdate = errors.New("repository update failed")
	ErrRepositoryAdd    = errors.New("failed to add to repository")
	ErrRepositoryRemove = errors.New("failed to remove from repository")

	// Network errors
	ErrNetworkAccess = errors.New("network access failed")
	ErrDownload      = errors.New("download failed")

	// Permission errors
	ErrPermissionDenied = errors.New("permission denied")
	ErrFileAccess       = errors.New("file access failed")

	// Configuration errors
	ErrInvalidConfig    = errors.New("invalid configuration")
	ErrConfigNotFound   = errors.New("configuration not found")
	ErrPostInstallSetup = errors.New("post-install setup failed")
)

// ErrorType represents different categories of installer errors
type ErrorType string

const (
	ErrorTypeSystem        ErrorType = "system"
	ErrorTypePackage       ErrorType = "package"
	ErrorTypeRepository    ErrorType = "repository"
	ErrorTypeNetwork       ErrorType = "network"
	ErrorTypePermission    ErrorType = "permission"
	ErrorTypeConfiguration ErrorType = "configuration"
)

// InstallerError provides structured error information for installer operations
type InstallerError struct {
	Type        ErrorType
	Operation   string
	Package     string
	Installer   string
	Underlying  error
	Message     string
	Recoverable bool
	Suggestions []string
}

// Error implements the error interface
func (ie *InstallerError) Error() string {
	if ie.Message != "" {
		return ie.Message
	}

	baseMsg := fmt.Sprintf("%s operation failed", ie.Operation)
	if ie.Package != "" {
		baseMsg = fmt.Sprintf("%s operation failed for package '%s'", ie.Operation, ie.Package)
	}
	if ie.Installer != "" {
		baseMsg = fmt.Sprintf("%s %s operation failed for package '%s'", ie.Installer, ie.Operation, ie.Package)
	}

	if ie.Underlying != nil {
		return fmt.Sprintf("%s: %v", baseMsg, ie.Underlying)
	}

	return baseMsg
}

// Unwrap returns the underlying error for error wrapping
func (ie *InstallerError) Unwrap() error {
	return ie.Underlying
}

// Is implements error comparison for errors.Is
func (ie *InstallerError) Is(target error) bool {
	if target == nil {
		return false
	}

	// Check if target is one of our base errors
	switch target {
	case ErrSystemValidation, ErrPackageManager, ErrDependencyMissing:
		return ie.Type == ErrorTypeSystem
	case ErrPackageNotFound, ErrPackageInstalled, ErrPackageNotInstalled,
		ErrInstallationFailed, ErrUninstallFailed, ErrVerificationFailed:
		return ie.Type == ErrorTypePackage
	case ErrRepositoryAccess, ErrRepositoryUpdate, ErrRepositoryAdd, ErrRepositoryRemove:
		return ie.Type == ErrorTypeRepository
	case ErrNetworkAccess, ErrDownload:
		return ie.Type == ErrorTypeNetwork
	case ErrPermissionDenied, ErrFileAccess:
		return ie.Type == ErrorTypePermission
	case ErrInvalidConfig, ErrConfigNotFound, ErrPostInstallSetup:
		return ie.Type == ErrorTypeConfiguration
	}

	// Check if target is another InstallerError with same type
	if ie2, ok := target.(*InstallerError); ok {
		return ie.Type == ie2.Type
	}

	return false
}

// NewInstallerError creates a new structured installer error
func NewInstallerError(errorType ErrorType, operation, packageName, installerName string, underlying error) *InstallerError {
	return &InstallerError{
		Type:        errorType,
		Operation:   operation,
		Package:     packageName,
		Installer:   installerName,
		Underlying:  underlying,
		Recoverable: isRecoverable(errorType, underlying),
	}
}

// NewSystemError creates a system validation error
func NewSystemError(installer, message string, underlying error) *InstallerError {
	err := &InstallerError{
		Type:        ErrorTypeSystem,
		Operation:   "system validation",
		Installer:   installer,
		Underlying:  underlying,
		Message:     message,
		Recoverable: false,
	}

	// Add common suggestions for system errors
	err.Suggestions = []string{
		fmt.Sprintf("Ensure %s is installed and available in PATH", installer),
		"Check system requirements and dependencies",
		"Verify user has necessary permissions",
	}

	return err
}

// NewPackageError creates a package operation error
func NewPackageError(operation, packageName, installer string, underlying error) *InstallerError {
	err := &InstallerError{
		Type:        ErrorTypePackage,
		Operation:   operation,
		Package:     packageName,
		Installer:   installer,
		Underlying:  underlying,
		Recoverable: isPackageErrorRecoverable(operation, underlying),
	}

	// Add operation-specific suggestions
	switch operation {
	case "install":
		err.Suggestions = []string{
			"Update package metadata/cache",
			"Check package name spelling",
			"Verify package exists in configured repositories",
			"Check available disk space",
		}
	case "uninstall":
		err.Suggestions = []string{
			"Verify package is actually installed",
			"Check for dependent packages that prevent removal",
		}
	case "verification":
		err.Suggestions = []string{
			"Package may have installed but not be in expected location",
			"Try reinstalling the package",
			"Check package manager logs for details",
		}
	}

	return err
}

// NewRepositoryError creates a repository operation error
func NewRepositoryError(operation, packageName, installer string, underlying error) *InstallerError {
	return &InstallerError{
		Type:        ErrorTypeRepository,
		Operation:   operation,
		Package:     packageName,
		Installer:   installer,
		Underlying:  underlying,
		Recoverable: true,
		Suggestions: []string{
			"Check database connectivity",
			"Verify repository configuration",
			"Try operation again after a short delay",
		},
	}
}

// NewNetworkError creates a network-related error
func NewNetworkError(operation, packageName, installer string, underlying error) *InstallerError {
	return &InstallerError{
		Type:        ErrorTypeNetwork,
		Operation:   operation,
		Package:     packageName,
		Installer:   installer,
		Underlying:  underlying,
		Recoverable: true,
		Suggestions: []string{
			"Check internet connectivity",
			"Verify repository URLs are accessible",
			"Try again later if servers are temporarily unavailable",
			"Check firewall and proxy settings",
		},
	}
}

// isRecoverable determines if an error type is generally recoverable
func isRecoverable(errorType ErrorType, underlying error) bool {
	switch errorType {
	case ErrorTypeSystem:
		return false // System errors usually require manual intervention
	case ErrorTypePackage:
		return isPackageErrorRecoverable("", underlying)
	case ErrorTypeRepository, ErrorTypeNetwork:
		return true // Often transient issues
	case ErrorTypePermission:
		return false // Permission errors need manual fixes
	case ErrorTypeConfiguration:
		return true // Configuration can often be corrected
	default:
		return false
	}
}

// isPackageErrorRecoverable determines if a package error can be recovered from
func isPackageErrorRecoverable(operation string, underlying error) bool {
	if underlying == nil {
		return true
	}

	// Check for specific error patterns that indicate non-recoverable issues
	underlyingStr := underlying.Error()
	nonRecoverablePatterns := []string{
		"permission denied",
		"access denied",
		"disk full",
		"no space left",
		"not found in any repository",
		"package does not exist",
	}

	for _, pattern := range nonRecoverablePatterns {
		if contains(underlyingStr, pattern) {
			return false
		}
	}

	return true
}

// contains is a case-insensitive substring check
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					indexOf(s, substr) >= 0))
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// WrapError wraps an existing error with installer context
func WrapError(err error, errorType ErrorType, operation, packageName, installer string) error {
	if err == nil {
		return nil
	}

	return &InstallerError{
		Type:        errorType,
		Operation:   operation,
		Package:     packageName,
		Installer:   installer,
		Underlying:  err,
		Recoverable: isRecoverable(errorType, err),
	}
}
