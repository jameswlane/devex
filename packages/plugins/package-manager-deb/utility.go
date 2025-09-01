package deb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/pkg/utils"
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
	log.Info("Fetching latest release", "owner", owner, "repo", repo, "baseURL", baseURL)

	if client == nil {
		client = http.DefaultClient
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", baseURL, owner, repo)
	ctx, cancel := context.WithTimeout(context.Background(), utils.DefaultHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Error("Failed to create HTTP request", err, "url", url)
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Error("Failed to fetch release data", err, "url", url)
		return "", fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error("Unexpected status code", err, "url", url, "status", resp.StatusCode)
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Error("Failed to decode release data", err, "url", url)
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Find the .deb file in the assets
	for _, asset := range release.Assets {
		if filepath.Ext(asset.Name) == ".deb" {
			log.Info("Found .deb asset", "name", asset.Name, "url", asset.URL)
			return asset.URL, nil
		}
	}

	log.Error("No .deb file found in the latest release", fmt.Errorf("URL: %s", url))
	return "", fmt.Errorf("no .deb file found in the latest release")
}

// DownloadDeb downloads a .deb file from the given URL and saves it to the destination path.
func DownloadDeb(url, destination string) error {
	log.Info("Downloading .deb file", "url", url, "destination", destination)

	// Use utils.DownloadFile for consistent download logic
	if err := utils.DownloadFile(url, destination); err != nil {
		log.Error("Failed to download .deb file", err, "url", url, "destination", destination)
		return fmt.Errorf("failed to download .deb file '%s': %w", destination, err)
	}

	log.Info("Downloaded .deb file successfully", "destination", destination)
	return nil
}
