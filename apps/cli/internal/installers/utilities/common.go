package utilities

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// GetCurrentUser returns the current username using multiple fallback methods
func GetCurrentUser() string {
	// Method 1: Try USER environment variable
	if username := os.Getenv("USER"); username != "" {
		return username
	}

	// Method 2: Try LOGNAME environment variable
	if username := os.Getenv("LOGNAME"); username != "" {
		return username
	}

	// Method 3: Use os/user package
	if currentUser, err := user.Current(); err == nil && currentUser.Username != "" {
		return currentUser.Username
	}

	// Method 4: Try whoami command as fallback
	if output, err := utils.CommandExec.RunShellCommand("whoami"); err == nil {
		username := strings.TrimSpace(output)
		if username != "" {
			return username
		}
	}

	return ""
}

// ValidatePath validates a file path to prevent directory traversal attacks
func ValidatePath(path, baseDir string) error {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)
	cleanBase := filepath.Clean(baseDir)

	// Ensure the path is absolute or convert relative paths
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(cleanBase, cleanPath)
	}

	// Check if the cleaned path is within the base directory
	relPath, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Ensure the relative path doesn't start with .. (directory traversal)
	if strings.HasPrefix(relPath, "..") || strings.Contains(relPath, "/../") {
		return fmt.Errorf("path traversal detected: %s", path)
	}

	return nil
}
