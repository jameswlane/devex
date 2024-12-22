package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/user"
	"strings"
)

func GetShellRCPath() (string, error) {
	// Get the correct user: use SUDO_USER if running as root, fallback to USER
	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		targetUser = os.Getenv("USER")
	}

	usr, err := user.Lookup(targetUser)
	if err != nil {
		return "", fmt.Errorf("failed to lookup user %s: %v", targetUser, err)
	}

	// Explicitly fetch the user's shell path
	shellPath, err := getUserShell(targetUser)
	if err != nil {
		return "", fmt.Errorf("failed to determine shell for user %s: %v", targetUser, err)
	}

	// Construct shell config file path
	var shellRCPath string
	switch {
	case strings.Contains(shellPath, "bash"):

		shellRCPath = fmt.Sprintf("%s/.bashrc", usr.HomeDir)
	case strings.Contains(shellPath, "zsh"):
		shellRCPath = fmt.Sprintf("%s/.zshrc", usr.HomeDir)
	default:

		return "", fmt.Errorf("unsupported shell: %s", shellPath)
	}

	return shellRCPath, nil
}

func getUserShell(username string) (string, error) {
	file, err := os.Open("/etc/passwd")
	if err != nil {
		return "", fmt.Errorf("failed to open /etc/passwd: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, username+":") {
			parts := strings.Split(line, ":")
			if len(parts) >= 7 {
				return parts[6], nil
			}
		}
	}

	return "", fmt.Errorf("shell not found for user %s", username)
}
