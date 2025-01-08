package utils

import (
	"os"
	"path/filepath"
)

func GetHomeDir() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return filepath.Join("/home", sudoUser), nil
	}
	return os.UserHomeDir()
}
