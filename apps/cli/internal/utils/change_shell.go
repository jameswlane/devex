package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// ChangeUserShell changes the user's default shell to the specified shell
func ChangeUserShell(shell string) error {
	log.Info("Changing user shell", "shell", shell)

	// Get current user
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = os.Getenv("USERNAME") // Windows fallback
		if currentUser == "" {
			return fmt.Errorf("unable to determine current user")
		}
	}

	// Validate that the shell exists and is executable
	if _, err := exec.LookPath(shell); err != nil {
		return fmt.Errorf("shell %s not found or not executable: %w", shell, err)
	}

	// Check if shell is in /etc/shells
	if err := validateShellInEtcShells(shell); err != nil {
		log.Warn("Shell not found in /etc/shells, attempting to add it", "shell", shell)
		if err := addShellToEtcShells(shell); err != nil {
			return fmt.Errorf("failed to add shell to /etc/shells: %w", err)
		}
	}

	// Change shell using chsh
	log.Info("Executing chsh to change shell", "user", currentUser, "shell", shell)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "chsh", "-s", shell, currentUser)

	// Run the command
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to change shell", err, "output", string(output))
		return fmt.Errorf("failed to change shell to %s: %w, output: %s", shell, err, string(output))
	}

	log.Info("Shell changed successfully", "user", currentUser, "shell", shell)
	return nil
}

// validateShellInEtcShells checks if the shell is listed in /etc/shells
func validateShellInEtcShells(shell string) error {
	content, err := os.ReadFile("/etc/shells")
	if err != nil {
		return fmt.Errorf("failed to read /etc/shells: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == shell {
			return nil
		}
	}

	return fmt.Errorf("shell %s not found in /etc/shells", shell)
}

// addShellToEtcShells adds the shell to /etc/shells if it's not already there
func addShellToEtcShells(shell string) error {
	log.Info("Adding shell to /etc/shells", "shell", shell)

	// Use sudo to append to /etc/shells
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "sh", "-c", fmt.Sprintf("echo '%s' >> /etc/shells", shell))
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Error("Failed to add shell to /etc/shells", err, "output", string(output))
		return fmt.Errorf("failed to add shell to /etc/shells: %w, output: %s", err, string(output))
	}

	log.Info("Shell added to /etc/shells successfully", "shell", shell)
	return nil
}

// GetShellPath returns the full path to a shell binary
func GetShellPath(shellName string) (string, error) {
	// Common shell paths
	commonPaths := map[string][]string{
		"zsh":  {"/usr/bin/zsh", "/bin/zsh", "/usr/local/bin/zsh"},
		"bash": {"/bin/bash", "/usr/bin/bash"},
		"fish": {"/usr/bin/fish", "/usr/local/bin/fish"},
	}

	// Try to find shell using 'which' first
	if path, err := exec.LookPath(shellName); err == nil {
		log.Info("Shell found using which", "shell", shellName, "path", path)
		return path, nil
	}

	// Try common paths if which fails
	if paths, exists := commonPaths[shellName]; exists {
		for _, path := range paths {
			if _, err := os.Stat(path); err == nil {
				log.Info("Shell found at common path", "shell", shellName, "path", path)
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("shell %s not found", shellName)
}
