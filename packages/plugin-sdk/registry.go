package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Constants for default configuration values
const (
	DefaultBaseURL      = "https://registry.devex.sh"
	DefaultTimeout      = 30 * time.Second
	DefaultUserAgent    = "devex-cli/1.0"
	DefaultSearchLimit  = 100
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
type RegistryClient struct {
	baseURL   string
	client    *http.Client
	userAgent string
}

// RegistryConfig configures the registry client for simple, read-only access
type RegistryConfig struct {
	BaseURL   string        // Base URL of the registry API (defaults to DefaultBaseURL)
	Timeout   time.Duration // HTTP timeout (defaults to DefaultTimeout)
	UserAgent string        // User agent string (defaults to DefaultUserAgent)
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

	return &RegistryClient{
		baseURL:   strings.TrimSuffix(config.BaseURL, "/"),
		userAgent: config.UserAgent,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// APIResponse represents a standard API response (no longer used for simple API)
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
	Code    int             `json:"code,omitempty"`
}

// Simple registry response structure
type RegistryResponse struct {
	BaseURL      string                      `json:"base_url"`
	Version      string                      `json:"version"`
	LastUpdated  string                      `json:"last_updated"`
	Plugins      map[string]PluginMetadata   `json:"plugins"`
}

// Note: Upload functionality and advanced plugin info not available in simple API

// GetRegistry fetches the complete plugin registry
func (c *RegistryClient) GetRegistry(ctx context.Context) (*PluginRegistry, error) {
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
		// Fallback to current time if parsing fails
		lastUpdated = time.Now()
	}

	// Convert to PluginRegistry format
	registry := &PluginRegistry{
		BaseURL:     registryResp.BaseURL,
		Version:     registryResp.Version,
		LastUpdated: lastUpdated,
		Plugins:     registryResp.Plugins,
	}

	return registry, nil
}

// GetPlugin fetches detailed information about a specific plugin
func (c *RegistryClient) GetPlugin(ctx context.Context, pluginName string) (*PluginMetadata, error) {
	path := fmt.Sprintf("/api/v1/plugins/%s", url.PathEscape(pluginName))
	resp, err := c.simpleRequest(ctx, "GET", path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plugin info: %w", err)
	}

	var pluginInfo PluginMetadata
	if err := json.Unmarshal(resp, &pluginInfo); err != nil {
		return nil, fmt.Errorf("failed to parse plugin data: %w", err)
	}

	return &pluginInfo, nil
}

// SearchPlugins searches for plugins by name or tags (client-side filtering)
func (c *RegistryClient) SearchPlugins(ctx context.Context, query string, tags []string, limit int) ([]PluginMetadata, error) {
	// Get all plugins first
	registry, err := c.GetRegistry(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry for search: %w", err)
	}

	var results []PluginMetadata
	query = strings.ToLower(query)

	// Use default limit if not specified
	if limit <= 0 {
		limit = DefaultSearchLimit
	}

	// Simple client-side filtering
	for _, plugin := range registry.Plugins {
		matches := false
		
		// Match by name or description
		if query == "" || 
		   strings.Contains(strings.ToLower(plugin.Name), query) ||
		   strings.Contains(strings.ToLower(plugin.Description), query) {
			matches = true
		}

		// Match by tags
		if len(tags) > 0 {
			tagMatch := false
			for _, searchTag := range tags {
				for _, pluginTag := range plugin.Tags {
					if strings.EqualFold(pluginTag, searchTag) {
						tagMatch = true
						break
					}
				}
				if tagMatch {
					break
				}
			}
			matches = matches && tagMatch
		}

		if matches {
			results = append(results, plugin)
			if len(results) >= limit {
				break
			}
		}
	}

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
	registryClient *RegistryClient
}

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

// GetAvailablePlugins fetches plugins using registry API
func (rd *RegistryDownloader) GetAvailablePlugins() (map[string]PluginMetadata, error) {
	ctx := context.Background()
	registry, err := rd.registryClient.GetRegistry(ctx)
	if err != nil {
		// Fallback to local method
		return rd.Downloader.GetAvailablePlugins()
	}
	return registry.Plugins, nil
}

// SearchPlugins searches using registry API
func (rd *RegistryDownloader) SearchPlugins(query string) (map[string]PluginMetadata, error) {
	ctx := context.Background()
	plugins, err := rd.registryClient.SearchPlugins(ctx, query, nil, DefaultSearchLimit)
	if err != nil {
		// Fallback to local method
		return rd.Downloader.SearchPlugins(query)
	}

	// Convert to PluginMetadata map
	results := make(map[string]PluginMetadata)
	for _, plugin := range plugins {
		results[plugin.Name] = plugin
	}
	return results, nil
}

// GetPluginDetails returns detailed plugin information from registry API
func (rd *RegistryDownloader) GetPluginDetails(pluginName string) (*PluginMetadata, error) {
	ctx := context.Background()
	return rd.registryClient.GetPlugin(ctx, pluginName)
}
