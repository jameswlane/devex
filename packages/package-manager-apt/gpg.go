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

// downloadAndInstallGPGKey downloads a GPG key and installs it to the system
// Based on the original robust implementation with plugin SDK integration
func (a *APTInstaller) downloadAndInstallGPGKey(source APTSource) error {
	a.logger.Printf("Downloading GPG key from %s\n", source.KeyURL)

	// Check if the GPG key file already exists
	if _, err := os.Stat(source.KeyPath); err == nil {
		a.logger.Printf("GPG key already exists at %s, skipping download\n", source.KeyPath)
		return nil
	}

	// Create keyring directory if it doesn't exist
	keyringDir := filepath.Dir(source.KeyPath)
	if err := os.MkdirAll(keyringDir, 0755); err != nil {
		return fmt.Errorf("failed to create keyring directory: %w", err)
	}

	// Download the key using HTTP client
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", source.KeyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download GPG key: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			a.logger.Warning("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download GPG key: HTTP %d", resp.StatusCode)
	}

	// Read the key content
	keyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read GPG key data: %w", err)
	}

	// Process the key based on dearmor requirement
	if source.RequireDearmor {
		a.logger.Debug("Converting ASCII-armored key to binary format")

		// Write key to temporary file first
		tempFile := "/tmp/repo_key.asc"
		if err := os.WriteFile(tempFile, keyData, 0644); err != nil {
			return fmt.Errorf("failed to write temporary key file: %w", err)
		}
		defer func() {
			if err := os.Remove(tempFile); err != nil {
				a.logger.Warning("Failed to remove temporary file: %v", err)
			}
		}()

		// Use gpg to dearmor the key
		if err := sdk.ExecCommand(false, "gpg", "--dearmor", "-o", source.KeyPath, tempFile); err != nil {
			return fmt.Errorf("failed to dearmor GPG key: %w", err)
		}
	} else {
		// Write key directly to destination
		if err := os.WriteFile(source.KeyPath, keyData, 0644); err != nil {
			return fmt.Errorf("failed to write GPG key: %w", err)
		}
	}

	// Verify the key was written correctly
	if _, err := os.Stat(source.KeyPath); err != nil {
		return fmt.Errorf("GPG key not found after installation: %w", err)
	}

	a.logger.Success("GPG key installed to %s", source.KeyPath)
	return nil
}

// validateGPGKeyFormat validates that a GPG key file is in the expected format
func (a *APTInstaller) validateGPGKeyFormat(keyPath string, requireDearmor bool) error {
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("GPG key file not found: %s", keyPath)
	}

	if requireDearmor {
		// For binary keys, check if it's a valid GPG keyring
		output, err := sdk.ExecCommandOutput("file", keyPath)
		if err != nil {
			return fmt.Errorf("failed to check key file format: %w", err)
		}
		if !strings.Contains(output, "GPG key") && !strings.Contains(output, "data") {
			return fmt.Errorf("key file does not appear to be a valid GPG keyring: %s", output)
		}
	}

	return nil
}
