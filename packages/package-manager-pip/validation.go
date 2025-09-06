package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// MaxPackageNameLength defines the maximum allowed package name length
	MaxPackageNameLength = 214

	// MaxSearchTermLength defines the maximum allowed search term length
	MaxSearchTermLength = 100

	// MaxVenvNameLength defines the maximum allowed virtual environment name length
	MaxVenvNameLength = 100

	// MaxFilePathLength defines the maximum allowed file path length
	MaxFilePathLength = 4096
)

// validatePackageName validates Python package names
func (p *PipPlugin) validatePackageName(packageName string) error {
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}

	if len(packageName) > MaxPackageNameLength {
		return fmt.Errorf("package name too long (max %d characters)", MaxPackageNameLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range packageName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("package name contains invalid characters")
		}
	}

	// Python package names follow PEP 508 naming conventions
	// Allow package names with version specifiers and extras
	// Format: package[extras]==version or package>=version etc.
	validPackageRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?(\[[a-zA-Z0-9,._-]*\])?(==|>=|<=|!=|~=|>|<)?[a-zA-Z0-9._-]*$`)
	if !validPackageRegex.MatchString(packageName) {
		return fmt.Errorf("invalid Python package name format")
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(packageName, char) {
			return fmt.Errorf("package name contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// validateSearchTerm validates search terms to prevent injection
func (p *PipPlugin) validateSearchTerm(searchTerm string) error {
	if searchTerm == "" {
		return fmt.Errorf("search term cannot be empty")
	}

	if len(searchTerm) > MaxSearchTermLength {
		return fmt.Errorf("search term too long (max %d characters)", MaxSearchTermLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range searchTerm {
		if r == 0 {
			return fmt.Errorf("search term contains null bytes")
		}
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(searchTerm, char) {
			return fmt.Errorf("search term contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// validateVenvName validates virtual environment names
func (p *PipPlugin) validateVenvName(venvName string) error {
	if venvName == "" {
		return fmt.Errorf("virtual environment name cannot be empty")
	}

	if len(venvName) > MaxVenvNameLength {
		return fmt.Errorf("virtual environment name too long (max %d characters)", MaxVenvNameLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range venvName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("virtual environment name contains invalid characters")
		}
	}

	// Virtual environment names should be valid directory names
	validVenvRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
	if !validVenvRegex.MatchString(venvName) {
		return fmt.Errorf("invalid virtual environment name format: must start with alphanumeric and contain only letters, numbers, dots, hyphens, and underscores")
	}

	// Prevent names that could cause issues
	reservedNames := []string{".", "..", "con", "prn", "aux", "nul", "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9", "lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9"}
	lowerName := strings.ToLower(venvName)
	for _, reserved := range reservedNames {
		if lowerName == reserved {
			return fmt.Errorf("virtual environment name cannot be a reserved name: %s", reserved)
		}
	}

	// Prevent paths with directory traversal
	if strings.Contains(venvName, "..") || strings.Contains(venvName, "/") || strings.Contains(venvName, "\\") {
		return fmt.Errorf("virtual environment name cannot contain path separators or directory traversal")
	}

	return nil
}

// validateFilePath validates file paths to prevent directory traversal
func (p *PipPlugin) validateFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	if len(filePath) > MaxFilePathLength {
		return fmt.Errorf("file path too long (max %d characters)", MaxFilePathLength)
	}

	// Check for null bytes
	for _, r := range filePath {
		if r == 0 {
			return fmt.Errorf("file path contains null bytes")
		}
	}

	// Clean and validate the path
	cleanPath := filepath.Clean(filePath)

	// Prevent directory traversal attacks
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("file path contains directory traversal")
	}

	// Prevent absolute paths outside current directory for security
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute file paths are not allowed")
	}

	// Validate file extension for requirements files
	if strings.HasSuffix(strings.ToLower(cleanPath), ".txt") ||
		strings.HasSuffix(strings.ToLower(cleanPath), ".in") ||
		strings.HasSuffix(strings.ToLower(cleanPath), ".pip") {
		return nil // Valid requirements file extensions
	}

	// Allow files without extensions (like "requirements")
	if !strings.Contains(filepath.Base(cleanPath), ".") {
		return nil
	}

	return fmt.Errorf("invalid file extension for requirements file")
}
