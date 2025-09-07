package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// installAppImage downloads and installs an AppImage
func (p *AppimagePlugin) installAppImage(downloadURL, binaryName, installLocation string) error {
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

	p.logger.Printf("Downloading AppImage to: %s\n", binaryPath)

	// Download the AppImage
	if err := p.downloadFile(downloadURL, binaryPath); err != nil {
		return fmt.Errorf("failed to download AppImage: %w", err)
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
func (p *AppimagePlugin) downloadFile(url, filepath string) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Minute, // 5 minute timeout for large AppImages
	}

	resp, err := client.Get(url)
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

// validateURLAccessibility checks if a URL is accessible
func (p *AppimagePlugin) validateURLAccessibility(downloadURL string) error {
	p.logger.Debug("Validating URL accessibility: %s", downloadURL)

	// Create a HEAD request to check if URL is accessible
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("HEAD", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for URL validation: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("URL is not accessible: %w (hint: check internet connection and URL validity)", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("URL returned status %d: %s (hint: check if the download link is valid)", resp.StatusCode, resp.Status)
	}

	p.logger.Debug("URL accessibility validated: %s (status: %d)", downloadURL, resp.StatusCode)
	return nil
}
