package appimage

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// Mock function to simulate file download
func mockDownloadFile(url, dest string) error {
	// Create a mock tarball in the destination path
	tarballContent := []byte("Mock tarball content")
	return ioutil.WriteFile(dest, tarballContent, 0644)
}

// Mock function to simulate extracting tarball
func mockExtractTarball(tarballPath, destDir string) error {
	// Create a mock binary file in the destination directory
	binaryPath := filepath.Join(destDir, "jetbrains-toolbox")
	return ioutil.WriteFile(binaryPath, []byte("Mock binary content"), 0755)
}

func TestInstallAppImage(t *testing.T) {
	// Replace the real functions with mock functions
	downloadFileFunc = mockDownloadFile
	extractTarballFunc = mockExtractTarball

	// Create a temporary directory to simulate the install directory
	installDir, err := ioutil.TempDir("", "appimage_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(installDir)

	// Run the InstallAppImage function
	err = Install("https://mockurl.com/jetbrains-toolbox.tar.gz", installDir, "jetbrains-toolbox")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Check if the binary was moved to the correct location and has executable permissions
	binaryPath := filepath.Join(installDir, "jetbrains-toolbox")
	info, err := os.Stat(binaryPath)
	if os.IsNotExist(err) {
		t.Errorf("Expected binary to be installed, but it does not exist")
	}
	if info.Mode().Perm() != 0755 {
		t.Errorf("Expected permissions 0755, but got: %v", info.Mode().Perm())
	}
}
