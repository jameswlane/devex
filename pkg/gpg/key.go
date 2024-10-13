package gpg

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

// execCommand is a variable to allow mocking for tests
var execCommand = exec.Command

// DownloadGPGKey downloads a GPG key from a URL and saves it to a temporary file
func DownloadGPGKey(url, destination string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download GPG key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Create the destination file
	outFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer outFile.Close()

	// Copy the content to the file
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save GPG key: %v", err)
	}

	return nil
}

// AddGPGKeyToKeyring adds the downloaded GPG key to the system keyring
func AddGPGKeyToKeyring(filePath string) error {
	cmd := execCommand("apt-key", "add", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add GPG key to keyring: %v - %s", err, string(output))
	}
	return nil
}
