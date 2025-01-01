package fs

import (
	"fmt"
	"os"
)

// PopulateMockFS initializes the mock filesystem with predefined files and directories.
func PopulateMockFS(mockFS *MockFS, files map[string]string, dirs []string) error {
	for path, content := range files {
		if err := mockFS.WriteFile(path, []byte(content), os.ModePerm); err != nil {
			return fmt.Errorf("failed to write file: %v", err)
		}
	}
	for _, dir := range dirs {
		mockFS.Dirs[dir] = true
	}
	return nil
}

// MockError injects errors into the mock filesystem for specific paths.
func MockError(mockFS *MockFS, path string, err error) {
	mockFS.Errors[path] = err
}
