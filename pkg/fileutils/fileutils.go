package fileutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyFile copies a file from source to destination
func CopyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destinationFile.Close()

	// Copy the file content
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

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
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		return fmt.Errorf("bin directory does not exist: %s", binDir)
	}

	// Get the destination file path
	dest := filepath.Join(binDir, filepath.Base(src))

	// Copy the file content
	if err := CopyFile(src, dest); err != nil {
		return fmt.Errorf("failed to copy file to bin: %w", err)
	}

	// Make the file executable
	if err := os.Chmod(dest, 0o755); err != nil {
		return fmt.Errorf("failed to make file executable: %w", err)
	}

	return nil
}
