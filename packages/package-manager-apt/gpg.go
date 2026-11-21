package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
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

		// Dearmor in-process so hosts without gnupg (e.g. Debian cloud images) work
		armored, err := os.ReadFile(downloadPath)
		if err != nil {
			return fmt.Errorf("failed to read temporary key file: %w", err)
		}
		keyring, err := DearmorPublicKey(armored)
		if err != nil {
			return fmt.Errorf("downloaded content from '%s' is not a valid ASCII-armored GPG public key: %w", source.KeyURL, err)
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
		_, writeErr := dearmoredFile.Write(keyring)
		if closeErr := dearmoredFile.Close(); writeErr == nil {
			writeErr = closeErr
		}
		if writeErr != nil {
			return fmt.Errorf("failed to write temporary dearmored key file: %w", writeErr)
		}
		installSource = dearmoredPath
	}

	// Install the key to its final destination (-D creates parent directories),
	// through sudo when it lives in a root-owned APT directory
	if err := sdk.ExecCommandWithContext(ctx, NeedsElevation(source.KeyPath), "install", "-D", "-m", "0644", installSource, source.KeyPath); err != nil {
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
		keyring, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read key file: %w", err)
		}
		if _, err := openpgp.ReadKeyRing(bytes.NewReader(keyring)); err != nil {
			return fmt.Errorf("key file does not appear to be a valid GPG keyring: %s: %w", keyPath, err)
		}
	}

	return nil
}

const (
	armorBeginPublicKey = "-----BEGIN PGP PUBLIC KEY BLOCK-----"
	armorEndPublicKey   = "-----END PGP PUBLIC KEY BLOCK-----"
)

// DearmorPublicKey converts ASCII-armored public keys into the binary keyring
// format that `gpg --dearmor` produces and APT's signed-by option reads. Like
// gpg, it decodes every armored block, so vendors that ship rollover keys in
// separate blocks keep all of them.
func DearmorPublicKey(armored []byte) ([]byte, error) {
	var keyring []byte
	var block []string
	inBlock := false
	for _, line := range strings.Split(string(armored), "\n") {
		marker := strings.TrimRight(line, " \t\r")
		switch {
		case marker == armorBeginPublicKey:
			if inBlock {
				return nil, fmt.Errorf("armored key block is missing its END line")
			}
			inBlock = true
			block = []string{marker}
		case marker == armorEndPublicKey && inBlock:
			inBlock = false
			block = append(block, marker)
			decoded, err := armor.Decode(strings.NewReader(strings.Join(block, "\n") + "\n"))
			if err != nil {
				return nil, err
			}
			if decoded.Type != openpgp.PublicKeyType {
				return nil, fmt.Errorf("unexpected armor type %q", decoded.Type)
			}
			body, err := io.ReadAll(decoded.Body)
			if err != nil {
				return nil, err
			}
			if len(body) == 0 {
				return nil, fmt.Errorf("armored key block is empty")
			}
			keyring = append(keyring, body...)
		case inBlock:
			block = append(block, line)
		}
	}
	if inBlock {
		return nil, fmt.Errorf("armored key block is missing its END line")
	}
	if len(keyring) == 0 {
		return nil, fmt.Errorf("no public key block found")
	}
	entities, err := openpgp.ReadKeyRing(bytes.NewReader(keyring))
	if err != nil {
		return nil, err
	}
	if len(entities) == 0 {
		return nil, fmt.Errorf("no public keys found")
	}
	return keyring, nil
}
