package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// DownloadGPGKey downloads and optionally processes a GPG key.
func DownloadGPGKey(url, destination string, dearmor bool) error {
	log.Info("Downloading GPG key", "url", url, "destination", destination)

	// Check if the GPG key file already exists
	exists, err := fs.FileExistsAndIsFile(destination)
	if err != nil {
		log.Error("Failed to check if GPG key file exists", err, "destination", destination)
		return fmt.Errorf("failed to check if GPG key file exists: %w", err)
	}

	if exists {
		log.Info("GPG key file already exists", "destination", destination)
		return nil
	}

	// Ensure the destination directory exists
	dir := filepath.Dir(destination)
	if err := fs.EnsureDir(dir, 0o755); err != nil {
		log.Error("Failed to create directory", err, "directory", dir)
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Create and execute the HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Error("Failed to create HTTP request", err, "url", url)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error("Failed to download GPG key", err, "url", url)
		return fmt.Errorf("failed to download GPG key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("Unexpected HTTP status code", fmt.Errorf("URL: %s, status code: %d", url, resp.StatusCode))
		return fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	// Write the response body to a temporary file
	tempFile := destination + ".tmp"
	outFile, err := fs.Create(tempFile)
	if err != nil {
		log.Error("Failed to create temp file", err, "tempFile", tempFile)
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if cerr := outFile.Close(); cerr != nil {
			log.Warn("Failed to close temp file", "tempFile", tempFile, "error", cerr)
		}
	}()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		log.Error("Failed to save GPG key", err, "tempFile", tempFile)
		return fmt.Errorf("failed to save GPG key: %w", err)
	}

	// Optionally dearmor the GPG key
	if dearmor {
		return ProcessGPGKey(tempFile, destination)
	}

	// Rename the temporary file to the final destination
	if err := fs.Rename(tempFile, destination); err != nil {
		log.Error("Failed to rename temp file", err, "tempFile", tempFile, "destination", destination)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	log.Info("GPG key downloaded successfully", "destination", destination)
	return nil
}
