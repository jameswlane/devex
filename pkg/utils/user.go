package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"

	"github.com/jameswlane/devex/pkg/log"
)

func GetShellRCPath() (string, error) {
	log.Info("Starting GetShellRCPath")

	// Get the correct user: use SUDO_USER if running as root, fallback to USER
	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		targetUser = os.Getenv("USER")
	}
	log.Info("Determined target user", "targetUser", targetUser)

	usr, err := user.Lookup(targetUser)
	if err != nil {
		log.Error("Failed to lookup user", "targetUser", targetUser, "error", err)
		return "", fmt.Errorf("failed to lookup user %s: %v", targetUser, err)
	}
	log.Info("User lookup successful", "username", usr.Username)

	// Explicitly fetch the user's shell path
	shellPath, err := getUserShell(targetUser)
	if err != nil {
		log.Error("Failed to determine shell for user", "targetUser", targetUser, "error", err)
		return "", fmt.Errorf("failed to determine shell for user %s: %v", targetUser, err)
	}
	log.Info("Determined user shell", "shellPath", shellPath)

	// Construct shell config file path
	var shellRCPath string
	switch {
	case strings.Contains(shellPath, "bash"):
		shellRCPath = fmt.Sprintf("%s/.bashrc", usr.HomeDir)
	case strings.Contains(shellPath, "zsh"):
		shellRCPath = fmt.Sprintf("%s/.zshrc", usr.HomeDir)
	default:
		log.Error("Unsupported shell", "shellPath", shellPath)
		return "", fmt.Errorf("unsupported shell: %s", shellPath)
	}
	log.Info("Constructed shell RC path", "shellRCPath", shellRCPath)

	return shellRCPath, nil
}

func getUserShell(username string) (string, error) {
	log.Info("Starting getUserShell", "username", username)

	file, err := os.Open("/etc/passwd")
	if err != nil {
		log.Error("Failed to open /etc/passwd", "error", err)
		return "", fmt.Errorf("failed to open /etc/passwd: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, username+":") {
			parts := strings.Split(line, ":")
			if len(parts) >= 7 {
				log.Info("Found shell for user", "username", username, "shell", parts[6])
				return parts[6], nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error("Error scanning /etc/passwd", "error", err)
		return "", fmt.Errorf("error scanning /etc/passwd: %v", err)
	}

	log.Error("Shell not found for user", "username", username)
	return "", fmt.Errorf("shell not found for user %s", username)
}
