package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// downloadDebFile downloads a .deb file from a URL
func (d *DebInstaller) downloadDebFile(ctx context.Context, url string) (string, error) {
	// Validate URL
	if err := d.validateURL(url); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "*.deb")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() { _ = tmpFile.Close() }()

	d.logger.Debug("Downloading to temporary file: %s", tmpFile.Name())

	// Download the file with proper context propagation
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Verify content type if available
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" && !d.isValidDebContentType(contentType) {
		d.logger.Warning("Unexpected content type: %s", contentType)
	}

	// Copy the response body to the file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	d.logger.Debug("Successfully downloaded package to: %s", tmpFile.Name())
	return tmpFile.Name(), nil
}

// extractPackage extracts a .deb package to a target directory
func (d *DebInstaller) extractPackage(ctx context.Context, debFile, targetDir string) error {
	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	d.logger.Printf("Extracting %s to %s\n", debFile, targetDir)

	// Get absolute paths
	absDebFile, err := filepath.Abs(debFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Extract the package
	if err := sdk.ExecCommandWithContext(ctx, false, "dpkg-deb", "-x", absDebFile, absTargetDir); err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}

	d.logger.Success("Successfully extracted package to %s", targetDir)
	return nil
}

// cleanupTempFile safely removes temporary files
func (d *DebInstaller) cleanupTempFile(filename string) {
	if err := os.Remove(filename); err != nil {
		d.logger.Warning("Failed to remove temporary file %s: %v", filename, err)
	} else {
		d.logger.Debug("Cleaned up temporary file: %s", filename)
	}
}

// isValidDebContentType checks if the content type indicates a .deb file
func (d *DebInstaller) isValidDebContentType(contentType string) bool {
	validTypes := []string{
		"application/vnd.debian.binary-package",
		"application/x-deb",
		"application/x-debian-package",
		"application/octet-stream", // Generic binary, often used for .deb files
	}

	for _, validType := range validTypes {
		if contentType == validType {
			return true
		}
	}

	return false
}

// isLocalFile checks if the given path refers to a local file
func (d *DebInstaller) isLocalFile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
