package plugin

import (
	"crypto/sha256"
	"encoding/hex"
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

// PluginRegistry represents the plugin registry configuration
type PluginRegistry struct {
	BaseURL     string                    `json:"base_url"`
	Plugins     map[string]PluginMetadata `json:"plugins"`
	LastUpdated time.Time                 `json:"last_updated"`
}

// PluginMetadata represents metadata about a plugin
type PluginMetadata struct {
	Name         string                    `json:"name"`
	Version      string                    `json:"version"`
	Description  string                    `json:"description"`
	Author       string                    `json:"author"`
	Repository   string                    `json:"repository"`
	Platforms    map[string]PlatformBinary `json:"platforms"`
	Dependencies []string                  `json:"dependencies"`
	Tags         []string                  `json:"tags"`
}

// PlatformBinary represents a binary for a specific platform
type PlatformBinary struct {
	URL      string `json:"url"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

// Downloader handles plugin downloading and management
type Downloader struct {
	registryURL string
	pluginDir   string
	cacheDir    string
	client      *http.Client
}

// NewDownloader creates a new plugin downloader
func NewDownloader(registryURL, pluginDir string) *Downloader {
	homeDir, _ := os.UserHomeDir()
	cacheDir := filepath.Join(homeDir, ".devex", "plugin-cache")

	return &Downloader{
		registryURL: registryURL,
		pluginDir:   pluginDir,
		cacheDir:    cacheDir,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// UpdateRegistry downloads and caches the latest plugin registry
func (d *Downloader) UpdateRegistry() (*PluginRegistry, error) {
	registryPath := filepath.Join(d.cacheDir, "registry.json")

	// Check if we have a cached registry less than 24 hours old
	if stat, err := os.Stat(registryPath); err == nil {
		if time.Since(stat.ModTime()) < 24*time.Hour {
			return d.loadCachedRegistry(registryPath)
		}
	}

	// Download fresh registry
	resp, err := d.client.Get(d.registryURL + "/registry.json")
	if err != nil {
		// Try to use cached version if download fails
		if registry, cacheErr := d.loadCachedRegistry(registryPath); cacheErr == nil {
			fmt.Println("Warning: Using cached plugin registry (network unavailable)")
			return registry, nil
		}
		return nil, fmt.Errorf("failed to download registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download registry: HTTP %d", resp.StatusCode)
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(d.cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Save to cache
	cacheFile, err := os.Create(registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache file: %w", err)
	}
	defer cacheFile.Close()

	// Parse and cache simultaneously
	var registry PluginRegistry
	teeReader := io.TeeReader(resp.Body, cacheFile)
	if err := json.NewDecoder(teeReader).Decode(&registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry: %w", err)
	}

	return &registry, nil
}

// loadCachedRegistry loads the registry from cache
func (d *Downloader) loadCachedRegistry(path string) (*PluginRegistry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var registry PluginRegistry
	if err := json.NewDecoder(file).Decode(&registry); err != nil {
		return nil, err
	}

	return &registry, nil
}

// DownloadPlugin downloads a specific plugin for the current platform
func (d *Downloader) DownloadPlugin(registry *PluginRegistry, pluginName string) error {
	metadata, exists := registry.Plugins[pluginName]
	if !exists {
		return fmt.Errorf("plugin %s not found in registry", pluginName)
	}

	// Get platform-specific binary info
	platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	binary, exists := metadata.Platforms[platformKey]
	if !exists {
		return fmt.Errorf("plugin %s not available for platform %s", pluginName, platformKey)
	}

	// Validate plugin metadata to prevent unnecessary downloads
	if err := d.validatePluginBinary(pluginName, binary); err != nil {
		return err
	}

	// Check if plugin already exists and is up to date
	pluginPath := filepath.Join(d.pluginDir, fmt.Sprintf("devex-plugin-%s", pluginName))
	if d.isPluginUpToDate(pluginPath, binary.Checksum) {
		fmt.Printf("Plugin %s is already up to date\n", pluginName)
		return nil
	}

	fmt.Printf("Downloading plugin %s v%s...\n", pluginName, metadata.Version)

	// Download the plugin
	resp, err := d.client.Get(binary.URL)
	if err != nil {
		return fmt.Errorf("failed to download plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download plugin: HTTP %d", resp.StatusCode)
	}

	// Ensure plugin directory exists
	if err := os.MkdirAll(d.pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Create temporary file
	tempPath := pluginPath + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Copy with checksum verification
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	if _, err := io.Copy(multiWriter, resp.Body); err != nil {
		_ = os.Remove(tempPath) // Cleanup temp file
		return fmt.Errorf("failed to download plugin: %w", err)
	}

	// Verify checksum
	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != binary.Checksum {
		_ = os.Remove(tempPath) // Cleanup temp file
		return fmt.Errorf("checksum mismatch: expected %s, got %s", binary.Checksum, actualChecksum)
	}

	// Make executable and move to final location
	if err := os.Chmod(tempPath, 0755); err != nil {
		_ = os.Remove(tempPath) // Cleanup temp file
		return fmt.Errorf("failed to make plugin executable: %w", err)
	}

	if err := os.Rename(tempPath, pluginPath); err != nil {
		_ = os.Remove(tempPath) // Cleanup temp file
		return fmt.Errorf("failed to move plugin to final location: %w", err)
	}

	fmt.Printf("Successfully installed plugin %s\n", pluginName)
	return nil
}

// validatePluginBinary validates plugin metadata to prevent unnecessary downloads
func (d *Downloader) validatePluginBinary(pluginName string, binary PlatformBinary) error {
	// Check if URL is empty or invalid
	if binary.URL == "" {
		return fmt.Errorf("plugin %s has no download URL for this platform", pluginName)
	}

	// Check if checksum is empty (indicates plugin not built yet)
	if binary.Checksum == "" {
		return fmt.Errorf("plugin %s has no checksum (binary not available)", pluginName)
	}

	// Check if size is zero (indicates empty or missing binary)
	if binary.Size == 0 {
		return fmt.Errorf("plugin %s has zero size (binary not available)", pluginName)
	}

	return nil
}

// isPluginUpToDate checks if a plugin is up to date by comparing checksums
func (d *Downloader) isPluginUpToDate(pluginPath, expectedChecksum string) bool {
	// If checksum is empty, we can't validate - assume not up to date
	if expectedChecksum == "" {
		return false
	}

	file, err := os.Open(pluginPath)
	if err != nil {
		return false
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	return actualChecksum == expectedChecksum
}

// DownloadRequiredPlugins downloads all plugins required for the current platform
func (d *Downloader) DownloadRequiredPlugins(requiredPlugins []string) error {
	registry, err := d.UpdateRegistry()
	if err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	fmt.Printf("Required plugins: %s\n", strings.Join(requiredPlugins, ", "))

	for _, pluginName := range requiredPlugins {
		if err := d.DownloadPlugin(registry, pluginName); err != nil {
			fmt.Printf("Warning: failed to download plugin %s: %v\n", pluginName, err)
			// Continue with other plugins rather than failing completely
		}
	}

	return nil
}

// GetAvailablePlugins returns all available plugins from the registry
func (d *Downloader) GetAvailablePlugins() (map[string]PluginMetadata, error) {
	registry, err := d.UpdateRegistry()
	if err != nil {
		return nil, err
	}

	return registry.Plugins, nil
}

// SearchPlugins searches for plugins by name or tags
func (d *Downloader) SearchPlugins(query string) (map[string]PluginMetadata, error) {
	allPlugins, err := d.GetAvailablePlugins()
	if err != nil {
		return nil, err
	}

	results := make(map[string]PluginMetadata)
	query = strings.ToLower(query)

	for name, metadata := range allPlugins {
		// Search in name, description, and tags
		if strings.Contains(strings.ToLower(name), query) ||
			strings.Contains(strings.ToLower(metadata.Description), query) {
			results[name] = metadata
			continue
		}

		// Search in tags
		for _, tag := range metadata.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results[name] = metadata
				break
			}
		}
	}

	return results, nil
}
