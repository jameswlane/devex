package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// GetUserShell returns the shell for the given username by parsing /etc/passwd.
func GetUserShell(username string) (string, error) {
	log.Info("Fetching user shell", "username", username)

	// Open /etc/passwd
	file, err := os.Open("/etc/passwd")
	if err != nil {
		log.Error("Failed to open /etc/passwd", err)
		return "", fmt.Errorf("%w: unable to access /etc/passwd", ErrFileNotFound)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Error("Failed to close /etc/passwd", err)
		}
	}(file)

	// Parse /etc/passwd to find the user's shell
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) < 7 || parts[0] != username {
			continue
		}
		shell := parts[6]
		log.Info("User shell detected", "username", username, "shell", shell)
		return shell, nil
	}

	// Handle scanning errors
	if err := scanner.Err(); err != nil {
		log.Error("Error reading /etc/passwd", err)
		return "", fmt.Errorf("%w: error reading /etc/passwd", err)
	}

	// Fallback to default shell if not found
	log.Warn("User shell not found, falling back to default", "username", username)
	return DefaultShell, nil
}
