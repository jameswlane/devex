package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// getSelectedShell returns the shell name corresponding to the selected index
func (m *SetupModel) getSelectedShell() string {
	if m.selectedShell >= 0 && m.selectedShell < len(m.shells) {
		return m.shells[m.selectedShell]
	}
	return "zsh" // Default fallback
}

// copyFile copies a file from source to destination
func (m *SetupModel) copyFile(src, dst string) error {
	// Validate source exists
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("source file not accessible: %w", err)
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer func() { _ = sourceFile.Close() }()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}
