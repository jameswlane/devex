package apt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

// DownloadGPGKey downloads a GPG key from a URL and saves it to a temporary file
func DownloadGPGKey(url string, destination string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download GPG key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	outFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save GPG key: %v", err)
	}

	return nil
}

// AddGPGKeyToKeyring adds the downloaded GPG key to the system keyring
func AddGPGKeyToKeyring(filePath string) error {
	cmd := exec.Command("apt-key", "add", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add GPG key to keyring: %v - %s", err, string(output))
	}
	return nil
}
