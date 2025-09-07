package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

	// Create keyring directory if it doesn't exist
	keyringDir := filepath.Dir(source.KeyPath)
	if err := os.MkdirAll(keyringDir, 0755); err != nil {
		return fmt.Errorf("failed to create keyring directory '%s' for GPG key storage: %w", keyringDir, err)
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

	// Process the key based on dearmor requirement
	if source.RequireDearmor {
		a.getLogger().Debug("Converting ASCII-armored key to binary format")

		// Create secure temporary file for GPG key
		tempFile, err := os.CreateTemp("", "repo_key_*.asc")
		if err != nil {
			return fmt.Errorf("failed to create temporary key file: %w", err)
		}
		tempFileName := tempFile.Name()
		defer func() {
			if err := os.Remove(tempFileName); err != nil {
				a.getLogger().Warning("Failed to remove temporary file: %v", err)
			}
		}()

		// Stream key data directly to temporary file
		bytesWritten, err := io.Copy(tempFile, resp.Body)
		if err != nil {
			if closeErr := tempFile.Close(); closeErr != nil {
				a.getLogger().Warning("Failed to close temporary file after copy error: %v", closeErr)
			}
			return fmt.Errorf("failed to stream GPG key to temporary file: %w", err)
		}

		// Validate that we actually received some data
		if bytesWritten == 0 {
			if closeErr := tempFile.Close(); closeErr != nil {
				a.getLogger().Warning("Failed to close temporary file after empty response: %v", closeErr)
			}
			return fmt.Errorf("received empty response for GPG key from '%s'", source.KeyURL)
		}

		// For dearmor operations, validate the content looks like an ASCII-armored key
		// Read back the content to validate format
		tempFileContent, err := os.ReadFile(tempFileName)
		if err != nil {
			return fmt.Errorf("failed to read temporary key file for validation: %w", err)
		}

		contentStr := string(tempFileContent)
		if !strings.Contains(contentStr, "BEGIN PGP PUBLIC KEY BLOCK") || !strings.Contains(contentStr, "END PGP PUBLIC KEY BLOCK") {
			return fmt.Errorf("downloaded content is not a valid ASCII-armored GPG key from '%s'", source.KeyURL)
		}

		// Close the file before passing to gpg command
		if err := tempFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary key file: %w", err)
		}

		// Validate destination path before GPG operation
		if err := a.validateFilePath(source.KeyPath); err != nil {
			return fmt.Errorf("invalid GPG key destination path: %w", err)
		}

		// Use gpg to dearmor the key
		if err := sdk.ExecCommandWithContext(ctx, false, "gpg", "--dearmor", "-o", source.KeyPath, tempFileName); err != nil {
			return fmt.Errorf("failed to dearmor GPG key from '%s' to '%s': %w", source.KeyURL, source.KeyPath, err)
		}
	} else {
		// Validate destination path before writing
		if err := a.validateFilePath(source.KeyPath); err != nil {
			return fmt.Errorf("invalid GPG key destination path: %w", err)
		}

		// Create the destination file
		keyFile, err := os.Create(source.KeyPath)
		if err != nil {
			return fmt.Errorf("failed to create GPG key file '%s': %w", source.KeyPath, err)
		}
		defer func() {
			if closeErr := keyFile.Close(); closeErr != nil {
				a.getLogger().Warning("Failed to close key file: %v", closeErr)
			}
		}()

		// Stream key data directly to destination file
		bytesWritten, err := io.Copy(keyFile, resp.Body)
		if err != nil {
			return fmt.Errorf("failed to stream GPG key to '%s': %w", source.KeyPath, err)
		}

		// Validate that we actually received some data
		if bytesWritten == 0 {
			return fmt.Errorf("received empty response for GPG key from '%s'", source.KeyURL)
		}

		// Set proper permissions
		if err := keyFile.Chmod(0644); err != nil {
			return fmt.Errorf("failed to set permissions on GPG key file '%s': %w", source.KeyPath, err)
		}
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
