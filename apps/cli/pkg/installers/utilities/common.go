package utilities

import (
	"os"
	"os/user"
	"strings"

	"github.com/jameswlane/devex/pkg/utils"
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
