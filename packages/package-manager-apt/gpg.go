package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

const (
	// GPGKeyDownloadTimeout defines the timeout for GPG key download operations
	GPGKeyDownloadTimeout = 30 * time.Second
)

// downloadAndInstallGPGKey downloads a GPG key and installs it to the system
// Based on the original robust implementation with plugin SDK integration
func (a *APTInstaller) downloadAndInstallGPGKey(ctx context.Context, source APTSource) error {
	a.getLogger().Printf("Downloading GPG key from %s\n", source.KeyURL)

	// Validate key path first - must fail for security issues
	if err := a.validateFilePath(source.KeyPath); err != nil {
		return fmt.Errorf("invalid GPG key destination path: %w", err)
	}

	// Validate the key URL for security
	if err := a.validateKeyURL(source.KeyURL); err != nil {
		return fmt.Errorf("invalid GPG key URL: %w", err)
	}

	// Check if the GPG key file already exists
	if _, err := os.Stat(source.KeyPath); err == nil {
		a.getLogger().Printf("GPG key already exists at %s, skipping download\n", source.KeyPath)
		return nil
	}

	// Download the key using HTTP client
	downloadCtx, cancel := context.WithTimeout(ctx, GPGKeyDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, "GET", source.KeyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: GPGKeyDownloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download GPG key: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			a.getLogger().Warning("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download GPG key from '%s': HTTP %d", source.KeyURL, resp.StatusCode)
	}

	// Download key into a user-writable temporary file before installing via sudo
	downloadFile, err := os.CreateTemp("", "repo_key_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary key file: %w", err)
	}
	downloadPath := downloadFile.Name()
	defer func() {
		if err := os.Remove(downloadPath); err != nil && !os.IsNotExist(err) {
			a.getLogger().Warning("Failed to remove temporary file: %v", err)
		}
	}()

	bytesWritten, copyErr := io.Copy(downloadFile, resp.Body)
	closeErr := downloadFile.Close()
	if copyErr != nil {
		if closeErr != nil {
			a.getLogger().Warning("Failed to close temporary file after copy error: %v", closeErr)
		}
		return fmt.Errorf("failed to stream GPG key to temporary file: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close temporary key file: %w", closeErr)
	}

	// Validate that we actually received some data
	if bytesWritten == 0 {
		return fmt.Errorf("received empty response for GPG key from '%s'", source.KeyURL)
	}

	// Process the key based on dearmor requirement
	installSource := downloadPath
	if source.RequireDearmor {
		a.getLogger().Debug("Converting ASCII-armored key to binary format")

		// For dearmor operations, validate the content looks like an ASCII-armored key
		tempFileContent, err := os.ReadFile(downloadPath)
		if err != nil {
			return fmt.Errorf("failed to read temporary key file for validation: %w", err)
		}

		contentStr := string(tempFileContent)
		if !strings.Contains(contentStr, "BEGIN PGP PUBLIC KEY BLOCK") || !strings.Contains(contentStr, "END PGP PUBLIC KEY BLOCK") {
			return fmt.Errorf("downloaded content is not a valid ASCII-armored GPG key from '%s'", source.KeyURL)
		}

		dearmoredFile, err := os.CreateTemp("", "repo_key_*.gpg")
		if err != nil {
			return fmt.Errorf("failed to create temporary dearmored key file: %w", err)
		}
		dearmoredPath := dearmoredFile.Name()
		defer func() {
			if err := os.Remove(dearmoredPath); err != nil && !os.IsNotExist(err) {
				a.getLogger().Warning("Failed to remove temporary file: %v", err)
			}
		}()
		if err := dearmoredFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary dearmored key file: %w", err)
		}

		// Use gpg to dearmor the key into the temporary output file
		if err := sdk.ExecCommandWithContext(ctx, false, "gpg", "--yes", "--dearmor", "-o", dearmoredPath, downloadPath); err != nil {
			return fmt.Errorf("failed to dearmor GPG key from '%s' to '%s': %w", source.KeyURL, source.KeyPath, err)
		}
		installSource = dearmoredPath
	}

	// Install the key to its final destination with sudo (-D creates parent directories)
	if err := sdk.ExecCommandWithContext(ctx, true, "install", "-D", "-m", "0644", installSource, source.KeyPath); err != nil {
		return fmt.Errorf("failed to install GPG key to '%s': %w", source.KeyPath, err)
	}

	// Verify the key was written correctly
	if _, err := os.Stat(source.KeyPath); err != nil {
		return fmt.Errorf("GPG key not found after installation: %w", err)
	}

	a.getLogger().Success("GPG key installed to %s", source.KeyPath)
	return nil
}

// validateGPGKeyFormat validates that a GPG key file is in the expected format
func (a *APTInstaller) validateGPGKeyFormat(keyPath string, requireDearmor bool) error {
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("GPG key file not found: %s", keyPath)
	}

	// Validate key path before file operations
	if err := a.validateFilePath(keyPath); err != nil {
		return fmt.Errorf("invalid GPG key path: %w", err)
	}

	if requireDearmor {
		// For binary keys, check if it's a valid GPG keyring
		output, err := sdk.ExecCommandOutputWithContext(context.Background(), "file", keyPath)
		if err != nil {
			return fmt.Errorf("failed to check key file format: %w", err)
		}
		if !strings.Contains(output, "GPG key") && !strings.Contains(output, "data") {
			return fmt.Errorf("key file does not appear to be a valid GPG keyring: %s", output)
		}
	}

	return nil
}
