package setup

import (
	"os"
	"path/filepath"
	"regexp"
)

// Compile regex patterns once at package initialization
var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

// isValidEmail validates email format using regex
// This provides better validation than basic string checking while remaining practical
// Returns true if email matches standard email format
func isValidEmail(email string) bool {
	// Uses pre-compiled regex for better performance
	return emailRegex.MatchString(email)
}

// detectAssetsDir detects the location of built-in assets (similar to template manager)
func (m *SetupModel) detectAssetsDir() string {
	// Try different possible locations for built-in assets
	possiblePaths := []string{
		"assets",                  // Development mode (relative to binary)
		"./assets",                // Current directory
		"/usr/share/devex/assets", // System install
		"/opt/devex/assets",       // Alternative system install
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Fallback - try to find relative to the executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		assetsPath := filepath.Join(execDir, "assets")
		if _, err := os.Stat(assetsPath); err == nil {
			return assetsPath
		}

		// Try going up directories (for development)
		for i := 0; i < 3; i++ {
			execDir = filepath.Dir(execDir)
			assetsPath := filepath.Join(execDir, "assets")
			if _, err := os.Stat(assetsPath); err == nil {
				return assetsPath
			}
		}
	}

	// Final fallback
	return "assets"
}
