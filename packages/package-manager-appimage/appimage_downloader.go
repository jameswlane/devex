package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// installAppImage downloads and installs an AppImage
func (p *AppimagePlugin) installAppImage(ctx context.Context, downloadURL, binaryName, installLocation string, metadata ...sdk.ArtifactVerification) error {
	homeDir := os.Getenv("HOME")
	var installDir string

	// Determine installation directory
	switch installLocation {
	case "gui":
		installDir = filepath.Join(homeDir, "Applications")
	case "cli":
		installDir = filepath.Join(homeDir, ".local", "bin")
	default:
		installDir = filepath.Join(homeDir, "Applications")
	}

	// Create installation directory if it doesn't exist
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("failed to create installation directory: %w", err)
	}

	binaryPath := filepath.Join(installDir, binaryName)
	temporary, err := os.CreateTemp(installDir, ".devex-appimage-*")
	if err != nil {
		return err
	}
	_ = temporary.Close()
	defer func() { _ = os.Remove(temporary.Name()) }()

	p.logger.Printf("Downloading AppImage to: %s\n", binaryPath)

	verification := sdk.ArtifactVerification{}
	if len(metadata) > 0 {
		verification = metadata[0]
	}
	if verification.CachedPath != "" {
		if err := sdk.CopyCachedArtifact(ctx, verification.CachedPath, temporary.Name()); err != nil {
			return err
		}
	} else if err := p.downloadFile(ctx, downloadURL, temporary.Name()); err != nil {
		return fmt.Errorf("failed to download AppImage: %w", err)
	}
	verified, err := sdk.VerifyArtifact(ctx, temporary.Name(), verification)
	if err != nil {
		return fmt.Errorf("download integrity verification failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Download verification: checksum=%t signature=%t (false means skipped)\n", verified.ChecksumVerified, verified.SignatureVerified)
	if err := os.Rename(temporary.Name(), binaryPath); err != nil {
		return err
	}

	// Set executable permissions
	if err := os.Chmod(binaryPath, 0o755); err != nil {
		return fmt.Errorf("failed to set permissions on AppImage: %w", err)
	}

	// Create desktop entry for GUI apps
	if installLocation == "gui" {
		if err := p.createDesktopEntry(binaryName, binaryPath); err != nil {
			p.logger.Warning("Failed to create desktop entry: %v", err)
			// Non-fatal error
		}
	}

	return nil
}

// downloadFile downloads a file from URL to local path
func (p *AppimagePlugin) downloadFile(ctx context.Context, url, filepath string) error {
	if err := p.validateURL(url); err != nil {
		return err
	}
	client := p.urlClient(5 * time.Minute)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// urlClient applies the same URL policy to probes, downloads, and every redirect.
func (p *AppimagePlugin) urlClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return p.validateURL(req.URL.String())
		},
	}
}

// validateURLAccessibility checks reachability, not artifact content or integrity.
func (p *AppimagePlugin) validateURLAccessibility(ctx context.Context, downloadURL string) error {
	if err := p.validateURL(downloadURL); err != nil {
		return err
	}
	client := p.urlClient(30 * time.Second)
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for URL validation: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("URL is not accessible: %w (hint: check internet connection and URL validity)", err)
	}
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented {
		_ = resp.Body.Close()
		// Probe one byte at the final URL; a server may ignore Range and return
		// 200, so never read the body of this accessibility-only request.
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, resp.Request.URL.String(), nil)
		if err != nil {
			return fmt.Errorf("failed to create fallback request: %w", err)
		}
		req.Header.Set("Range", "bytes=0-0")
		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("URL is not accessible: %w", err)
		}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("URL returned status %d: %s (hint: check if the download link is valid)", resp.StatusCode, resp.Status)
	}
	return nil
}
