package errors

import (
	"errors"
	"fmt"
)

// Wrap adds context to an existing error.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// New creates a new error.
func New(message string) error {
	return errors.New(message)
}

// Is checks if an error matches a target error.
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As attempts to map an error to a specific type.
func As(err error, target any) bool {
	return errors.As(err, target)
}

// Unwrap retrieves the underlying error, if present.
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

var (
	// Common Errors
	ErrInvalidInput     = New("invalid input")
	ErrDependencyFailed = New("dependency failed")
	ErrFileNotFound     = New("file not found")

	// Installer Errors
	ErrUnsupportedInstaller = New("unsupported installer")
	ErrInstallFailed        = New("installation failed")
	ErrHookExecutionFailed  = New("hook execution failed")

	// Database Errors
	ErrDatabaseConnection = New("database connection failed")
	ErrQueryExecution     = New("query execution failed")
)
