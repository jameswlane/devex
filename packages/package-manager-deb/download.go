package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// downloadDebFile downloads a .deb file from a URL
func (d *DebInstaller) downloadDebFile(ctx context.Context, url string) (downloaded string, downloadErr error) {
	// Validate URL
	if err := d.validateURL(url); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "*.deb")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		_ = tmpFile.Close()
		if downloadErr != nil {
			if cleanupErr := os.Remove(tmpFile.Name()); cleanupErr != nil {
				downloadErr = errors.Join(downloadErr, fmt.Errorf("failed to remove partial download %s: %w", tmpFile.Name(), cleanupErr))
			}
		}
	}()

	d.logger.Debug(fmt.Sprintf("Downloading to temporary file: %s", tmpFile.Name()))

	// Download the file with proper context propagation
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("refusing redirect downgrade from https to %s", req.URL.Scheme)
			}
			_, err := d.validateURLOrigin(req.URL.String())
			return err
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Verify that a redirecting URL resolved to a .deb path or an explicit Debian package content type.
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	finalPath := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalPath = strings.ToLower(resp.Request.URL.Path)
	}
	if !strings.HasSuffix(finalPath, ".deb") && !d.isExplicitDebContentType(contentType) {
		return "", fmt.Errorf("resolved download URL does not point to a .deb package")
	}
	if contentType != "" && !d.isValidDebContentType(contentType) {
		d.logger.Warning("Unexpected content type: %s", contentType)
	}

	// Copy the response body to the file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	d.logger.Debug(fmt.Sprintf("Successfully downloaded package to: %s", tmpFile.Name()))
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
		d.logger.Debug(fmt.Sprintf("Cleaned up temporary file: %s", filename))
	}
}

// isExplicitDebContentType checks if the content type explicitly identifies a .deb package.
func (d *DebInstaller) isExplicitDebContentType(contentType string) bool {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "application/vnd.debian.binary-package",
		"application/x-deb",
		"application/x-debian-package":
		return true
	default:
		return false
	}
}

// isValidDebContentType checks if the content type indicates a .deb file
func (d *DebInstaller) isValidDebContentType(contentType string) bool {
	if d.isExplicitDebContentType(contentType) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(contentType), "application/octet-stream")
}

// isLocalFile checks if the given path refers to a local file
func (d *DebInstaller) isLocalFile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
