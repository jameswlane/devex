package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GitHubRegistry represents a GitHub-based plugin registry
type GitHubRegistry struct {
	client      *http.Client
	registryURL string
	cacheDir    string
	owner       string
	repo        string
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	Assets      []GitHubAsset `json:"assets"`
	PublishedAt time.Time     `json:"published_at"`
}

// GitHubAsset represents a GitHub release asset
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

// NewGitHubRegistry creates a new GitHub-based registry
func NewGitHubRegistry(registryURL, owner, repo, cacheDir string) *GitHubRegistry {
	return &GitHubRegistry{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		registryURL: registryURL,
		cacheDir:    cacheDir,
		owner:       owner,
		repo:        repo,
	}
}

// GetRegistry fetches the plugin registry
func (r *GitHubRegistry) GetRegistry(ctx context.Context) (*PluginRegistry, error) {
	// Try to load from cache first
	cachedRegistry := r.loadCachedRegistry()
	if cachedRegistry != nil && time.Since(cachedRegistry.LastUpdated) < 1*time.Hour {
		return cachedRegistry, nil
	}

	// Fetch fresh registry
	var registry *PluginRegistry
	var err error

	if r.registryURL != "" {
		// Use hosted registry (Vercel)
		registry, err = r.fetchHostedRegistry(ctx)
	} else {
		// Fallback to GitHub releases
		registry, err = r.buildRegistryFromReleases(ctx)
	}

	if err != nil {
		// Return cached registry if available
		if cachedRegistry != nil {
			fmt.Printf("Warning: using cached registry (failed to fetch: %v)\n", err)
			return cachedRegistry, nil
		}
		return nil, err
	}

	// Cache the registry
	r.cacheRegistry(registry)
	return registry, nil
}

// fetchHostedRegistry fetches registry from Vercel-hosted API
func (r *GitHubRegistry) fetchHostedRegistry(ctx context.Context) (*PluginRegistry, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.registryURL+"/api/v1/registry.json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var registry PluginRegistry
	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	return &registry, nil
}

// buildRegistryFromReleases builds registry by parsing GitHub releases
func (r *GitHubRegistry) buildRegistryFromReleases(ctx context.Context) (*PluginRegistry, error) {
	// Get latest release
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", r.owner, r.repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: HTTP %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub release: %w", err)
	}

	// Build registry from release assets
	registry := &PluginRegistry{
		BaseURL:     fmt.Sprintf("https://github.com/%s/%s/releases/download", r.owner, r.repo),
		LastUpdated: time.Now(),
		Plugins:     make(map[string]PluginMetadata),
	}

	// Group assets by plugin
	pluginAssets := r.groupPluginAssets(release.Assets)

	for pluginName, assets := range pluginAssets {
		metadata := PluginMetadata{
			Name:        pluginName,
			Version:     strings.TrimPrefix(release.TagName, "v"),
			Description: fmt.Sprintf("DevEx plugin: %s", pluginName),
			Platforms:   make(map[string]PlatformBinary),
		}

		// Process each platform asset
		for _, asset := range assets {
			platform := r.extractPlatformFromAsset(asset.Name)
			if platform != "" {
				metadata.Platforms[platform] = PlatformBinary{
					URL:  asset.BrowserDownloadURL,
					Size: asset.Size,
					// Note: Checksum would need to be calculated or stored separately
				}
			}
		}

		if len(metadata.Platforms) > 0 {
			registry.Plugins[pluginName] = metadata
		}
	}

	return registry, nil
}

// groupPluginAssets groups GitHub release assets by plugin name
func (r *GitHubRegistry) groupPluginAssets(assets []GitHubAsset) map[string][]GitHubAsset {
	groups := make(map[string][]GitHubAsset)

	for _, asset := range assets {
		if !strings.HasPrefix(asset.Name, "devex-plugin-") {
			continue
		}

		pluginName := r.extractPluginNameFromAsset(asset.Name)
		if pluginName != "" {
			groups[pluginName] = append(groups[pluginName], asset)
		}
	}

	return groups
}

// extractPluginNameFromAsset extracts plugin name from asset filename
func (r *GitHubRegistry) extractPluginNameFromAsset(assetName string) string {
	// Parse: devex-plugin-package-manager-apt_v1.0.0_linux_amd64.tar.gz
	// Remove prefix and suffix to get plugin name
	name := strings.TrimPrefix(assetName, "devex-plugin-")

	// Find the version separator
	if idx := strings.Index(name, "_v"); idx != -1 {
		name = name[:idx]
	}

	return name
}

// extractPlatformFromAsset extracts platform string from asset filename
func (r *GitHubRegistry) extractPlatformFromAsset(assetName string) string {
	// Parse: devex-plugin-package-manager-apt_v1.0.0_linux_amd64.tar.gz
	parts := strings.Split(assetName, "_")
	if len(parts) >= 3 {
		// Get OS and arch parts
		os := parts[len(parts)-2]
		arch := strings.Split(parts[len(parts)-1], ".")[0] // Remove file extension
		return fmt.Sprintf("%s-%s", os, arch)
	}
	return ""
}

// cacheRegistry saves registry to cache
func (r *GitHubRegistry) cacheRegistry(registry *PluginRegistry) {
	if err := os.MkdirAll(r.cacheDir, 0755); err != nil {
		return // Ignore cache errors
	}

	cachePath := filepath.Join(r.cacheDir, "github-registry.json")
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(cachePath, data, 0644) // Ignore cache write errors
}

// loadCachedRegistry loads registry from cache
func (r *GitHubRegistry) loadCachedRegistry() *PluginRegistry {
	cachePath := filepath.Join(r.cacheDir, "github-registry.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}

	var registry PluginRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil
	}

	return &registry
}

// DownloadPlugin downloads a plugin from GitHub releases
func (r *GitHubRegistry) DownloadPlugin(ctx context.Context, pluginName, targetDir string) error {
	registry, err := r.GetRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to get registry: %w", err)
	}

	metadata, exists := registry.Plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s not found in registry", pluginName)
	}

	// Get platform-specific binary
	platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	binary, exists := metadata.Platforms[platformKey]
	if !exists {
		return fmt.Errorf("plugin %s not available for platform %s", pluginName, platformKey)
	}

	// Download the plugin
	pluginPath := filepath.Join(targetDir, fmt.Sprintf("devex-plugin-%s", pluginName))

	// Add .exe extension on Windows
	if runtime.GOOS == "windows" {
		pluginPath += ".exe"
	}

	fmt.Printf("Downloading %s v%s for %s...\n", pluginName, metadata.Version, platformKey)

	req, err := http.NewRequestWithContext(ctx, "GET", binary.URL, nil)
	if err != nil {
		return err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Handle compressed archives
	if strings.HasSuffix(binary.URL, ".tar.gz") || strings.HasSuffix(binary.URL, ".zip") {
		return r.extractPlugin(resp.Body, pluginPath, binary.URL)
	}

	// Direct binary download
	file, err := os.Create(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to create plugin file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to save plugin: %w", err)
	}

	// Make executable
	if err := os.Chmod(pluginPath, 0755); err != nil {
		return fmt.Errorf("failed to make plugin executable: %w", err)
	}

	fmt.Printf("Successfully downloaded %s\n", pluginName)
	return nil
}

// extractPlugin extracts plugin from compressed archive
func (r *GitHubRegistry) extractPlugin(reader io.Reader, targetPath, sourceURL string) error {
	// For now, assume direct binary in archive with same name as target
	// In a full implementation, you'd use archive/tar or archive/zip

	tempFile, err := os.CreateTemp("", "plugin-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, reader); err != nil {
		return err
	}

	// This is a simplified extraction - you'd want proper tar.gz/zip handling
	// For now, assume the archive contains a single binary
	_ = tempFile.Close() // Close temp file

	return os.Rename(tempFile.Name(), targetPath)
}
