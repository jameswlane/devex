package plugin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// RegistryClient handles communication with the DevEx plugin registry
type RegistryClient struct {
	baseURL    string
	httpClient *http.Client
	cache      *registryCache
}

// registryCache provides in-memory caching for registry queries with TTL and size limits
type registryCache struct {
	mu              sync.RWMutex
	plugins         map[string]*PluginMetadata
	metadata        map[string]*RegistryMetadata
	expiry          map[string]time.Time
	maxEntries      int
	lastCleanup     time.Time
	cleanupInterval time.Duration
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
func NewRegistryClient(baseURL string) (*RegistryClient, error) {
	if baseURL == "" {
		baseURL = "https://registry.devex.sh/api/v1"
	}

	// Validate URL to prevent SSRF attacks
	if !isValidRegistryURL(baseURL) {
		return nil, fmt.Errorf("invalid registry URL: %s", baseURL)
	}

	return &RegistryClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Shorter timeout for better responsiveness
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion:         tls.VersionTLS12,
					InsecureSkipVerify: false, // Always verify certificates
				},
				MaxIdleConns:       10,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
		cache: &registryCache{
			plugins:         make(map[string]*PluginMetadata),
			metadata:        make(map[string]*RegistryMetadata),
			expiry:          make(map[string]time.Time),
			maxEntries:      1000, // Prevent unlimited growth
			lastCleanup:     time.Now(),
			cleanupInterval: 15 * time.Minute, // Clean up expired entries every 15 minutes
		},
	}, nil
}

// isValidRegistryURL validates registry URLs to prevent SSRF and other attacks
func isValidRegistryURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	// Must use HTTPS for security
	if parsedURL.Scheme != "https" {
		return false
	}

	// Validate hostname - must not be empty and should be reasonable
	hostname := parsedURL.Hostname()
	if hostname == "" || len(hostname) > 253 {
		return false
	}

	// Prevent localhost and private network access to avoid SSRF
	privateNetworks := []string{
		"localhost",
		"127.",
		"10.",
		"172.16.", "172.17.", "172.18.", "172.19.", "172.20.",
		"172.21.", "172.22.", "172.23.", "172.24.", "172.25.",
		"172.26.", "172.27.", "172.28.", "172.29.", "172.30.", "172.31.",
		"192.168.",
		"169.254.", // Link-local
		"::1",      // IPv6 localhost
		"fc00:",    // IPv6 private
		"fd00:",    // IPv6 private
		"fe80:",    // IPv6 link-local
	}

	for _, private := range privateNetworks {
		if strings.HasPrefix(hostname, private) {
			return false
		}
	}

	// Ensure it's a reasonable registry domain
	allowedDomains := []string{
		"registry.devex.sh",
		"api.devex.sh",
		"cdn.devex.sh",
	}

	// Allow any devex.sh subdomain or the specific allowed domains
	if strings.HasSuffix(hostname, ".devex.sh") || strings.HasSuffix(hostname, "devex.sh") {
		return true
	}

	for _, allowed := range allowedDomains {
		if hostname == allowed {
			return true
		}
	}

	// For development/testing, allow explicit localhost with specific ports
	if hostname == "localhost" && (parsedURL.Port() == "8080" || parsedURL.Port() == "3000") {
		return true
	}

	return false
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

	// Make request with retry logic
	endpoint := fmt.Sprintf("%s/plugins?%s", rc.baseURL, params.Encode())

	var plugins []PluginMetadata
	err := rc.retryRequest(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := rc.httpClient.Do(req)
		if err != nil {
			log.Printf("Registry request failed, will retry: %v", err)
			return fmt.Errorf("registry request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Registry returned status %d, will retry if appropriate", resp.StatusCode)
			return fmt.Errorf("registry returned status %d", resp.StatusCode)
		}

		if err := json.NewDecoder(resp.Body).Decode(&plugins); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("Registry request failed after all retries: %v", err)
		return nil, err
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

	// Check for expired entry
	if expiry, ok := rc.cache.expiry[key]; ok {
		if time.Now().Before(expiry) {
			if metadata, ok := rc.cache.metadata[key]; ok {
				return metadata
			}
			// Check plugins cache for backwards compatibility
			if strings.HasPrefix(key, "plugins_") {
				// This would need to be handled differently for plugin lists
				// For now, return nil to trigger fresh fetch
				return nil
			}
		}
	}
	return nil
}

func (rc *RegistryClient) putInCache(key string, value interface{}, duration time.Duration) {
	rc.cache.mu.Lock()
	defer rc.cache.mu.Unlock()

	// Perform cleanup if needed
	rc.cleanupExpiredEntries()

	// Check if cache is at capacity
	if len(rc.cache.expiry) >= rc.cache.maxEntries {
		// Remove oldest entries (simple LRU-like behavior)
		rc.evictOldestEntries(rc.cache.maxEntries / 4) // Remove 25% of entries
	}

	rc.cache.expiry[key] = time.Now().Add(duration)

	switch v := value.(type) {
	case *RegistryMetadata:
		rc.cache.metadata[key] = v
	case []PluginMetadata:
		// For plugin lists, we don't store in the main cache map
		// but the expiry is still tracked
		// Note: In a full implementation, we'd store these separately
	}
}

// cleanupExpiredEntries removes expired cache entries (should be called with cache lock held)
func (rc *RegistryClient) cleanupExpiredEntries() {
	if time.Since(rc.cache.lastCleanup) < rc.cache.cleanupInterval {
		return
	}

	now := time.Now()
	expiredKeys := []string{}

	// Find expired keys
	for key, expiry := range rc.cache.expiry {
		if now.After(expiry) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	// Remove expired entries
	for _, key := range expiredKeys {
		delete(rc.cache.expiry, key)
		delete(rc.cache.plugins, key)
		delete(rc.cache.metadata, key)
	}

	rc.cache.lastCleanup = now

	if len(expiredKeys) > 0 {
		log.Printf("Cleaned up %d expired cache entries", len(expiredKeys))
	}
}

// evictOldestEntries removes the oldest cache entries (should be called with cache lock held)
func (rc *RegistryClient) evictOldestEntries(count int) {
	if count <= 0 {
		return
	}

	// Create a list of keys sorted by expiry time
	type keyExpiry struct {
		key    string
		expiry time.Time
	}

	entries := make([]keyExpiry, 0, len(rc.cache.expiry))
	for key, expiry := range rc.cache.expiry {
		entries = append(entries, keyExpiry{key: key, expiry: expiry})
	}

	// Sort by expiry time (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].expiry.After(entries[j].expiry) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Remove the oldest entries
	toRemove := count
	if toRemove > len(entries) {
		toRemove = len(entries)
	}

	for i := 0; i < toRemove; i++ {
		key := entries[i].key
		delete(rc.cache.expiry, key)
		delete(rc.cache.plugins, key)
		delete(rc.cache.metadata, key)
	}

	log.Printf("Evicted %d oldest cache entries to free memory", toRemove)
}

// ClearCache clears the registry cache
func (rc *RegistryClient) ClearCache() {
	rc.cache.mu.Lock()
	defer rc.cache.mu.Unlock()

	rc.cache.plugins = make(map[string]*PluginMetadata)
	rc.cache.metadata = make(map[string]*RegistryMetadata)
	rc.cache.expiry = make(map[string]time.Time)
}

// retryRequest executes a request with exponential backoff retry logic
func (rc *RegistryClient) retryRequest(ctx context.Context, operation func() error) error {
	maxRetries := 3
	baseDelay := 500 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}

		// Don't retry on final attempt
		if attempt == maxRetries {
			return err
		}

		// Check if we should retry based on error type
		if !isRetryableError(err) {
			return err
		}

		// Calculate exponential backoff delay
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))

		log.Printf("Registry request failed (attempt %d/%d), retrying in %v: %v",
			attempt+1, maxRetries+1, delay, err)

		// Wait with context cancellation support
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("max retries exceeded")
}

// isRetryableError determines if an error should trigger a retry
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Retry on network errors
	retryableErrors := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"network is unreachable",
		"no such host",
		"status 429", // Rate limited
		"status 500", // Internal server error
		"status 502", // Bad gateway
		"status 503", // Service unavailable
		"status 504", // Gateway timeout
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(strings.ToLower(errStr), retryable) {
			return true
		}
	}

	return false
}
