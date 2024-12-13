package fileutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyToBin(t *testing.T) {
	t.Parallel()
	// Create a temporary bin directory for testing
	tempBinDir, err := os.MkdirTemp("", "bin_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempBinDir)

	// Create a temporary source file
	srcFile, err := os.CreateTemp("", "src_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(srcFile.Name())

	// Write something to the source file
	_, err = srcFile.WriteString("This is a test file")
	if err != nil {
		t.Fatal(err)
	}
	srcFile.Close()

	// Test CopyToBin
	err = CopyToBin(srcFile.Name(), tempBinDir)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Check if the file exists in the bin directory
	copiedFilePath := filepath.Join(tempBinDir, filepath.Base(srcFile.Name()))
	if _, err := os.Stat(copiedFilePath); os.IsNotExist(err) {
		t.Errorf("Expected file to exist in bin directory, but it does not")
	}

	// Check if the file is executable
	info, err := os.Stat(copiedFilePath)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o755 {
		t.Errorf("Expected file to be executable, but permissions are: %v", info.Mode().Perm())
	}
}
