package docker

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/metrics"
)

// SecureGPGDownloader handles secure GPG key downloads with certificate pinning
type SecureGPGDownloader struct {
	client   *http.Client
	verifier GPGVerifier
}

// GPGVerifier interface for GPG operations (enables mocking in tests)
type GPGVerifier interface {
	VerifyFingerprint(keyPath, expectedFingerprint string) error
	ImportKey(keyPath, outputPath string) error
}

// DefaultGPGVerifier implements GPGVerifier using system gpg command
type DefaultGPGVerifier struct{}

// NewSecureGPGDownloader creates a new secure GPG downloader with certificate pinning
func NewSecureGPGDownloader() *SecureGPGDownloader {
	// Create HTTP client with certificate pinning
	client := &http.Client{
		Timeout: GPGDownloadTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:       tls.VersionTLS12, // Secure minimum TLS version
				VerifyConnection: verifyDockerCertificate,
			},
		},
	}

	return &SecureGPGDownloader{
		client:   client,
		verifier: &DefaultGPGVerifier{},
	}
}

// DownloadAndVerifyGPGKey securely downloads and verifies Docker's GPG key
func (d *SecureGPGDownloader) DownloadAndVerifyGPGKey(outputPath string) error {
	log.Info("Starting secure Docker GPG key download with certificate pinning")

	// Create temporary file for download
	tempFile, err := os.CreateTemp("", "docker-gpg-key-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Download GPG key with timeout and certificate pinning
	ctx, cancel := context.WithTimeout(context.Background(), GPGDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", DockerGPGKeyURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add security headers
	req.Header.Set("User-Agent", "DevEx-CLI/1.0 (Secure GPG Downloader)")
	req.Header.Set("Accept", "application/pgp-keys, */*")

	log.Debug("Downloading Docker GPG key", "url", DockerGPGKeyURL)
	resp, err := d.client.Do(req)
	if err != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"operation": "gpg_key_download",
			"reason":    "network_error",
		})
		return fmt.Errorf("failed to download Docker GPG key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"operation": "gpg_key_download",
			"reason":    "http_error",
			"status":    fmt.Sprintf("%d", resp.StatusCode),
		})
		return fmt.Errorf("HTTP error downloading GPG key: %d %s", resp.StatusCode, resp.Status)
	}

	// Copy response to temporary file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save GPG key: %w", err)
	}

	// Sync and close before verification
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync GPG key file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		log.Warn("Failed to close temporary file", "error", err)
	}

	// Verify GPG key fingerprint
	log.Debug("Verifying Docker GPG key fingerprint")
	if err := d.verifier.VerifyFingerprint(tempFile.Name(), DockerGPGKeyFingerprint); err != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"operation": "gpg_key_verification",
			"reason":    "fingerprint_mismatch",
		})
		return fmt.Errorf("GPG key fingerprint verification failed: %w", err)
	}

	// Import verified key to final location
	log.Debug("Importing verified GPG key", "outputPath", outputPath)
	if err := d.verifier.ImportKey(tempFile.Name(), outputPath); err != nil {
		return fmt.Errorf("failed to import GPG key: %w", err)
	}

	log.Info("Docker GPG key downloaded and verified successfully")
	metrics.RecordCount(metrics.MetricSecurityValidationSuccess, map[string]string{
		"operation": "gpg_key_download_and_verify",
	})

	return nil
}

// verifyDockerCertificate performs certificate pinning verification
func verifyDockerCertificate(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return fmt.Errorf("no peer certificates found")
	}

	cert := cs.PeerCertificates[0]

	// Verify certificate is for the expected domain
	if err := cert.VerifyHostname(DockerGPGKeyDomain); err != nil {
		return fmt.Errorf("certificate hostname verification failed: %w", err)
	}

	// Calculate certificate fingerprint (SHA-256)
	certHash := sha256.Sum256(cert.Raw)
	certFingerprint := strings.ToUpper(fmt.Sprintf("%X", certHash[:]))

	// Format as colon-separated hex
	var formattedFingerprint strings.Builder
	for i := 0; i < len(certFingerprint); i += 2 {
		if i > 0 {
			formattedFingerprint.WriteString(":")
		}
		formattedFingerprint.WriteString(certFingerprint[i : i+2])
	}

	expectedFingerprint := strings.ReplaceAll(DockerCertFingerprint, ":", "")
	actualFingerprint := strings.ReplaceAll(formattedFingerprint.String(), ":", "")

	if !strings.EqualFold(expectedFingerprint, actualFingerprint) {
		log.Warn("Certificate pinning failed",
			"expected", DockerCertFingerprint,
			"actual", formattedFingerprint.String(),
		)
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"operation": "certificate_pinning",
			"reason":    "fingerprint_mismatch",
		})
		return fmt.Errorf("certificate pinning failed - fingerprint mismatch")
	}

	log.Debug("Certificate pinning verification successful")
	return nil
}

// VerifyFingerprint verifies the GPG key fingerprint with timeout
func (d *DefaultGPGVerifier) VerifyFingerprint(keyPath, expectedFingerprint string) error {
	ctx, cancel := context.WithTimeout(context.Background(), GPGVerificationTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gpg", "--with-fingerprint", "--with-colons", keyPath)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			metrics.RecordCount(metrics.MetricTimeoutOccurred, map[string]string{
				"operation": "gpg_fingerprint_verification",
			})
			return fmt.Errorf("GPG fingerprint verification timed out")
		}
		return fmt.Errorf("failed to get GPG fingerprint: %w", err)
	}

	// Parse fingerprint from GPG output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "fpr:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 10 {
				actualFingerprint := parts[9]
				if actualFingerprint == expectedFingerprint {
					log.Debug("GPG fingerprint verification successful")
					return nil
				}
				return fmt.Errorf("GPG fingerprint mismatch - expected: %s, got: %s",
					expectedFingerprint, actualFingerprint)
			}
		}
	}

	return fmt.Errorf("could not find GPG fingerprint in output")
}

// ImportKey imports the GPG key to the system keyring with timeout
func (d *DefaultGPGVerifier) ImportKey(keyPath, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), GPGVerificationTimeout)
	defer cancel()

	// Ensure output directory exists
	outputDir := strings.TrimSuffix(outputPath, "/docker-archive-keyring.gpg")
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create keyring directory: %w", err)
	}

	// Import key using gpg --dearmor
	cmd := exec.CommandContext(ctx, "gpg", "--dearmor", "--output", outputPath, keyPath)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			metrics.RecordCount(metrics.MetricTimeoutOccurred, map[string]string{
				"operation": "gpg_key_import",
			})
			return fmt.Errorf("GPG key import timed out")
		}
		return fmt.Errorf("failed to import GPG key: %w", err)
	}

	// Set secure permissions
	if err := os.Chmod(outputPath, 0600); err != nil {
		return fmt.Errorf("failed to set GPG key permissions: %w", err)
	}

	return nil
}
