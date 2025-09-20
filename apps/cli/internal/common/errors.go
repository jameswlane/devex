package common

import "fmt"

// InstallerError represents different types of installer errors
type InstallerError struct {
	Type        ErrorType
	Installer   string
	Package     string
	Underlying  error
	Message     string
	Suggestions []string
}

// ErrorType defines the category of installer error
type ErrorType int

const (
	// ErrorTypeSystemNotFound indicates the package manager is not available
	ErrorTypeSystemNotFound ErrorType = iota
	// ErrorTypeSystemNotFunctional indicates the package manager is present but not working
	ErrorTypeSystemNotFunctional
	// ErrorTypePackageNotFound indicates the package was not found in repositories
	ErrorTypePackageNotFound
	// ErrorTypePackageAlreadyInstalled indicates the package is already installed
	ErrorTypePackageAlreadyInstalled
	// ErrorTypePackageNotInstalled indicates the package is not installed
	ErrorTypePackageNotInstalled
	// ErrorTypeInstallationFailed indicates the installation command failed
	ErrorTypeInstallationFailed
	// ErrorTypeUninstallationFailed indicates the uninstallation command failed
	ErrorTypeUninstallationFailed
	// ErrorTypeValidationFailed indicates package validation failed
	ErrorTypeValidationFailed
	// ErrorTypeNotImplemented indicates the installer is not yet implemented
	ErrorTypeNotImplemented
	// ErrorTypePermissionDenied indicates insufficient permissions
	ErrorTypePermissionDenied
	// ErrorTypeRepositoryError indicates repository-related errors
	ErrorTypeRepositoryError
)

// Error returns the error message
func (e *InstallerError) Error() string {
	if e.Message != "" {
		return e.Message
	}

	switch e.Type {
	case ErrorTypeSystemNotFound:
		return fmt.Sprintf("%s package manager not found on system", e.Installer)
	case ErrorTypeSystemNotFunctional:
		return fmt.Sprintf("%s package manager found but not functional", e.Installer)
	case ErrorTypePackageNotFound:
		return fmt.Sprintf("package '%s' not found in %s repositories", e.Package, e.Installer)
	case ErrorTypePackageAlreadyInstalled:
		return fmt.Sprintf("package '%s' is already installed via %s", e.Package, e.Installer)
	case ErrorTypePackageNotInstalled:
		return fmt.Sprintf("package '%s' is not installed via %s", e.Package, e.Installer)
	case ErrorTypeInstallationFailed:
		return fmt.Sprintf("failed to install package '%s' via %s", e.Package, e.Installer)
	case ErrorTypeUninstallationFailed:
		return fmt.Sprintf("failed to uninstall package '%s' via %s", e.Package, e.Installer)
	case ErrorTypeValidationFailed:
		return fmt.Sprintf("validation failed for package '%s' in %s", e.Package, e.Installer)
	case ErrorTypeNotImplemented:
		return fmt.Sprintf("%s installer not yet implemented", e.Installer)
	case ErrorTypePermissionDenied:
		return fmt.Sprintf("insufficient permissions for %s operation on '%s'", e.Installer, e.Package)
	case ErrorTypeRepositoryError:
		return fmt.Sprintf("repository error in %s for package '%s'", e.Installer, e.Package)
	default:
		return fmt.Sprintf("unknown error in %s installer for package '%s'", e.Installer, e.Package)
	}
}

// Unwrap returns the underlying error
func (e *InstallerError) Unwrap() error {
	return e.Underlying
}

// GetSuggestions returns helpful suggestions for resolving the error
func (e *InstallerError) GetSuggestions() []string {
	if len(e.Suggestions) > 0 {
		return e.Suggestions
	}

	switch e.Type {
	case ErrorTypeSystemNotFound:
		return []string{
			fmt.Sprintf("Install %s package manager for your distribution", e.Installer),
			"Check if you're running on the correct Linux distribution",
			"Verify system PATH includes package manager binaries",
		}
	case ErrorTypeSystemNotFunctional:
		return []string{
			fmt.Sprintf("Try running '%s --version' to check %s status", e.Installer, e.Installer),
			"Check system logs for package manager errors",
			"Restart package manager services if applicable",
		}
	case ErrorTypePackageNotFound:
		return []string{
			fmt.Sprintf("Update package repositories: '%s update' or equivalent", e.Installer),
			"Check package name spelling and availability",
			"Search for similar packages or alternatives",
		}
	case ErrorTypePermissionDenied:
		return []string{
			"Run command with sudo privileges",
			"Check if your user is in required groups (e.g., docker)",
			"Verify file system permissions",
		}
	case ErrorTypeNotImplemented:
		return []string{
			"Use manual installation commands provided in logs",
			"Contribute to the project by implementing this installer",
			"Use alternative package managers if available",
		}
	default:
		return []string{"Check logs for more detailed error information"}
	}
}

// NewInstallerError creates a new installer error with the specified type
func NewInstallerError(errorType ErrorType, installer, packageName string, cause error) *InstallerError {
	return &InstallerError{
		Type:       errorType,
		Installer:  installer,
		Package:    packageName,
		Underlying: cause,
	}
}

// NewInstallerErrorWithCause creates a new installer error with an underlying cause
func NewInstallerErrorWithCause(errorType ErrorType, installer, packageName string, cause error) *InstallerError {
	return &InstallerError{
		Type:       errorType,
		Installer:  installer,
		Package:    packageName,
		Underlying: cause,
	}
}

// NewInstallerErrorWithSuggestions creates a new installer error with custom suggestions
func NewInstallerErrorWithSuggestions(errorType ErrorType, installer, packageName string, suggestions []string) *InstallerError {
	return &InstallerError{
		Type:        errorType,
		Installer:   installer,
		Package:     packageName,
		Suggestions: suggestions,
	}
}
