package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// GetShellRCPath determines the RC file path for the current user's shell.
func GetShellRCPath(shellPath, homeDir string) (string, error) {
	log.Info("Determining shell RC path", "shellPath", shellPath, "homeDir", homeDir)

	// Fallback for missing home directory
	if homeDir == "" {
		homeDir = os.Getenv("HOME")
		if homeDir == "" {
			// log.Error("Failed to determine home directory")
			return "", fmt.Errorf("%w: home directory not set", ErrFileNotFound)
		}
	}

	// Map of supported shells and their RC files
	shellRCFiles := map[string]string{
		"bash": filepath.Join(homeDir, BashRC),
		"zsh":  filepath.Join(homeDir, ZshRC),
		"fish": filepath.Join(homeDir, ".config/fish/config.fish"),
	}

	for shell, rcPath := range shellRCFiles {
		if filepath.Base(shellPath) == shell {
			log.Info("Detected shell RC path", "shell", shell, "rcPath", rcPath)
			return rcPath, nil
		}
	}

	// Unsupported shell
	log.Error("Unsupported shell detected", fmt.Errorf("shell path: %s", shellPath))
	return "", fmt.Errorf("%w: unsupported shell %s", ErrUnsupportedShell, shellPath)
}
