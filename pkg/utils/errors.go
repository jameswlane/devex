package utils

import "errors"

// General Errors
var (
	ErrFileNotFound     = errors.New("file not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrUnsupportedShell = errors.New("unsupported shell")
)

// Command Execution Errors
var (
	ErrCommandFailed  = errors.New("command execution failed")
	ErrCommandTimeout = errors.New("command execution timed out")
)

// Download Errors
var (
	ErrDownloadFailed = errors.New("failed to download file")
	ErrInvalidURL     = errors.New("invalid URL")
)

// User Errors
var (
	ErrUserNotFound = errors.New("user not found")
)
