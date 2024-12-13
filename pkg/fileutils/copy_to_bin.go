package fileutils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyToBin copies a file from the source path to the bin directory and makes it executable
func CopyToBin(src, binDir string) error {
	// Ensure the bin directory exists
	if _, err := os.Stat(binDir); os.IsNotExist(err) {
		return fmt.Errorf("bin directory does not exist: %s", binDir)
	}

	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer sourceFile.Close()

	// Get the base name of the file to use in the bin directory
	fileName := filepath.Base(src)
	dest := filepath.Join(binDir, fileName)

	// Create the destination file
	destinationFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destinationFile.Close()

	// Copy the file content
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	// Make the file executable
	err = os.Chmod(dest, 0o755)
	if err != nil {
		return fmt.Errorf("failed to make file executable: %v", err)
	}

	return nil
}
