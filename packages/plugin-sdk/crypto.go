package sdk

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// GPGVerifier handles GPG signature verification for plugins
type GPGVerifier struct {
	publicKeyRing openpgp.EntityList
	keyservers    []string
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
	_, err = openpgp.CheckDetachedSignature(v.publicKeyRing, file, sigReader, nil)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
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
	gpgPath string
}

// NewSystemGPGVerifier creates a GPG verifier using system gpg command
func NewSystemGPGVerifier() (*SystemGPGVerifier, error) {
	gpgPath, err := exec.LookPath("gpg")
	if err != nil {
		return nil, fmt.Errorf("system gpg not found: %w", err)
	}

	return &SystemGPGVerifier{gpgPath: gpgPath}, nil
}

// ImportPublicKey imports a public key using system GPG
func (v *SystemGPGVerifier) ImportPublicKey(keyPath string) error {
	cmd := exec.Command(v.gpgPath, "--import", keyPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to import public key: %w (output: %s)", err, string(output))
	}
	return nil
}

// VerifyDetachedSignature verifies a detached signature using system GPG
func (v *SystemGPGVerifier) VerifyDetachedSignature(filePath, signaturePath string) error {
	cmd := exec.Command(v.gpgPath, "--verify", signaturePath, filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("signature verification failed: %w (output: %s)", err, string(output))
	}
	return nil
}

// Enhanced verifyPluginSignature implementation
func (d *Downloader) verifyPluginSignature(pluginPath, downloadURL string) error {
	if d.publicKeyPath == "" {
		if d.logger != nil {
			d.logger.Warning("Plugin signature verification skipped: no public key configured. To enable verification, set publicKeyPath in DownloaderConfig")
		}
		return nil
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

	// Import public key if it's a file path
	if !strings.HasPrefix(d.publicKeyPath, "http") && FileExists(d.publicKeyPath) {
		if err := verifier.ImportPublicKey(d.publicKeyPath); err != nil {
			return fmt.Errorf("failed to import public key: %w", err)
		}
	}

	// Download signature
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(signatureURL)
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
	if err := verifier.VerifyDetachedSignature(pluginPath, tempSig.Name()); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	if d.logger != nil {
		d.logger.Success("Plugin signature verification successful (system GPG)")
	}
	return nil
}
