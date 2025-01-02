package deb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Release represents a GitHub release
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents an asset in a GitHub release
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// GetLatestDebURL fetches the latest release and returns the URL of the .deb asset.
// It accepts a base URL and HTTP client for testing.
func GetLatestDebURL(owner, repo, baseURL string, client *http.Client) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo)
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release Release
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	// Find the .deb file in the assets
	for _, asset := range release.Assets {
		if filepath.Ext(asset.Name) == ".deb" {
			return asset.URL, nil
		}
	}

	return "", fmt.Errorf("no .deb file found in the latest release")
}

// DownloadDeb downloads a .deb file from the given URL and saves it to the destination path
func DownloadDeb(url, destination string) error {
	client := &http.Client{}
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Create the destination file
	outFile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer outFile.Close()

	// Copy the content to the file
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save .deb file: %v", err)
	}

	return nil
}
