package sdk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// Common typed errors for plugin signature verification
var (
	// ErrNoPublicKey is returned when signature verification is enabled but no public key is configured.
	ErrNoPublicKey = errors.New("no public key configured for signature verification")

	// ErrSignerMismatch is returned when the signature was made by a key other than the expected signer fingerprint.
	ErrSignerMismatch = errors.New("signature signer does not match expected fingerprint")

	// ErrSignatureVerificationFailed is returned when signature verification fails.
	ErrSignatureVerificationFailed = errors.New("signature verification failed")
)

// GPGVerifier handles GPG signature verification for plugins
type GPGVerifier struct {
	publicKeyRing       openpgp.EntityList
	keyservers          []string
	expectedFingerprint string
}

// NewGPGVerifier creates a new GPG verifier
func NewGPGVerifier() *GPGVerifier {
	return &GPGVerifier{
		keyservers: []string{
			"https://keys.openpgp.org",
			"https://keyserver.ubuntu.com",
		},
	}
}

// SetExpectedFingerprint sets the expected fingerprint for the signer
func (v *GPGVerifier) SetExpectedFingerprint(fingerprint string) {
	v.expectedFingerprint = strings.ToUpper(strings.ReplaceAll(fingerprint, " ", ""))
}

// LoadPublicKey loads a public key from file or URL
func (v *GPGVerifier) LoadPublicKey(keyPath string) error {
	var keyData []byte
	var err error

	if strings.HasPrefix(keyPath, "http://") || strings.HasPrefix(keyPath, "https://") {
		// Download key from URL
		keyData, err = v.downloadKey(keyPath)
	} else {
		// Read key from file
		keyData, err = os.ReadFile(keyPath)
	}

	if err != nil {
		return fmt.Errorf("failed to load public key: %w", err)
	}

	// Try to parse as armored key first
	block, err := armor.Decode(bytes.NewReader(keyData))
	if err == nil {
		// Armored key
		entities, err := openpgp.ReadEntity(packet.NewReader(block.Body))
		if err != nil {
			return fmt.Errorf("failed to parse armored public key: %w", err)
		}
		v.publicKeyRing = append(v.publicKeyRing, entities)
	} else {
		// Try as binary key
		entities, err := openpgp.ReadEntity(packet.NewReader(bytes.NewReader(keyData)))
		if err != nil {
			return fmt.Errorf("failed to parse binary public key: %w", err)
		}
		v.publicKeyRing = append(v.publicKeyRing, entities)
	}

	return nil
}

// LoadPublicKeyFromKeyserver loads a public key from a keyserver
func (v *GPGVerifier) LoadPublicKeyFromKeyserver(keyID string) error {
	// Validate keyID before making network requests
	if keyID == "" {
		return fmt.Errorf("failed to load key  from any keyserver")
	}

	// Basic validation for key ID format (should be hex)
	if len(keyID) < 8 {
		return fmt.Errorf("failed to load key %s from any keyserver", keyID)
	}

	for _, keyserver := range v.keyservers {
		keyURL := fmt.Sprintf("%s/pks/lookup?op=get&search=0x%s", keyserver, keyID)

		if err := v.LoadPublicKey(keyURL); err == nil {
			return nil // Successfully loaded from this keyserver
		}
		// Continue to next keyserver if this one fails
	}

	return fmt.Errorf("failed to load key %s from any keyserver", keyID)
}

// VerifySignature verifies a GPG signature for a file
func (v *GPGVerifier) VerifySignature(filePath, signaturePath string) error {
	if len(v.publicKeyRing) == 0 {
		return fmt.Errorf("no public keys loaded for verification")
	}

	// Read the file to be verified
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for verification: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read the signature file
	sigData, err := os.ReadFile(signaturePath)
	if err != nil {
		return fmt.Errorf("failed to read signature file: %w", err)
	}

	// Try to parse as armored signature
	var sigReader io.Reader = bytes.NewReader(sigData)

	if block, err := armor.Decode(bytes.NewReader(sigData)); err == nil {
		sigReader = block.Body
	}

	// Verify the signature
	signer, err := openpgp.CheckDetachedSignature(v.publicKeyRing, file, sigReader, nil)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	if signer == nil {
		return fmt.Errorf("signature verification failed: no signer found")
	}

	if v.expectedFingerprint != "" {
		signerFP := strings.ToUpper(fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint))
		expectedFP := strings.ToUpper(strings.ReplaceAll(v.expectedFingerprint, " ", ""))
		if signerFP != expectedFP {
			return fmt.Errorf("%w: expected %s, got %s", ErrSignerMismatch, expectedFP, signerFP)
		}
	}

	return nil
}

// VerifySignatureFromURL downloads and verifies a signature from a URL
func (v *GPGVerifier) VerifySignatureFromURL(filePath, signatureURL string) error {
	// Download signature
	sigData, err := v.downloadSignature(signatureURL)
	if err != nil {
		return fmt.Errorf("failed to download signature: %w", err)
	}

	// Create temporary signature file
	tempSig, err := os.CreateTemp("", "plugin-sig-*.sig")
	if err != nil {
		return fmt.Errorf("failed to create temp signature file: %w", err)
	}
	defer func() { _ = os.Remove(tempSig.Name()) }()
	defer func() { _ = tempSig.Close() }()

	if _, err := tempSig.Write(sigData); err != nil {
		return fmt.Errorf("failed to write signature to temp file: %w", err)
	}

	_ = tempSig.Close() // Close before verification

	return v.VerifySignature(filePath, tempSig.Name())
}

// downloadKey downloads a public key from URL
func (v *GPGVerifier) downloadKey(keyURL string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(keyURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download key: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// downloadSignature downloads a signature from URL
func (v *GPGVerifier) downloadSignature(sigURL string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(sigURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download signature: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download signature: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// SystemGPGVerifier uses system GPG for verification (fallback)
type SystemGPGVerifier struct {
	gpgPath             string
	tempDir             string
	expectedFingerprint string
}

// NewSystemGPGVerifier creates a GPG verifier using system gpg command
func NewSystemGPGVerifier() (*SystemGPGVerifier, error) {
	gpgPath, err := exec.LookPath("gpg")
	if err != nil {
		gpgPath, err = exec.LookPath("gpgv")
		if err != nil {
			return nil, fmt.Errorf("system gpg not found: %w", err)
		}
	}

	tempDir, err := os.MkdirTemp("", "gpg-verifier-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir for gpg: %w", err)
	}

	return &SystemGPGVerifier{
		gpgPath: gpgPath,
		tempDir: tempDir,
	}, nil
}

// SetExpectedFingerprint sets the expected fingerprint for the signer
func (v *SystemGPGVerifier) SetExpectedFingerprint(fingerprint string) {
	v.expectedFingerprint = strings.ToUpper(strings.ReplaceAll(fingerprint, " ", ""))
}

// Close cleans up temporary keyring resources
func (v *SystemGPGVerifier) Close() error {
	if v.tempDir != "" {
		err := os.RemoveAll(v.tempDir)
		v.tempDir = ""
		return err
	}
	return nil
}

func (v *SystemGPGVerifier) getOrCreateTempDir() (string, error) {
	if v.tempDir == "" {
		tempDir, err := os.MkdirTemp("", "gpg-verifier-*")
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir for gpg: %w", err)
		}
		v.tempDir = tempDir
	}
	return v.tempDir, nil
}

// ImportPublicKey imports a public key using system GPG
func (v *SystemGPGVerifier) ImportPublicKey(keyPath string) error {
	return v.ImportPublicKeyWithContext(context.Background(), keyPath)
}

// ImportPublicKeyWithContext imports a public key using system GPG with context
func (v *SystemGPGVerifier) ImportPublicKeyWithContext(ctx context.Context, keyPath string) error {
	tempDir, err := v.getOrCreateTempDir()
	if err != nil {
		return err
	}

	keyring := filepath.Join(tempDir, "pubring.kbx")
	cmd := NewCommandContext(ctx, v.gpgPath, "--homedir", tempDir, "--no-default-keyring", "--keyring", keyring, "--batch", "--import", keyPath)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to import public key: %w (output: %s)", err, string(output))
	}
	return nil
}

// VerifyDetachedSignature verifies a detached signature using system GPG
func (v *SystemGPGVerifier) VerifyDetachedSignature(filePath, signaturePath string) error {
	return v.VerifyDetachedSignatureWithContext(context.Background(), filePath, signaturePath)
}

// VerifyDetachedSignatureWithContext verifies a detached signature using system GPG with context
func (v *SystemGPGVerifier) VerifyDetachedSignatureWithContext(ctx context.Context, filePath, signaturePath string) error {
	tempDir, err := v.getOrCreateTempDir()
	if err != nil {
		return err
	}

	keyring := filepath.Join(tempDir, "pubring.kbx")
	cmd := NewCommandContext(ctx, v.gpgPath, "--homedir", tempDir, "--no-default-keyring", "--keyring", keyring, "--batch", "--status-fd", "1", "--verify", signaturePath, filePath)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+tempDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signature verification failed: %w (output: %s)", err, string(output))
	}

	if v.expectedFingerprint != "" {
		signerFP := extractValidsigFingerprint(string(output))
		expectedFP := strings.ToUpper(strings.ReplaceAll(v.expectedFingerprint, " ", ""))
		signerFP = strings.ToUpper(strings.ReplaceAll(signerFP, " ", ""))
		if signerFP == "" || signerFP != expectedFP {
			return fmt.Errorf("%w: expected %s, got %s", ErrSignerMismatch, expectedFP, signerFP)
		}
	}

	return nil
}

func extractValidsigFingerprint(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[GNUPG:] VALIDSIG ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[2]
			}
		}
	}
	return ""
}

// Enhanced verifyPluginSignature implementation
func (d *Downloader) verifyPluginSignature(pluginPath, downloadURL string) error {
	if d.publicKeyPath == "" {
		if d.logger != nil {
			d.logger.Warning("Plugin signature verification failed: no public key configured")
		}
		return ErrNoPublicKey
	}

	// Generate signature URL (assuming .sig suffix)
	signatureURL := downloadURL + ".sig"

	// Try modern Go crypto first
	verifier := NewGPGVerifier()

	// Load public key
	if err := verifier.LoadPublicKey(d.publicKeyPath); err != nil {
		if d.logger != nil {
			d.logger.Warning("Failed to load public key with Go crypto: %v", err)
		}

		// Fallback to system GPG
		return d.verifyWithSystemGPG(pluginPath, signatureURL)
	}

	// Verify using Go crypto
	if err := verifier.VerifySignatureFromURL(pluginPath, signatureURL); err != nil {
		if d.logger != nil {
			d.logger.Warning("Go crypto verification failed: %v", err)
		}

		// Fallback to system GPG
		return d.verifyWithSystemGPG(pluginPath, signatureURL)
	}

	if d.logger != nil {
		d.logger.Success("Plugin signature verification successful")
	}
	return nil
}

// verifyWithSystemGPG falls back to system GPG for verification
func (d *Downloader) verifyWithSystemGPG(pluginPath, signatureURL string) error {
	verifier, err := NewSystemGPGVerifier()
	if err != nil {
		return fmt.Errorf("system GPG not available: %w", err)
	}
	defer func() { _ = verifier.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Import public key if it's a file path
	if !strings.HasPrefix(d.publicKeyPath, "http") && FileExists(d.publicKeyPath) {
		if err := verifier.ImportPublicKeyWithContext(ctx, d.publicKeyPath); err != nil {
			return fmt.Errorf("failed to import public key: %w", err)
		}
	} else if strings.HasPrefix(d.publicKeyPath, "http://") || strings.HasPrefix(d.publicKeyPath, "https://") {
		// Download key to temp file and import
		keyData, err := d.downloadKeyData(ctx, d.publicKeyPath)
		if err != nil {
			return fmt.Errorf("failed to download public key: %w", err)
		}
		tempKey, err := os.CreateTemp("", "plugin-key-*.asc")
		if err != nil {
			return fmt.Errorf("failed to create temp key file: %w", err)
		}
		defer func() { _ = os.Remove(tempKey.Name()) }()
		if _, err := tempKey.Write(keyData); err != nil {
			_ = tempKey.Close()
			return fmt.Errorf("failed to write temp key file: %w", err)
		}
		_ = tempKey.Close()

		if err := verifier.ImportPublicKeyWithContext(ctx, tempKey.Name()); err != nil {
			return fmt.Errorf("failed to import public key: %w", err)
		}
	}

	// Download signature
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signatureURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create signature request: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download signature: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("signature not available: HTTP %d", resp.StatusCode)
	}

	// Create temporary signature file
	tempSig, err := os.CreateTemp("", "plugin-sig-*.sig")
	if err != nil {
		return fmt.Errorf("failed to create temp signature file: %w", err)
	}
	defer func() { _ = os.Remove(tempSig.Name()) }()
	defer func() { _ = tempSig.Close() }()

	if _, err := io.Copy(tempSig, resp.Body); err != nil {
		return fmt.Errorf("failed to save signature: %w", err)
	}

	_ = tempSig.Close() // Close before verification

	// Verify signature
	if err := verifier.VerifyDetachedSignatureWithContext(ctx, pluginPath, tempSig.Name()); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	if d.logger != nil {
		d.logger.Success("Plugin signature verification successful (system GPG)")
	}
	return nil
}

func (d *Downloader) downloadKeyData(ctx context.Context, keyURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keyURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create key download request: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download key: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download key: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
