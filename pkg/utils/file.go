package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

func CopyFile(source, destination string) error {
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return fmt.Errorf("source file does not exist: %s", source)
	}

	destDir := filepath.Dir(destination)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	srcFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %v", err)
	}

	log.Info("Copied file", "source", source, "destination", destination)
	return nil
}
