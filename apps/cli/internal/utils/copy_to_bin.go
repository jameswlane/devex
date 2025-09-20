package utils

import (
	"fmt"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// CopyToBin copies a file to the system's binary directory.
func CopyToBin(source string) error {
	log.Info("Starting copy to binary directory", "source", source)

	// Determine binary directory
	binDir := "/usr/local/bin"

	// Ensure binary directory exists and is writable
	if err := fs.EnsureDir(binDir, 0o755); err != nil {
		log.Error("Failed to ensure binary directory exists", err, "binDir", binDir)
		return fmt.Errorf("failed to ensure binary directory '%s': %w", binDir, err)
	}

	// Determine the destination path
	dest := filepath.Join(binDir, filepath.Base(source))

	// Check if the file already exists
	if exists, err := fs.FileExistsAndIsFile(dest); err != nil {
		log.Error("Failed to check if destination file exists", err, "destination", dest)
		return fmt.Errorf("failed to check destination file '%s': %w", dest, err)
	} else if exists {
		log.Warn("File already exists in binary directory, skipping copy", "destination", dest)
		return nil
	}

	// Copy the file
	if err := CopyFile(source, dest); err != nil {
		log.Error("Failed to copy file to binary directory", err, "source", source, "destination", dest)
		return fmt.Errorf("failed to copy file to binary directory '%s': %w", dest, err)
	}

	log.Info("File successfully copied to binary directory", "source", source, "destination", dest)
	return nil
}
