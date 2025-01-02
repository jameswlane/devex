package apt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/log"
)

// DownloadGPGKey downloads and optionally processes a GPG key.
func DownloadGPGKey(url, destination string, dearmor bool) error {
	log.Info("Downloading GPG key", "url", url, "destination", destination)

	exists, err := fs.FileExistsAndIsFile(destination)
	if err != nil {
		return fmt.Errorf("failed to check if GPG key file exists: %v", err)
	}

	if exists {
		log.Info("GPG key file already exists", "destination", destination)
		return nil
	}

	dir := filepath.Dir(destination)
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Create a context
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Get the data
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download GPG key: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	tempFile := destination + ".tmp"
	outFile, err := fs.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save GPG key: %v", err)
	}

	if dearmor {
		return ProcessGPGKey(tempFile, destination)
	}

	return os.Rename(tempFile, destination)
}
