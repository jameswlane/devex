package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// RegistryClient handles communication with the DevEx plugin registry
type RegistryClient struct {
	baseURL    string
	httpClient *http.Client
	cache      *registryCache
}

// registryCache provides in-memory caching for registry queries
type registryCache struct {
	mu       sync.RWMutex
	plugins  map[string]*PluginMetadata
	metadata map[string]*RegistryMetadata
	expiry   map[string]time.Time
}

// RegistryMetadata contains metadata about the registry
type RegistryMetadata struct {
	Version      string         `json:"version"`
	LastUpdated  time.Time      `json:"last_updated"`
	TotalPlugins int            `json:"total_plugins"`
	Platforms    []string       `json:"platforms"`
	Categories   map[string]int `json:"categories"`
}

// PluginQueryOptions provides filtering options for plugin queries
type PluginQueryOptions struct {
	OS           string   `json:"os,omitempty"`
	Distribution string   `json:"distribution,omitempty"`
	Desktop      string   `json:"desktop,omitempty"`
	Type         string   `json:"type,omitempty"`
	Categories   []string `json:"categories,omitempty"`
	IncludeBeta  bool     `json:"include_beta,omitempty"`
}

// NewRegistryClient creates a new registry client
func NewRegistryClient(baseURL string) *RegistryClient {
	if baseURL == "" {
		baseURL = "https://registry.devex.sh/api/v1"
	}

	return &RegistryClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache: &registryCache{
			plugins:  make(map[string]*PluginMetadata),
			metadata: make(map[string]*RegistryMetadata),
			expiry:   make(map[string]time.Time),
		},
	}
}

// GetAvailablePlugins queries the registry for plugins compatible with the given platform
func (rc *RegistryClient) GetAvailablePlugins(ctx context.Context, os, distribution string) ([]PluginMetadata, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("plugins_%s_%s", os, distribution)
	if cached := rc.getFromCache(cacheKey); cached != nil {
		if plugins, ok := cached.([]PluginMetadata); ok {
			return plugins, nil
		}
	}

	// Build query parameters
	params := url.Values{}
	params.Set("os", os)
	if distribution != "" && distribution != "unknown" {
		params.Set("distribution", distribution)
	}

	// Make request
	endpoint := fmt.Sprintf("%s/plugins?%s", rc.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var plugins []PluginMetadata
	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Cache the result
	rc.putInCache(cacheKey, plugins, 1*time.Hour)

	return plugins, nil
}

// GetPluginMetadata retrieves metadata for a specific plugin
func (rc *RegistryClient) GetPluginMetadata(ctx context.Context, pluginName string) (*PluginMetadata, error) {
	// Check cache first
	if cached := rc.cache.plugins[pluginName]; cached != nil {
		if expiry, ok := rc.cache.expiry[pluginName]; ok && time.Now().Before(expiry) {
			return cached, nil
		}
	}

	// Make request
	endpoint := fmt.Sprintf("%s/plugins/%s", rc.baseURL, pluginName)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var metadata PluginMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Cache the result
	rc.cache.mu.Lock()
	rc.cache.plugins[pluginName] = &metadata
	rc.cache.expiry[pluginName] = time.Now().Add(1 * time.Hour)
	rc.cache.mu.Unlock()

	return &metadata, nil
}

// QueryPlugins performs an advanced query with filtering options
func (rc *RegistryClient) QueryPlugins(ctx context.Context, options PluginQueryOptions) ([]PluginMetadata, error) {
	// Build query parameters
	params := url.Values{}
	if options.OS != "" {
		params.Set("os", options.OS)
	}
	if options.Distribution != "" {
		params.Set("distribution", options.Distribution)
	}
	if options.Desktop != "" {
		params.Set("desktop", options.Desktop)
	}
	if options.Type != "" {
		params.Set("type", options.Type)
	}
	for _, cat := range options.Categories {
		params.Add("category", cat)
	}
	if options.IncludeBeta {
		params.Set("include_beta", "true")
	}

	// Make request
	endpoint := fmt.Sprintf("%s/plugins/query?%s", rc.baseURL, params.Encode())
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var plugins []PluginMetadata
	if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return plugins, nil
}

// GetRegistryMetadata retrieves metadata about the registry itself
func (rc *RegistryClient) GetRegistryMetadata(ctx context.Context) (*RegistryMetadata, error) {
	// Check cache
	if cached := rc.getFromCache("registry_metadata"); cached != nil {
		if metadata, ok := cached.(*RegistryMetadata); ok {
			return metadata, nil
		}
	}

	// Make request
	endpoint := fmt.Sprintf("%s/metadata", rc.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := rc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registry request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	var metadata RegistryMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Cache the result
	rc.putInCache("registry_metadata", &metadata, 6*time.Hour)

	return &metadata, nil
}

// CheckPluginCompatibility checks if a plugin is compatible with the current platform
func (rc *RegistryClient) CheckPluginCompatibility(ctx context.Context, pluginName, os, arch string) (bool, error) {
	metadata, err := rc.GetPluginMetadata(ctx, pluginName)
	if err != nil {
		return false, err
	}

	// Check if platform is supported
	platformKey := fmt.Sprintf("%s-%s", os, arch)
	for platform := range metadata.Platforms {
		if platform == platformKey {
			return true, nil
		}
		// Also check wildcard matches (e.g., "linux-*" matches any Linux arch)
		if platform == fmt.Sprintf("%s-*", os) {
			return true, nil
		}
	}

	return false, nil
}

// GetPluginDependencies retrieves the dependency tree for a plugin
func (rc *RegistryClient) GetPluginDependencies(ctx context.Context, pluginName string) ([]string, error) {
	metadata, err := rc.GetPluginMetadata(ctx, pluginName)
	if err != nil {
		return nil, err
	}

	// Return direct dependencies
	// In a full implementation, this would recursively resolve the dependency tree
	return metadata.Dependencies, nil
}

// Cache management helpers

func (rc *RegistryClient) getFromCache(key string) interface{} {
	rc.cache.mu.RLock()
	defer rc.cache.mu.RUnlock()

	if expiry, ok := rc.cache.expiry[key]; ok {
		if time.Now().Before(expiry) {
			if metadata, ok := rc.cache.metadata[key]; ok {
				return metadata
			}
		}
	}
	return nil
}

func (rc *RegistryClient) putInCache(key string, value interface{}, duration time.Duration) {
	rc.cache.mu.Lock()
	defer rc.cache.mu.Unlock()

	rc.cache.expiry[key] = time.Now().Add(duration)

	switch v := value.(type) {
	case *RegistryMetadata:
		rc.cache.metadata[key] = v
	case []PluginMetadata:
		// For plugin lists, we don't store in the main cache map
		// but the expiry is still tracked
	}
}

// ClearCache clears the registry cache
func (rc *RegistryClient) ClearCache() {
	rc.cache.mu.Lock()
	defer rc.cache.mu.Unlock()

	rc.cache.plugins = make(map[string]*PluginMetadata)
	rc.cache.metadata = make(map[string]*RegistryMetadata)
	rc.cache.expiry = make(map[string]time.Time)
}
