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
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destinationFile.Close()

	// Copy the file content
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}

// CopyConfigFiles copies all config files from a source directory to a destination directory
func CopyConfigFiles(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Construct destination file path
			dstFile := filepath.Join(dstDir, info.Name())

			// Copy the file
			err = CopyFile(path, dstFile)
			if err != nil {
				return fmt.Errorf("failed to copy %s: %v", path, err)
			}
		}
		return nil
	})
}
