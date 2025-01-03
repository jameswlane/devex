package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
)

func CopyFile(source, destination string) error {
	log.Info("Starting CopyFile", "source", source, "destination", destination)

	if _, err := fs.Stat(source); os.IsNotExist(err) {
		log.Error("Source file does not exist", "source", source)
		return fmt.Errorf("source file does not exist: %s", source)
	}

	destDir := filepath.Dir(destination)
	if err := fs.MkdirAll(destDir, 0o755); err != nil {
		log.Error("Failed to create destination directory", "destination", destination, "error", err)
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	srcFile, err := fs.Open(source)
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

// CopyConfigFiles copies all config files from a source directory to a destination directory
func CopyConfigFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing %s: %w", path, err)
		}

		if !info.IsDir() {
			dstFile := filepath.Join(dstDir, info.Name())
			if err := CopyFile(path, dstFile); err != nil {
				return fmt.Errorf("failed to copy %s: %w", path, err)
			}
		}
		return nil
	})
}

// CopyToBin copies a file to the bin directory and makes it executable
func CopyToBin(src, binDir string) error {
	// Ensure the bin directory exists
	if _, err := fs.Stat(binDir); os.IsNotExist(err) {
		return fmt.Errorf("bin directory does not exist: %s", binDir)
	}

	// Get the destination file path
	dest := filepath.Join(binDir, filepath.Base(src))

	// Copy the file content
	if err := CopyFile(src, dest); err != nil {
		return fmt.Errorf("failed to copy file to bin: %w", err)
	}

	// Make the file executable
	if err := fs.Chmod(dest, 0o755); err != nil {
		return fmt.Errorf("failed to make file executable: %w", err)
	}

	return nil
}
