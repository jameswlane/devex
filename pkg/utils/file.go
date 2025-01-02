package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/log"
)

func CopyFile(source, destination string) error {
	log.Info("Starting CopyFile", "source", source, "destination", destination)

	if _, err := os.Stat(source); os.IsNotExist(err) {
		log.Error("Source file does not exist", "source", source)
		return fmt.Errorf("source file does not exist: %s", source)
	}

	destDir := filepath.Dir(destination)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		log.Error("Failed to create destination directory", "destination", destination, "error", err)
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	srcFile, err := os.Open(source)
	if err != nil {
		log.Error("Failed to open source file", "source", source, "error", err)
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destination)
	if err != nil {
		log.Error("Failed to create destination file", "destination", destination, "error", err)
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		log.Error("Failed to copy file", "source", source, "destination", destination, "error", err)
		return fmt.Errorf("failed to copy file: %v", err)
	}

	log.Info("Copied file successfully", "source", source, "destination", destination)
	return nil
}
