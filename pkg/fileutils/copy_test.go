package fileutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	t.Parallel()
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "copy_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary source file
	srcFile := filepath.Join(tempDir, "source.txt")
	err = os.WriteFile(srcFile, []byte("Hello, World!"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	// Set the destination file
	dstFile := filepath.Join(tempDir, "destination.txt")

	// Test CopyFile
	err = CopyFile(srcFile, dstFile)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Check if destination file was copied correctly
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	if string(dstContent) != "Hello, World!" {
		t.Errorf("Expected file content to be 'Hello, World!', but got: %s", string(dstContent))
	}
}

func TestCopyConfigFiles(t *testing.T) {
	t.Parallel()
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "copy_config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary source directory and files
	srcDir := filepath.Join(tempDir, "src")
	err = os.Mkdir(srcDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create srcDir: %v", err)
	}

	err = os.WriteFile(filepath.Join(srcDir, "config1.txt"), []byte("Config 1"), 0o644)
	if err != nil {
		t.Fatalf("failed to write config1.txt: %v", err)
	}

	err = os.WriteFile(filepath.Join(srcDir, "config2.txt"), []byte("Config 2"), 0o644)
	if err != nil {
		t.Fatalf("failed to write config2.txt: %v", err)
	}

	// Set the destination directory
	dstDir := filepath.Join(tempDir, "dst")
	err = os.Mkdir(dstDir, 0o755)
	if err != nil {
		t.Fatalf("failed to create dstDir: %v", err)
	}

	// Test CopyConfigFiles
	err = CopyConfigFiles(srcDir, dstDir)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Verify that both config files were copied
	for _, file := range []string{"config1.txt", "config2.txt"} {
		dstFile := filepath.Join(dstDir, file)
		_, err := os.Stat(dstFile)
		if os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist, but it does not", dstFile)
		}
	}
}
