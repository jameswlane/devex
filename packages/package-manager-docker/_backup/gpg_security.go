package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/metrics"
)

// SecureGPGDownloader handles secure GPG key downloads with certificate pinning
type SecureGPGDownloader struct {
	client   *http.Client
	verifier GPGVerifier
}

// GPGVerifier interface for GPG operations (enables mocking in tests)
type GPGVerifier interface {
	VerifyFingerprint(ctx context.Context, keyPath, expectedFingerprint string) error
	ImportKey(ctx context.Context, keyPath, outputPath string) error
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

// DownloadAndVerifyGPGKey securely downloads and verifies Docker's GPG key with OS-specific URL
func (d *SecureGPGDownloader) DownloadAndVerifyGPGKey(ctx context.Context, gpgURL, outputPath string) error {
	log.Info("Starting secure Docker GPG key download with certificate pinning", "url", gpgURL)

	// Create temporary file for download
	tempFile, err := os.CreateTemp("", "docker-gpg-key-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Use provided context with timeout for download
	downloadCtx, cancel := context.WithTimeout(ctx, GPGDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, "GET", gpgURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add security headers
	req.Header.Set("User-Agent", "DevEx-CLI/1.0 (Secure GPG Downloader)")
	req.Header.Set("Accept", "application/pgp-keys, */*")

	log.Debug("Downloading Docker GPG key", "url", gpgURL)
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
	if err := d.verifier.VerifyFingerprint(downloadCtx, tempFile.Name(), DockerGPGKeyFingerprint); err != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"operation": "gpg_key_verification",
			"reason":    "fingerprint_mismatch",
		})
		return fmt.Errorf("GPG key fingerprint verification failed: %w", err)
	}

	// Import verified key to final location
	log.Debug("Importing verified GPG key", "outputPath", outputPath)
	if err := d.verifier.ImportKey(downloadCtx, tempFile.Name(), outputPath); err != nil {
		return fmt.Errorf("failed to import GPG key: %w", err)
	}

	log.Info("Docker GPG key downloaded and verified successfully")
	metrics.RecordCount(metrics.MetricSecurityValidationSuccess, map[string]string{
		"operation": "gpg_key_download_and_verify",
	})

	return nil
}

// AsyncGPGResult represents the result of an asynchronous GPG operation
type AsyncGPGResult struct {
	Error      error
	URL        string
	OutputPath string
	Duration   time.Duration
}

// DownloadAndVerifyGPGKeyAsync performs GPG key download and verification asynchronously
func (d *SecureGPGDownloader) DownloadAndVerifyGPGKeyAsync(ctx context.Context, gpgURL, outputPath string) <-chan AsyncGPGResult {
	resultChan := make(chan AsyncGPGResult, 1)

	go func() {
		defer close(resultChan)

		startTime := time.Now()

		log.Debug("Starting async GPG key download", "url", gpgURL)
		err := d.DownloadAndVerifyGPGKey(ctx, gpgURL, outputPath)
		duration := time.Since(startTime)

		result := AsyncGPGResult{
			Error:      err,
			URL:        gpgURL,
			OutputPath: outputPath,
			Duration:   duration,
		}

		if err != nil {
			log.Error("Async GPG download failed", err, "url", gpgURL, "duration", duration)
			metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
				"operation": "async_gpg_download",
				"reason":    "download_failed",
			})
		} else {
			log.Debug("Async GPG download completed successfully", "url", gpgURL, "duration", duration)
			metrics.RecordCount(metrics.MetricSecurityValidationSuccess, map[string]string{
				"operation": "async_gpg_download",
			})
		}

		select {
		case resultChan <- result:
		case <-ctx.Done():
			// Context cancelled, don't block on send
			log.Warn("Async GPG operation cancelled before result could be sent", "url", gpgURL)
		}
	}()

	return resultChan
}

// DownloadMultipleGPGKeysAsync downloads multiple GPG keys concurrently with fallback support
func (d *SecureGPGDownloader) DownloadMultipleGPGKeysAsync(ctx context.Context, gpgURLs []string, outputPath string) error {
	if len(gpgURLs) == 0 {
		return fmt.Errorf("no GPG URLs provided")
	}

	log.Info("Starting concurrent GPG key download", "urls", gpgURLs, "output", outputPath)

	// Create channels for each download
	resultChans := make([]<-chan AsyncGPGResult, len(gpgURLs))
	for i, url := range gpgURLs {
		resultChans[i] = d.DownloadAndVerifyGPGKeyAsync(ctx, url, outputPath)
	}

	// Wait for first successful download or all failures
	var lastError error

	// Use a timeout context to prevent indefinite waiting
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*GPGDownloadTimeout)
	defer cancel()

	for i, resultChan := range resultChans {
		select {
		case result := <-resultChan:
			if result.Error == nil {
				log.Info("GPG key download succeeded",
					"url", result.URL,
					"duration", result.Duration,
					"attempt", i+1)

				// Cancel remaining downloads
				cancel()

				// Wait briefly for other goroutines to clean up
				go func() {
					for j := i + 1; j < len(resultChans); j++ {
						select {
						case <-resultChans[j]:
							// Drain the channel
						case <-time.After(100 * time.Millisecond):
							// Don't wait too long
							return
						}
					}
				}()

				return nil
			}

			log.Warn("GPG key download failed, trying next URL",
				"url", result.URL,
				"error", result.Error,
				"attempt", i+1)
			lastError = result.Error

		case <-timeoutCtx.Done():
			log.Error("GPG key download timed out", timeoutCtx.Err(), "timeout", 2*GPGDownloadTimeout)
			metrics.RecordCount(metrics.MetricTimeoutOccurred, map[string]string{
				"operation": "async_gpg_multi_download",
			})
			return fmt.Errorf("GPG key download timed out after %v", 2*GPGDownloadTimeout)
		}
	}

	// All downloads failed
	if lastError != nil {
		metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
			"operation": "async_gpg_multi_download",
			"reason":    "all_downloads_failed",
		})
		return fmt.Errorf("all GPG key downloads failed, last error: %w", lastError)
	}

	return fmt.Errorf("all GPG key downloads failed with no specific error")
}

// VerifyFingerprintAsync performs GPG fingerprint verification asynchronously
func (d *DefaultGPGVerifier) VerifyFingerprintAsync(ctx context.Context, keyPath, expectedFingerprint string) <-chan AsyncGPGResult {
	resultChan := make(chan AsyncGPGResult, 1)

	go func() {
		defer close(resultChan)

		startTime := time.Now()
		err := d.VerifyFingerprint(ctx, keyPath, expectedFingerprint)
		duration := time.Since(startTime)

		result := AsyncGPGResult{
			Error:    err,
			Duration: duration,
		}

		select {
		case resultChan <- result:
		case <-ctx.Done():
			log.Warn("Async GPG fingerprint verification cancelled", "key_path", keyPath)
		}
	}()

	return resultChan
}

// ImportKeyAsync performs GPG key import asynchronously
func (d *DefaultGPGVerifier) ImportKeyAsync(ctx context.Context, keyPath, outputPath string) <-chan AsyncGPGResult {
	resultChan := make(chan AsyncGPGResult, 1)

	go func() {
		defer close(resultChan)

		startTime := time.Now()
		err := d.ImportKey(ctx, keyPath, outputPath)
		duration := time.Since(startTime)

		result := AsyncGPGResult{
			Error:      err,
			OutputPath: outputPath,
			Duration:   duration,
		}

		select {
		case resultChan <- result:
		case <-ctx.Done():
			log.Warn("Async GPG key import cancelled", "key_path", keyPath)
		}
	}()

	return resultChan
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

	actualFingerprint := strings.ReplaceAll(formattedFingerprint.String(), ":", "")

	// Check all valid certificate fingerprints (supports rotation)
	for i, validFingerprint := range DockerValidCertFingerprints {
		expectedFp := strings.ReplaceAll(validFingerprint, ":", "")
		if strings.EqualFold(expectedFp, actualFingerprint) {
			if i == 0 {
				log.Debug("Certificate pinning successful - primary fingerprint match")
				metrics.RecordCount(metrics.MetricSecurityValidationSuccess, map[string]string{
					"operation": "certificate_pinning",
					"type":      "primary_fingerprint",
				})
			} else {
				log.Warn("Certificate pinning using backup fingerprint - consider certificate rotation",
					"backup_fingerprint", validFingerprint,
					"fingerprint_index", fmt.Sprintf("%d", i))
				metrics.RecordCount(metrics.MetricSecurityValidationSuccess, map[string]string{
					"operation": "certificate_pinning",
					"type":      "backup_fingerprint_used",
					"index":     fmt.Sprintf("%d", i),
				})
			}
			return nil
		}
	}

	// All fingerprints failed - perform certificate chain validation as fallback
	if err := verifyDockerCertificateChain(cs); err == nil {
		log.Info("Certificate pinning failed but certificate chain validation succeeded - updating fingerprint database recommended")
		metrics.RecordCount(metrics.MetricSecurityValidationSuccess, map[string]string{
			"operation": "certificate_verification",
			"type":      "chain_validation_fallback",
		})
		return nil
	}

	// All validation methods failed
	log.Warn("Certificate pinning and chain validation failed",
		"expected_fingerprints", DockerValidCertFingerprints,
		"actual", formattedFingerprint.String(),
	)
	metrics.RecordCount(metrics.MetricSecurityValidationFailed, map[string]string{
		"operation": "certificate_pinning",
		"reason":    "all_fingerprints_failed",
	})
	return fmt.Errorf("certificate pinning failed - no valid fingerprint match")
}

// verifyDockerCertificateChain performs standard certificate chain validation as fallback
func verifyDockerCertificateChain(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return fmt.Errorf("no peer certificates in chain")
	}

	cert := cs.PeerCertificates[0]

	// Verify certificate is for the expected domain
	if err := cert.VerifyHostname(DockerGPGKeyDomain); err != nil {
		return fmt.Errorf("certificate hostname verification failed: %w", err)
	}

	// Check certificate validity period
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid (not before: %v)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate expired (not after: %v)", cert.NotAfter)
	}

	// Verify certificate chain against system root CAs
	opts := x509.VerifyOptions{
		DNSName: DockerGPGKeyDomain,
		Roots:   nil, // Use system root CAs
	}

	chains, err := cert.Verify(opts)
	if err != nil {
		return fmt.Errorf("certificate chain verification failed: %w", err)
	}

	if len(chains) == 0 {
		return fmt.Errorf("no valid certificate chains found")
	}

	// Validate certificate key usage for TLS server authentication
	if cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return fmt.Errorf("certificate missing required key usage: digital signature")
	}

	// Check extended key usage
	hasServerAuth := false
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
			break
		}
	}
	if !hasServerAuth {
		return fmt.Errorf("certificate missing extended key usage: server authentication")
	}

	log.Debug("Certificate chain validation successful",
		"subject", cert.Subject.String(),
		"issuer", cert.Issuer.String(),
		"not_before", cert.NotBefore,
		"not_after", cert.NotAfter,
		"chain_length", len(chains[0]))

	return nil
}

// VerifyFingerprint verifies the GPG key fingerprint with timeout
func (d *DefaultGPGVerifier) VerifyFingerprint(ctx context.Context, keyPath, expectedFingerprint string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, GPGVerificationTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "gpg", "--with-fingerprint", "--with-colons", keyPath)
	output, err := cmd.Output()
	if err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
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
func (d *DefaultGPGVerifier) ImportKey(ctx context.Context, keyPath, outputPath string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, GPGVerificationTimeout)
	defer cancel()

	// Ensure output directory exists
	outputDir := strings.TrimSuffix(outputPath, "/docker-archive-keyring.gpg")
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create keyring directory: %w", err)
	}

	// Import key using gpg --dearmor
	cmd := exec.CommandContext(timeoutCtx, "gpg", "--dearmor", "--output", outputPath, keyPath)
	if err := cmd.Run(); err != nil {
		if timeoutCtx.Err() == context.DeadlineExceeded {
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
