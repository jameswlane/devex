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
	"sync/atomic"
	"time"
	"unsafe"
)

// Constants for default configuration values
const (
	DefaultBaseURL           = "https://registry.devex.sh"
	DefaultTimeout           = 30 * time.Second
	DefaultUserAgent         = "devex-cli/1.0"
	DefaultSearchLimit       = 100
	DefaultCacheTTL          = 5 * time.Minute
	MaxPluginNameLength      = 100
	DefaultMaxIdleConns      = 10
	DefaultMaxIdleConnsPerHost = 2
	DefaultIdleConnTimeout   = 30 * time.Second
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
	searchIndex unsafe.Pointer // *SearchIndex - atomic pointer for lock-free reads
}

// SearchIndex provides efficient searching capabilities with pre-sorted data
type SearchIndex struct {
	tagIndex    map[string][]string // tag -> plugin names (pre-sorted)
	nameIndex   map[string]bool     // normalized names for fast lookup
	allPlugins  []PluginMetadata    // all plugins pre-sorted alphabetically
	pluginIndex map[string]int      // plugin name -> index in allPlugins for O(1) lookup
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
		config.MaxIdleConns = DefaultMaxIdleConns
	}
	if config.MaxIdleConnsPerHost == 0 {
		config.MaxIdleConnsPerHost = DefaultMaxIdleConnsPerHost
	}
	if config.IdleConnTimeout == 0 {
		config.IdleConnTimeout = DefaultIdleConnTimeout
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
		// Use Unix epoch as fallback to avoid downstream time-based logic issues
		// This is a known reference point (January 1, 1970 UTC) that won't break time comparisons
		fallbackTime := time.Unix(0, 0)
		c.logger.Warn("timestamp parsing failed, applying Unix epoch fallback to prevent time logic errors", 
			"operation", "parse_registry_timestamp",
			"parse_error", err.Error(), 
			"invalid_value", registryResp.LastUpdated,
			"fallback_time", fallbackTime.Format(time.RFC3339),
			"fallback_unix", fallbackTime.Unix(),
			"impact", "registry will appear as very old, may trigger unnecessary updates")
		lastUpdated = fallbackTime
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
		tagIndex:    make(map[string][]string),
		nameIndex:   make(map[string]bool),
		allPlugins:  make([]PluginMetadata, 0, len(registry.Plugins)),
		pluginIndex: make(map[string]int),
	}
	
	// Build sorted list of all plugins
	for _, plugin := range registry.Plugins {
		index.allPlugins = append(index.allPlugins, plugin)
	}
	
	// Sort all plugins alphabetically for consistent ordering
	sort.Slice(index.allPlugins, func(i, j int) bool {
		return index.allPlugins[i].Name < index.allPlugins[j].Name
	})
	
	// Build lookup indexes
	for i, plugin := range index.allPlugins {
		// Plugin name -> index mapping for O(1) lookup
		index.pluginIndex[plugin.Name] = i
		
		// Index normalized plugin names
		normalizedName := strings.ToLower(plugin.Name)
		index.nameIndex[normalizedName] = true
		
		// Index tags with pre-sorted plugin names
		for _, tag := range plugin.Tags {
			normalizedTag := strings.ToLower(tag)
			index.tagIndex[normalizedTag] = append(index.tagIndex[normalizedTag], plugin.Name)
		}
	}
	
	// Sort plugin names within each tag for consistent ordering
	for tag := range index.tagIndex {
		sort.Strings(index.tagIndex[tag])
	}
	
	// Atomically update the search index pointer for lock-free reads
	atomic.StorePointer(&c.searchIndex, unsafe.Pointer(index))
}

// getSearchIndex returns the cached search index or builds one if not available
func (c *RegistryClient) getSearchIndex(registry *PluginRegistry) *SearchIndex {
	// Atomically load the search index pointer for lock-free reads
	if ptr := atomic.LoadPointer(&c.searchIndex); ptr != nil {
		return (*SearchIndex)(ptr)
	}
	
	// Build index if not cached
	c.updateSearchIndex(registry)
	
	// Load again after building
	return (*SearchIndex)(atomic.LoadPointer(&c.searchIndex))
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

	// Get cached search index for efficiency
	searchIndex := c.getSearchIndex(registry)
	query = strings.ToLower(query)
	
	var results []PluginMetadata
	
	// If we have tag filters, use the index for efficient lookup
	if len(tags) > 0 {
		candidateIndices := make(map[int]bool)
		
		// Find plugins that match any of the tags using pre-sorted data
		for _, searchTag := range tags {
			normalizedTag := strings.ToLower(searchTag)
			if pluginNames, exists := searchIndex.tagIndex[normalizedTag]; exists {
				// Plugin names are already sorted, get their indices efficiently
				for _, name := range pluginNames {
					if idx, exists := searchIndex.pluginIndex[name]; exists {
						candidateIndices[idx] = true
					}
				}
			}
		}
		
		// Collect indices in sorted order for deterministic results
		sortedIndices := make([]int, 0, len(candidateIndices))
		for idx := range candidateIndices {
			sortedIndices = append(sortedIndices, idx)
		}
		sort.Ints(sortedIndices)
		
		// Apply query filter and relevance scoring using pre-sorted data
		// Pre-allocate slices with reasonable capacity to reduce allocations
		exactMatches := make([]PluginMetadata, 0, min(limit/4, 8))     // Assume ~25% exact matches max
		otherMatches := make([]PluginMetadata, 0, min(limit*3/4, 32))  // Remaining capacity
		
		for _, idx := range sortedIndices {
			plugin := searchIndex.allPlugins[idx]
			if query == "" || c.matchesQuery(plugin, query) {
				// Separate exact matches for relevance scoring
				if strings.EqualFold(plugin.Name, query) {
					exactMatches = append(exactMatches, plugin)
				} else {
					otherMatches = append(otherMatches, plugin)
				}
				
				// Early exit if we have enough results
				if len(exactMatches)+len(otherMatches) >= limit {
					break
				}
			}
		}
		
		// Combine results with exact matches first (already sorted)
		results = append(results, exactMatches...)
		if len(results) < limit && len(otherMatches) > 0 {
			remaining := limit - len(results)
			if remaining > len(otherMatches) {
				remaining = len(otherMatches)
			}
			results = append(results, otherMatches[:remaining]...)
		}
		
	} else {
		// No tag filter, use pre-sorted all plugins list
		// Pre-allocate slices with reasonable capacity to reduce allocations
		exactMatches := make([]PluginMetadata, 0, min(limit/4, 8))     // Assume ~25% exact matches max
		otherMatches := make([]PluginMetadata, 0, min(limit*3/4, 32))  // Remaining capacity
		
		// Search through pre-sorted plugins
		for _, plugin := range searchIndex.allPlugins {
			if query == "" || c.matchesQuery(plugin, query) {
				// Separate exact matches for relevance scoring
				if strings.EqualFold(plugin.Name, query) {
					exactMatches = append(exactMatches, plugin)
				} else {
					otherMatches = append(otherMatches, plugin)
				}
				
				// Early exit if we have enough results
				if len(exactMatches)+len(otherMatches) >= limit {
					break
				}
			}
		}
		
		// Combine results with exact matches first (already sorted)
		results = append(results, exactMatches...)
		if len(results) < limit && len(otherMatches) > 0 {
			remaining := limit - len(results)
			if remaining > len(otherMatches) {
				remaining = len(otherMatches)
			}
			results = append(results, otherMatches[:remaining]...)
		}
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

// GetAvailablePlugins fetches all available plugins from the registry
func (rd *RegistryDownloader) GetAvailablePlugins(ctx context.Context) (map[string]PluginMetadata, error) {
	registry, err := rd.registryClient.GetRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return registry.Plugins, nil
}

// SearchPlugins searches for plugins using the registry API
func (rd *RegistryDownloader) SearchPlugins(ctx context.Context, query string) (map[string]PluginMetadata, error) {
	plugins, err := rd.registryClient.SearchPlugins(ctx, query, nil, DefaultSearchLimit)
	if err != nil {
		return nil, err
	}

	// Convert to PluginMetadata map
	results := make(map[string]PluginMetadata)
	for _, plugin := range plugins {
		results[plugin.Name] = plugin
	}
	return results, nil
}

// GetPluginDetails returns detailed plugin information from the registry API
func (rd *RegistryDownloader) GetPluginDetails(ctx context.Context, pluginName string) (*PluginMetadata, error) {
	return rd.registryClient.GetPlugin(ctx, pluginName)
}
