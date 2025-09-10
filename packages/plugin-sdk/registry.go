package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Constants for default configuration values
const (
	DefaultBaseURL      = "https://registry.devex.sh"
	DefaultTimeout      = 30 * time.Second
	DefaultUserAgent    = "devex-cli/1.0"
	DefaultSearchLimit  = 100
	DefaultCacheTTL     = 5 * time.Minute
	MaxPluginNameLength = 100
)

// RegistryError represents an error from the registry API
type RegistryError struct {
	HTTPStatus int
	Message    string
	Err        error
}

func (e *RegistryError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("registry error (HTTP %d): %s: %v", e.HTTPStatus, e.Message, e.Err)
	}
	return fmt.Sprintf("registry error (HTTP %d): %s", e.HTTPStatus, e.Message)
}

func (e *RegistryError) Unwrap() error {
	return e.Err
}

// RegistryClient provides simple, read-only access to the plugin registry API.
// This client connects to a simplified registry API that does not require authentication.
// It implements the Registry interface for easy testing and mocking.
type RegistryClient struct {
	baseURL     string
	client      *http.Client
	userAgent   string
	cache       *MemoryCache
	logger      Logger
	searchIndex *SearchIndex
	indexMu     sync.RWMutex
}

// SearchIndex provides efficient searching capabilities
type SearchIndex struct {
	tagIndex  map[string][]string // tag -> plugin names
	nameIndex map[string]bool     // normalized names for fast lookup
}

var _ Registry = (*RegistryClient)(nil) // Compile-time interface check

// RegistryConfig configures the registry client for simple, read-only access
type RegistryConfig struct {
	BaseURL           string        // Base URL of the registry API (defaults to DefaultBaseURL)
	Timeout           time.Duration // HTTP timeout (defaults to DefaultTimeout)
	UserAgent         string        // User agent string (defaults to DefaultUserAgent)
	CacheTTL          time.Duration // Cache TTL (defaults to DefaultCacheTTL)
	Logger            Logger        // Logger for debugging (optional)
	MaxIdleConns      int           // Maximum idle connections (defaults to 10)
	MaxIdleConnsPerHost int         // Maximum idle connections per host (defaults to 2)
	IdleConnTimeout   time.Duration // Idle connection timeout (defaults to 30s)
}

// NewRegistryClient creates a new registry client for simple, read-only access.
// This client connects to a simplified registry API without authentication.
func NewRegistryClient(config RegistryConfig) (*RegistryClient, error) {
	// Validation
	if config.BaseURL == "" {
		config.BaseURL = DefaultBaseURL
	}
	
	// Validate URL format
	if _, err := url.Parse(config.BaseURL); err != nil {
		return nil, fmt.Errorf("invalid BaseURL: %w", err)
	}

	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}
	if config.UserAgent == "" {
		config.UserAgent = DefaultUserAgent
	}

	// Set cache TTL default
	if config.CacheTTL == 0 {
		config.CacheTTL = DefaultCacheTTL
	}
	
	// Set logger default
	if config.Logger == nil {
		config.Logger = NewDefaultLogger(false)
	}
	
	// Set connection pooling defaults
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 10
	}
	if config.MaxIdleConnsPerHost == 0 {
		config.MaxIdleConnsPerHost = 2
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = 30 * time.Second
	}
	
	// Create HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
	}
	
	return &RegistryClient{
		baseURL:   strings.TrimSuffix(config.BaseURL, "/"),
		userAgent: config.UserAgent,
		client: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
		},
		cache:  NewMemoryCache(config.CacheTTL),
		logger: config.Logger,
	}, nil
}

// Simple registry response structure
type RegistryResponse struct {
	BaseURL      string                      `json:"base_url"`
	Version      string                      `json:"version"`
	LastUpdated  string                      `json:"last_updated"`
	Plugins      map[string]PluginMetadata   `json:"plugins"`
}

// Note: Upload functionality and advanced plugin info not available in simple API

// validatePluginName validates a plugin name to prevent security issues
func validatePluginName(name string) error {
	if name == "" {
		return errors.New("plugin name cannot be empty")
	}
	
	if len(name) > MaxPluginNameLength {
		return fmt.Errorf("plugin name too long (max %d characters)", MaxPluginNameLength)
	}
	
	// Check for path traversal attempts
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid plugin name: contains path separators or traversal sequences")
	}
	
	// Check for absolute paths
	if path.IsAbs(name) {
		return fmt.Errorf("invalid plugin name: absolute paths not allowed")
	}
	
	// Validate against a safe pattern (alphanumeric, dash, underscore)
	validNamePattern := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-_]*$`)
	if !validNamePattern.MatchString(name) {
		return fmt.Errorf("invalid plugin name: must contain only alphanumeric characters, dashes, and underscores")
	}
	
	return nil
}

// GetRegistry fetches the complete plugin registry with caching
func (c *RegistryClient) GetRegistry(ctx context.Context) (*PluginRegistry, error) {
	cacheKey := "registry:full"
	
	// Check cache first
	if cached, found := c.cache.Get(cacheKey); found {
		if registry, ok := cached.(*PluginRegistry); ok {
			c.logger.Debug("registry cache hit")
			return registry, nil
		}
	}
	
	c.logger.Debug("registry cache miss, fetching from API")
	resp, err := c.simpleRequest(ctx, "GET", "/api/v1/registry")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	var registryResp RegistryResponse
	if err := json.Unmarshal(resp, &registryResp); err != nil {
		return nil, fmt.Errorf("failed to parse registry response: %w", err)
	}

	// Parse the last updated timestamp
	lastUpdated, err := time.Parse(time.RFC3339, registryResp.LastUpdated)
	if err != nil {
		// Use zero time for invalid timestamps to clearly indicate the issue
		c.logger.Warn("failed to parse last_updated timestamp, using zero time", "error", err, "value", registryResp.LastUpdated)
		lastUpdated = time.Time{} // Zero time clearly indicates invalid/missing timestamp
	}

	// Convert to PluginRegistry format
	registry := &PluginRegistry{
		BaseURL:     registryResp.BaseURL,
		Version:     registryResp.Version,
		LastUpdated: lastUpdated,
		Plugins:     registryResp.Plugins,
	}

	// Cache the result
	c.cache.Set(cacheKey, registry)
	
	// Update search index
	c.updateSearchIndex(registry)
	
	return registry, nil
}

// GetPlugin fetches detailed information about a specific plugin
func (c *RegistryClient) GetPlugin(ctx context.Context, pluginName string) (*PluginMetadata, error) {
	// Validate plugin name to prevent path traversal
	if err := validatePluginName(pluginName); err != nil {
		return nil, fmt.Errorf("invalid plugin name: %w", err)
	}
	
	cacheKey := fmt.Sprintf("plugin:%s", pluginName)
	
	// Check cache first
	if cached, found := c.cache.Get(cacheKey); found {
		if plugin, ok := cached.(*PluginMetadata); ok {
			c.logger.Debug("plugin cache hit", "plugin", pluginName)
			return plugin, nil
		}
	}
	
	c.logger.Debug("plugin cache miss, fetching from API", "plugin", pluginName)
	path := fmt.Sprintf("/api/v1/plugins/%s", url.PathEscape(pluginName))
	resp, err := c.simpleRequest(ctx, "GET", path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plugin info: %w", err)
	}

	var pluginInfo PluginMetadata
	if err := json.Unmarshal(resp, &pluginInfo); err != nil {
		return nil, fmt.Errorf("failed to parse plugin data: %w", err)
	}

	// Cache the result
	c.cache.Set(cacheKey, &pluginInfo)
	
	return &pluginInfo, nil
}

// updateSearchIndex updates the cached search index from the registry
func (c *RegistryClient) updateSearchIndex(registry *PluginRegistry) {
	index := &SearchIndex{
		tagIndex:  make(map[string][]string),
		nameIndex: make(map[string]bool),
	}
	
	for name, plugin := range registry.Plugins {
		// Index normalized plugin names
		normalizedName := strings.ToLower(plugin.Name)
		index.nameIndex[normalizedName] = true
		
		// Index tags
		for _, tag := range plugin.Tags {
			normalizedTag := strings.ToLower(tag)
			index.tagIndex[normalizedTag] = append(index.tagIndex[normalizedTag], name)
		}
	}
	
	c.indexMu.Lock()
	c.searchIndex = index
	c.indexMu.Unlock()
}

// getSearchIndex returns the cached search index or builds one if not available
func (c *RegistryClient) getSearchIndex(registry *PluginRegistry) *SearchIndex {
	c.indexMu.RLock()
	if c.searchIndex != nil {
		defer c.indexMu.RUnlock()
		return c.searchIndex
	}
	c.indexMu.RUnlock()
	
	// Build index if not cached
	c.updateSearchIndex(registry)
	
	c.indexMu.RLock()
	defer c.indexMu.RUnlock()
	return c.searchIndex
}

// matchesQuery checks if a plugin matches the search query
func (c *RegistryClient) matchesQuery(plugin PluginMetadata, query string) bool {
	return strings.Contains(strings.ToLower(plugin.Name), query) ||
		   strings.Contains(strings.ToLower(plugin.Description), query)
}

// SearchPlugins searches for plugins by name or tags with optimized indexing and caching
func (c *RegistryClient) SearchPlugins(ctx context.Context, query string, tags []string, limit int) ([]PluginMetadata, error) {
	// Create cache key based on search parameters
	cacheKey := fmt.Sprintf("search:%s:%v:%d", query, tags, limit)
	
	// Check cache first
	if cached, found := c.cache.Get(cacheKey); found {
		if results, ok := cached.([]PluginMetadata); ok {
			c.logger.Debug("search cache hit", "query", query)
			return results, nil
		}
	}
	
	c.logger.Debug("search cache miss", "query", query)
	
	// Get all plugins first
	registry, err := c.GetRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry for search: %w", err)
	}

	// Use default limit if not specified
	if limit <= 0 {
		limit = DefaultSearchLimit
	}

	var results []PluginMetadata
	
	// Get cached search index for efficiency
	searchIndex := c.getSearchIndex(registry)
	query = strings.ToLower(query)
	
	// If we have tag filters, use the index for efficient lookup
	if len(tags) > 0 {
		candidateNames := make(map[string]bool)
		
		// Find plugins that match any of the tags
		for _, searchTag := range tags {
			normalizedTag := strings.ToLower(searchTag)
			if pluginNames, exists := searchIndex.tagIndex[normalizedTag]; exists {
				for _, name := range pluginNames {
					candidateNames[name] = true
				}
			}
		}
		
		// Collect all matching candidates first
		var candidates []PluginMetadata
		for pluginName := range candidateNames {
			plugin := registry.Plugins[pluginName]
			if query == "" || c.matchesQuery(plugin, query) {
				candidates = append(candidates, plugin)
			}
		}
		
		// Sort candidates for deterministic results
		sort.Slice(candidates, func(i, j int) bool {
			// Sort by relevance score (exact name match first), then alphabetically
			iExact := strings.EqualFold(candidates[i].Name, query)
			jExact := strings.EqualFold(candidates[j].Name, query)
			
			if iExact && !jExact {
				return true
			}
			if !iExact && jExact {
				return false
			}
			
			return candidates[i].Name < candidates[j].Name
		})
		
		// Apply limit after sorting
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		results = candidates
		
	} else {
		// No tag filter, search all plugins
		var candidates []PluginMetadata
		for _, plugin := range registry.Plugins {
			if query == "" || c.matchesQuery(plugin, query) {
				candidates = append(candidates, plugin)
			}
		}
		
		// Sort candidates for deterministic results
		sort.Slice(candidates, func(i, j int) bool {
			// Sort by relevance score (exact name match first), then alphabetically
			iExact := strings.EqualFold(candidates[i].Name, query)
			jExact := strings.EqualFold(candidates[j].Name, query)
			
			if iExact && !jExact {
				return true
			}
			if !iExact && jExact {
				return false
			}
			
			return candidates[i].Name < candidates[j].Name
		})
		
		// Apply limit after sorting
		if len(candidates) > limit {
			candidates = candidates[:limit]
		}
		results = candidates
	}

	// Ensure we return an empty slice, not nil
	if results == nil {
		results = []PluginMetadata{}
	}
	
	// Cache the results
	c.cache.Set(cacheKey, results)
	
	return results, nil
}

// Note: Upload, delete, and stats operations are not supported in the simple API
// These operations may be added in future versions with authentication

// simpleRequest makes a simple request to the registry API
func (c *RegistryClient) simpleRequest(ctx context.Context, method, path string) ([]byte, error) {
	fullURL := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to parse error message from response body
		var errorMsg string
		var errorResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errorResp) == nil && errorResp.Error != "" {
			errorMsg = errorResp.Error
		} else {
			errorMsg = string(respBody)
		}
		
		return nil, &RegistryError{
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("%s (URL: %s)", errorMsg, fullURL),
		}
	}

	return respBody, nil
}

// Note: Authentication and rate limiting functions removed 
// for simplified API compatibility

// Enhanced Downloader with registry client
type RegistryDownloader struct {
	*Downloader
	registryClient Registry // Use interface for better testability
}

var _ RegistryDownloaderInterface = (*RegistryDownloader)(nil) // Compile-time interface check

// NewRegistryDownloader creates a downloader with registry access
func NewRegistryDownloader(config DownloaderConfig, registryConfig RegistryConfig) (*RegistryDownloader, error) {
	downloader := NewSecureDownloader(config)
	registryClient, err := NewRegistryClient(registryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create registry client: %w", err)
	}
	
	return &RegistryDownloader{
		Downloader:     downloader,
		registryClient: registryClient,
	}, nil
}

// GetAvailablePlugins fetches plugins using registry API (backward compatible)
func (rd *RegistryDownloader) GetAvailablePlugins() (map[string]PluginMetadata, error) {
	ctx := context.Background()
	return rd.GetAvailablePluginsWithContext(ctx)
}

// GetAvailablePluginsWithContext fetches plugins using registry API with context
func (rd *RegistryDownloader) GetAvailablePluginsWithContext(ctx context.Context) (map[string]PluginMetadata, error) {
	registry, err := rd.registryClient.GetRegistry(ctx)
	if err != nil {
		// Fallback to local method if available
		if rd.Downloader != nil {
			return rd.Downloader.GetAvailablePlugins()
		}
		return nil, err
	}
	return registry.Plugins, nil
}

// SearchPlugins searches using registry API (backward compatible)
func (rd *RegistryDownloader) SearchPlugins(query string) (map[string]PluginMetadata, error) {
	ctx := context.Background()
	return rd.SearchPluginsWithContext(ctx, query)
}

// SearchPluginsWithContext searches using registry API with context
func (rd *RegistryDownloader) SearchPluginsWithContext(ctx context.Context, query string) (map[string]PluginMetadata, error) {
	plugins, err := rd.registryClient.SearchPlugins(ctx, query, nil, DefaultSearchLimit)
	if err != nil {
		// Fallback to local method if we have a Downloader
		if rd.Downloader != nil {
			return rd.Downloader.SearchPlugins(query)
		}
		return nil, err
	}

	// Convert to PluginMetadata map
	results := make(map[string]PluginMetadata)
	for _, plugin := range plugins {
		results[plugin.Name] = plugin
	}
	return results, nil
}

// GetPluginDetails returns detailed plugin information from registry API (backward compatible)
func (rd *RegistryDownloader) GetPluginDetails(pluginName string) (*PluginMetadata, error) {
	ctx := context.Background()
	return rd.GetPluginDetailsWithContext(ctx, pluginName)
}

// GetPluginDetailsWithContext returns detailed plugin information from registry API with context
func (rd *RegistryDownloader) GetPluginDetailsWithContext(ctx context.Context, pluginName string) (*PluginMetadata, error) {
	return rd.registryClient.GetPlugin(ctx, pluginName)
}
